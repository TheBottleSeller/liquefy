package liquidengine

import (
	"fmt"
	"math"
	"time"
	"strconv"

	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/service/ec2"

	lq "bargain/liquefy/models"
	"bargain/liquefy/db"
	aws "bargain/liquefy/cloudprovider"
)

type SpotRequest struct {
	Cpu     float64
	Memory  float64
	Gpu     float64
	Disk    float64
}

type SpotMatch struct {
	AwsInstanceType     aws.InstanceType
	AwsAvailabilityZone aws.AZ
	AwsSpotPrice        float64
}

type CostEngine interface {
	Match(userId uint, req *SpotRequest) (*SpotMatch, error)
	TrackResourceCost(resourceId uint) (float64, error)
	GetResourceCostWithAwsApi(resource *lq.ResourceInstance, startTime, endTime time.Time) (float64, error)
}

type awsEngine struct {}

func NewCostEngine() (CostEngine) {
	return &awsEngine{}
}

// This algorithm will blow y'alls mother fucker's minds
func (engine *awsEngine) Match(userId uint, req *SpotRequest) (*SpotMatch, error) {
	log.Debugf("Finding optimal match for request %v", req)
	var optimalMatch SpotMatch
	var minSpotPrice ec2.SpotPrice

	user, err := db.Users().Get(userId)
	if err != nil {
		return nil, lq.NewErrorf(err, "Engine failed matching request")
	}
	awsAccount, err := db.AwsAccounts().Get(user.AwsAccountID)
	if err != nil {
		return nil, lq.NewErrorf(err, "Engine failed matching request")
	}

	unavailableMarkets := engine.findUsersUnavailableMarkets(user, awsAccount)

	// Find all the possible markets to query for prices
	marketsExist := false
	availableInstances := aws.FindPossibleInstances(req.Cpu, req.Memory, req.Gpu, req.Disk)
	availableMarkets := make(map[aws.AZ]map[aws.InstanceType]struct{})
	for _, az := range aws.AllAvailabilityZones {
		availableMarkets[az] = make(map[aws.InstanceType]struct{})
		for _, instance := range availableInstances {
			_, isInstanceUnavailable := unavailableMarkets[az][instance]
			if !isInstanceUnavailable {
				marketsExist = true
				availableMarkets[az][instance] = struct{}{}
			}
		}
	}

	if !marketsExist {
		return &SpotMatch{}, fmt.Errorf("There are no available markets to run this job")
	}

	awsCloud := aws.NewAwsCloud(awsAccount.AwsAccessKey, awsAccount.AwsSecretKey)
	azToSpotPrices := make(map[aws.AZ]map[aws.InstanceType]float64)
	for az, instanceMap := range availableMarkets {
		instances := []aws.InstanceType{}
		for instance := range instanceMap {
			instances = append(instances, instance)
		}
		spotPriceMap, err := awsCloud.GetCurrentSpotPrices(az, instances)
		if err != nil {
			err = lq.NewError("Failed to fetch current spot prices", err)
			log.Debug(err)
			continue
		}

		azToSpotPrices[az] = spotPriceMap
	}
	//azToSpotPrices, err := engine.SpotPriceDB.GetCurrentSpotPrices(availableMarkets)

	minPrice := math.Inf(1)
	for az, instanceToPrices := range azToSpotPrices {
		for instance, price := range instanceToPrices {
			if price < minPrice {
				minPrice = price

				// Fudge the min price by increasing it by 25%
				priceString := fmt.Sprintf("%f", 1.25 * minPrice)
				minSpotPrice.InstanceType = instance.StringPtr()
				minSpotPrice.AvailabilityZone = az.StringPtr()
				minSpotPrice.SpotPrice = &priceString
			}
		}
	}

	if math.IsInf(minPrice, 1) {
		return nil, fmt.Errorf("No instance in any az can be found")
	}

	// HARDCODE THE MAX SPOT PRICE TO BE $2
	if minPrice > 2.0 {
		return nil, fmt.Errorf("No instance in any az can be found for less than $2")
	}

	log.Debugf("Found matching spot price: %v", minSpotPrice)
	price, err := strconv.ParseFloat(*minSpotPrice.SpotPrice, 64)
	if err != nil {
		return nil, lq.NewError("Recieved a malformed spot price from Amazon oO", err)
	}

	// Construct and return optimal match
	optimalMatch.AwsSpotPrice = price
	optimalMatch.AwsAvailabilityZone = aws.AZ(*minSpotPrice.AvailabilityZone)
	optimalMatch.AwsInstanceType = aws.InstanceType(*minSpotPrice.InstanceType)
	return &optimalMatch, nil
}

func (engine *awsEngine) findUsersUnavailableMarkets(user *lq.User, awsAccount *lq.AwsAccount) map[aws.AZ]map[aws.InstanceType]struct{} {
	// Get current list of known unavailable markets
	unavailableMarkets := aws.GetUnavailableMarkets()

	// Remove markets which the user does not have a subnet for
	for _, az := range aws.AllAvailabilityZones {
		if awsAccount.GetSubnetId(az.String()) == "" {
			log.Debugf("User does not have subnet for availability zone %s, skipping all markets", az.String())
			for instance := range aws.AvailableInstances {
				unavailableMarkets[az][instance] = struct{}{}
			}
		}
	}

	// Remove markets which the user does not have a ssh key for
	for region, azs := range aws.AWSRegionsToAZs {
		if awsAccount.GetSshPrivateKey(region.String()) == "" {
			log.Debugf("User does not have private key for region %s, skipping all markets", region.String())
			for _, az := range azs {
				for instance := range aws.AvailableInstances {
					unavailableMarkets[az][instance] = struct{}{}
				}
			}
		}
	}

	return unavailableMarkets
}

/*
 * TrackResourceCost
 * Finds the total cost of running an AWS spot instance
 */
func (engine *awsEngine) TrackResourceCost(resourceId uint) (float64, error) {
	resource, err := db.Resources().Get(resourceId)
	if err != nil {
		return 0.0, err
	}
	return engine.GetResourceCostWithAwsApi(resource, time.Unix(0, resource.LaunchTime), time.Now())
}

func (engine *awsEngine) GetResourceCostWithAwsApi(resource *lq.ResourceInstance, startTime, endTime time.Time) (float64, error) {
	log.Debugf("Getting resource cost for market (%s, %s) between %s and %s",
		resource.AwsAvailabilityZone, resource.AwsInstanceType, startTime.UTC().String(), endTime.UTC().String())
	user, err := db.Users().Get(resource.OwnerId)
	if err != nil {
		return 0.0, lq.NewError("Failed getting resource cost", err)
	}

	awsAccount, err := db.AwsAccounts().Get(user.AwsAccountID)
	if err != nil {
		return 0.0, lq.NewError("Failed getting resource cost", err)
	}

	awsCloud := aws.NewAwsCloud(awsAccount.AwsAccessKey, awsAccount.AwsSecretKey)
	history, err := awsCloud.GetSpotPriceHistory(aws.AZ(resource.AwsAvailabilityZone),
		aws.InstanceType(resource.AwsInstanceType), startTime, endTime)
	if err != nil {
		return 0.0, lq.NewError("Failed getting resource cost", err)
	}

	if !history[0].Timestamp.Before(startTime) {
		log.Warn("Recieved price history does not cover time bounds")
	} else {
		history[0].Timestamp = &startTime
	}

	totalPrice := 0.0
	for i := 0; i < len(history); i++ {
		var start time.Time
		var end time.Time

		// if we are at the first price point, use the launch time to start
		if i == 0 {
			start = startTime
		} else {
			start = *history[i].Timestamp
		}

		// if we are the last price point, use the endTime (set as time.Now above)
		if i == len(history) - 1 {
			end = endTime
		} else {
			end = *history[i + 1].Timestamp
		}

		diff := end.Sub(start)
		price, err := strconv.ParseFloat(*history[i].SpotPrice, 64)
		if err != nil {
			log.Warnf("Could not parse price %s", history[i].SpotPrice)
			continue
		}
		diffPrice := diff.Hours() * price // price = $/hr
		totalPrice += diffPrice
	}

	return totalPrice, nil
}

//func (engine *awsEngine) getResourceCostWithInfluxDB(resource *lq.ResourceInstance, startTimeUnix, endTimeUnix int64) (float64, error) {
//	startTime := time.Unix(startTimeUnix, 0)
//	endTime := time.Unix(endTimeUnix, 0)
//
//	// We fetch the price history 20 seconds before the start time to ensure that we cover the whole lifetime
//	// of the resource
//	justPriorToStartTime := time.Unix(startTimeUnix, 0).Add(-20 * time.Second)
//
//	history, err := engine.SpotPriceDB.GetSpotPriceHistory(resource.AwsAvailabilityZone, resource.AwsInstanceType,
//		justPriorToStartTime, endTime)
//	if err != nil {
//		return 0.0, err
//	}
//
//	// Because first few times may be before start time, find the latest price that is earlier than the start time.
//	// Imagine the flow of time, with the | signifying points in the pricing history
//	// L = launch time, I = index we want to start at
//	// 0         1         2          3
//	// | ------- | ------- | -------- | ----->
//	//                     ^I   ^L
//	// In the above, we want to skip the first two price points and jump straight to index 2
//	startIndex := 0
//	for _, price := range history {
//		if startTime.After(price.Time) {
//			break
//		}
//		startIndex++
//	}
//
//	// Only deal with the price history from startIndex
//	history =  history[startIndex:]
//
//	// Do some validation on the history
//	if len(history) == 0 {
//		return 0.0, fmt.Errorf("Cannot calculate total cost with empty price history")
//	}
//
//	if history[0].Time.After(startTime) {
//		return 0.0, fmt.Errorf("Price history bound mismatch: first entry is after the start time")
//	}
//
//	// Calculate price
//	totalPrice := 0.0
//	for i := 0; i < len(history); i++ {
//		var start time.Time
//		var end time.Time
//
//		// if we are at the first price point, use the launch time to start
//		if i == 0 {
//			start = startTime
//		} else {
//			start = history[i].Time
//		}
//
//		// if we are the last price point, use the endTime (set as time.Now above)
//		if i == len(history) - 1 {
//			end = endTime
//		} else {
//			end = history[i + 1].Time
//		}
//
//		diff := end.Sub(start)
//		diffPrice := diff.Hours() * history[i].Price // price = $/hr
//		totalPrice += diffPrice
//	}
//
//	return totalPrice, nil
//}