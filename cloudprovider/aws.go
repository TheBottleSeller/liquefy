package cloudprovider

import (
    "errors"
    "strconv"
    "time"
    "fmt"
    "sync"
    "sort"

    log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"

    lq "bargain/liquefy/models"
    "github.com/aws/aws-sdk-go/service/iam"
    "strings"
)

const (
    AwsSshKeyName = "Liquefy"
    TagKey = "Liquefy"
    TagValue = "Liquefy"
    TagName = "Name"
)

var DefaultCIDR = "10.0.0.0/16"

type AZ string
type Region string
type InstanceType string

func (az *AZ) String() string {
    return string(*az)
}

func (az *AZ) StringPtr() *string {
    s := string(*az)
    return &s
}

func (az *AZ) GetRegion() Region {
    return Region(AzToRegion(az.String()))
}

func AzToRegion(az string) string {
    return az[:len(az) - 1]
}

func (region *Region) String() string {
    return string(*region)
}

func (region *Region) StringPtr() *string {
    s := string(*region)
    return &s
}

func (instanceType *InstanceType) String() string {
    return string(*instanceType)
}

func (instanceType *InstanceType) StringPtr() *string {
    s := string(*instanceType)
    return &s
}

// The start of the hard coded, static map of regions and AZs
var AWSRegionsToAZs = map[Region][]AZ{
    "us-east-1": []AZ{ "us-east-1a", "us-east-1b", "us-east-1c", "us-east-1e" },
    "us-west-1": []AZ{ "us-west-1a", "us-west-1b" },
    "us-west-2": []AZ{ "us-west-2a", "us-west-2b", "us-west-2c" },
}

var AllAvailabilityZones = []AZ{}

func init() {
    for _, azs := range AWSRegionsToAZs {
        for _, az := range azs {
            AllAvailabilityZones = append(AllAvailabilityZones, az)
        }
    }
}

type AwsCloud interface {
    // Account Mgmt
    SetupAwsAccountResources(*lq.AwsAccount) (*lq.AwsAccount, error)
    CreateSshKey(region Region) (*AwsSshKey, error)
    CreateVPC(region Region) (string, error)
    DestroyVPC(region Region, vpcID string) error
    CreateSubnet(region Region, az, vpcId string) (string, error)
    CreateSecurityGroup(region Region, vpcID string, sgname string) (string, error)
    VerifyPolicy() error
    CleanUpAwsAccount()

    // Spot Price Mgmt
    GetSpotPriceHistory(az AZ, instance InstanceType, startTime, endTime time.Time) ([]*ec2.SpotPrice, error)
    GetCurrentSpotPrices(az AZ, instanceTypes []InstanceType) (map[InstanceType]float64, error)

    // Spot Request Mgmt
    CreateSpotInstanceRequest(region Region, az string, imageId string, subnetId string, securityGroupName string,
        instanceType string, spotPrice float64, resourceId uint) (*ec2.SpotInstanceRequest, error)
    GetSpotRequestById(region Region, spotReqId string) (*ec2.SpotInstanceRequest, error)
    GetSpotRequestByInstanceId(region Region, instanceId string) (*ec2.SpotInstanceRequest, error)
    WaitForSpotRequestToFinish(spotReq *ec2.SpotInstanceRequest) (*ec2.SpotInstanceRequest, error)
    CancelSpotInstanceRequest(region Region, instanceId string) error

    // Instance Mgmt
    GetInstance(region Region, instanceId string) (*ec2.Instance, error)
    WaitForInstanceRunning(region Region, instance *ec2.Instance) (*ec2.Instance, error)
    WaitForInstanceTerminated(region Region, instance *ec2.Instance) (*ec2.Instance, error)
    WaitForIpAllocation(region Region, instance *ec2.Instance) (*ec2.Instance, error)
    GetAllActiveTaggedInstances(region Region, tag string) ([]*ec2.Instance, error)
    TerminateInstance(region Region, instanceId string) error

    // Tag Mgmt
    TagInstance(region Region, instanceId string, tag string) error
    TagResources(region Region, resources[] *string, tagKey string, tagValue string) (error)
}

type AwsSshKey struct {
    Region      string
    PublicKey   string
    PrivateKey  string
}

type awsCloud struct {
    awsKey    *string
    awsSecret *string
}

func NewAwsCloud(awsKey *string, awsSecret *string) AwsCloud {
	return &awsCloud{awsKey, awsSecret}
}

func (cloud *awsCloud) connect(region Region) *ec2.EC2 {
	config := &aws.Config{
		Credentials: credentials.NewStaticCredentials(*cloud.awsKey, *cloud.awsSecret, ""),
		Region:      aws.String(region.String()),
	}
    return ec2.New(session.New(config), config)
}

//TODO : Lock to prevent this from being double called ?
func (cloud *awsCloud) SetupAwsAccountResources(existingAccount *lq.AwsAccount) (*lq.AwsAccount, error) {
    var setupError error

    for region, azs := range AWSRegionsToAZs {
        // Create SSH key for region if we do not already have a key stored
        if existingAccount.GetSshPrivateKey(region.String()) == "" {
            key, err := cloud.CreateSshKey(region)
            if err != nil {
                setupError = lq.NewErrorf(setupError, "Failed creating ssh key for %s: %s", region.String(), err.Error())
                continue
            }

            log.Debugf("Created ssh key for %s", region.String())
            existingAccount.SetSshPrivateKey(region.String(), key.PrivateKey)
        }

        // If there is no existing vpc for the region, create it
        vpcId := existingAccount.GetVpcId(region.String())
        if existingAccount.GetVpcId(region.String()) == "" {
            if currentVPCID, err := cloud.CreateVPC(region); err != nil {
                // Do not continue creating subnets and security group, move on to next region
                setupError = lq.NewErrorf(setupError, "Failed creating ssh key for %s: %s", region.String(), err.Error())
                continue
            } else {
                log.Debugf("Created %s in %s", currentVPCID, region.String())
                existingAccount.SetVpcId(region.String(), currentVPCID)
                vpcId = currentVPCID
            }
        }

        // If security group does not exist, create one
        if existingAccount.GetSecurityGroupId(region.String()) == "" {
            if sgId, err := cloud.CreateSecurityGroup(region, vpcId, "Liquefy"); err != nil {
                setupError = lq.NewErrorf(setupError, "Failed creating security group: %s", err.Error())
                continue
            } else {
                log.Debugf("Created security group %s %s", sgId, "Liquefy")
                existingAccount.SetSecurityGroupId(region.String(), sgId)
                existingAccount.SetSecurityGroupName(region.String(), "Liquefy")
            }
        }

        // If there is no existing subnet for the az, create one
        for _, az := range azs {
            if existingAccount.GetSubnetId(az.String()) == "" {
                if subnetId, err := cloud.CreateSubnet(region, az.String(), vpcId); err != nil {
                    setupError = lq.NewErrorf(setupError, "Failed to create subnet in %s: %s", az.String(), err.Error())
                } else {
                    log.Debugf("Created %s in %s", subnetId, az)
                    existingAccount.SetSubnetId(az.String(), subnetId)
                }
            }
        }
    }

    return existingAccount, setupError
}

func (cloud *awsCloud) VerifyPolicy() error{
    config := &aws.Config{
        Credentials: credentials.NewStaticCredentials(*cloud.awsKey, *cloud.awsSecret, ""),
    }

    svc := iam.New(session.New(config))

    iamPolicyOutput, err := svc.ListPolicies(&iam.ListPoliciesInput{
        OnlyAttached: aws.Bool(true),
    })
    if err != nil {
        if strings.Contains(err.Error(), "not authorized to perform: iam:ListPolicies") {
            return lq.NewError("Unable to verify required policies", errors.New("IAMReadOnlyAccess was not found in policies"))
        }
        return lq.NewError("Unable to retrieve list of IAM policies", err)
    }

    var containsPolicy = func(item string) bool {
        for _, policy := range iamPolicyOutput.Policies {
            if *policy.PolicyName == item {
                return true
            }
        }
        return false
    }

    // If has full access move on
    if containsPolicy("AmazonEC2FullAccess") {
        return nil
    }

    // Otherwise we have specific access
    if ! containsPolicy("IAMReadOnlyAccess") {
        return lq.NewError("Unable to verify required policies", errors.New("IAMReadOnlyAccess was not found in policies"))
    }
    if ! containsPolicy("AmazonS3FullAccess") {
        return lq.NewError("Unable to verify required policies",errors.New("AmazonS3FullAccess was not found in policies"))
    }
    if ! containsPolicy("AmazonEC2FullAccess") {
        return lq.NewError("Unable to Verify required policies",errors.New("AmazonEC2FullAccess was not found in policies"))
    }

    return nil
}

func (cloud *awsCloud) CreateSshKey(region Region) (*AwsSshKey, error) {
    svc := cloud.connect(region)

    // Create a new key
    createParams := &ec2.CreateKeyPairInput{
        KeyName: aws.String(AwsSshKeyName),
    }
    out, err := svc.CreateKeyPair(createParams)
    if err != nil {
        log.Errorf("Failed creating ssh key for region %s", region.String())
        log.Error(err)
        return &AwsSshKey{}, err
    }
    keyPair := &AwsSshKey{
        Region: region.String(),
        PublicKey: *out.KeyFingerprint,
        PrivateKey: *out.KeyMaterial,
    }
    return keyPair, nil
}

func (cloud *awsCloud) CreateVPC(region Region) (string, error) {
	svc := cloud.connect(region)

    cleanupVPC := func(vpcId *string) {
        internalError := cloud.DestroyVPC(region, *vpcId)
        if internalError != nil {
            log.Errorf("Failed cleaning up VPC %s", *vpcId)
            log.Error(internalError)
        }
    }

    cleanupIg := func(igId *string) {
        log.Debugf("Destroying %s", *igId)
        _, internalError := svc.DeleteInternetGateway(&ec2.DeleteInternetGatewayInput{
            InternetGatewayId: igId,
        })
        if internalError != nil {
            log.Errorf("Failed cleaning up Internet Gateway %s", *igId)
            log.Error(internalError)
        }
    }

    cleanupRouteTable := func(routeTableId *string) {
        log.Debugf("Destroying %s", *routeTableId)
        _, internalError := svc.DeleteRouteTable(&ec2.DeleteRouteTableInput{
            RouteTableId: routeTableId,
        })
        if internalError != nil {
            log.Errorf("Failed cleaning up Route Table %s", *routeTableId)
            log.Error(internalError)
        }
    }

	vpcInput := &ec2.CreateVpcInput{
		CidrBlock: &DefaultCIDR, // Required
	}
	vpcResp, err := svc.CreateVpc(vpcInput)
	if err != nil {
		return "", err
	}
    log.Debugf("Created %s", *vpcResp.Vpc.VpcId)

    // To connect to the internet, VPCs need
    // 1. An internet gateway attached to them
    // 2. An entry added to it's routing table
    igResp, err := svc.CreateInternetGateway(&ec2.CreateInternetGatewayInput{})
    if err != nil {
        cleanupVPC(vpcResp.Vpc.VpcId)
        return "", err
    }
    log.Debugf("Created %s", *igResp.InternetGateway.InternetGatewayId)

    _, err = svc.AttachInternetGateway(&ec2.AttachInternetGatewayInput{
        InternetGatewayId: igResp.InternetGateway.InternetGatewayId,
        VpcId: vpcResp.Vpc.VpcId,
    })
    if err != nil {
        cleanupIg(igResp.InternetGateway.InternetGatewayId)
        cleanupVPC(vpcResp.Vpc.VpcId)
        return "", err
    }
    log.Debugf("Attached %s to %s", *igResp.InternetGateway.InternetGatewayId, *vpcResp.Vpc.VpcId)

    routeTablesResp, err := svc.DescribeRouteTables(&ec2.DescribeRouteTablesInput{
        Filters: []*ec2.Filter{
            &ec2.Filter{
                Name: aws.String("vpc-id"),
                Values: []*string { vpcResp.Vpc.VpcId },
            },
        },
    })
    if len(routeTablesResp.RouteTables) == 0 {
        err = fmt.Errorf("No route table found for %s and %s", *vpcResp.Vpc.VpcId,
            *igResp.InternetGateway.InternetGatewayId)
        log.Error(err)
        cleanupIg(igResp.InternetGateway.InternetGatewayId)
        cleanupVPC(vpcResp.Vpc.VpcId)
        return "", err
    }

    routeId := routeTablesResp.RouteTables[0].RouteTableId
    _, err = svc.CreateRoute(&ec2.CreateRouteInput{
        RouteTableId: routeId,
        GatewayId: igResp.InternetGateway.InternetGatewayId,
        DestinationCidrBlock: aws.String("0.0.0.0/0"),
    })
    if err != nil {
        log.Error("Failed creating route in %s with %s for %s", *routeId, *vpcResp.Vpc.VpcId,
            *igResp.InternetGateway.InternetGatewayId)
        cleanupRouteTable(routeId)
        cleanupIg(igResp.InternetGateway.InternetGatewayId)
        cleanupVPC(vpcResp.Vpc.VpcId)
        return "", err
    }

    cloud.TagResources(region, []*string{ vpcResp.Vpc.VpcId }, TagKey, TagValue)
    cloud.TagResources(region, []*string{ vpcResp.Vpc.VpcId }, TagName, TagValue)

    return *vpcResp.Vpc.VpcId, err
}

func (cloud *awsCloud) DestroyVPC(region Region, vpcid string) error {
    log.Infof("Destroying %s in %s", vpcid, region.String())
	svc := cloud.connect(region)
	params := &ec2.DeleteVpcInput{
		VpcId: &vpcid,
	}
	_, err := svc.DeleteVpc(params)
	if err != nil {
		return err
	}
	return err
}

func (cloud *awsCloud) CreateSubnet(region Region, az, vpcId string) (string, error) {
    index := -1
    for i, loopAz := range AWSRegionsToAZs[Region(region)] {
        if az == loopAz.String() {
            index = i
            break
        }
    }
    
    if index == -1 {
        return "", fmt.Errorf("%s is not an az for %s", az, region.String())
    }

	svc := cloud.connect(region)

    //We need to partition the IP spcace in the VPC into the subnets
    //We can start by doing this equally and then change it later
    //We partition by using the index of the az in the AWSRegionsToAZ map
    subnetCIDR := "10.0." + strconv.Itoa(index) + ".0/24" //
    params := &ec2.CreateSubnetInput{
        CidrBlock:        &subnetCIDR,
        VpcId:            &vpcId,
        AvailabilityZone: &az,
    }
    sub, err := svc.CreateSubnet(params)
    if err != nil {
        log.Error(err.Error())
        return "", err
    }

    cloud.TagResources(region, []*string{ sub.Subnet.SubnetId }, TagKey , TagValue)
    cloud.TagResources(region, []*string{ sub.Subnet.SubnetId }, TagName, TagValue)

    return *sub.Subnet.SubnetId, nil
}

func (cloud *awsCloud) CreateSecurityGroup(region Region, vpcId string, sgName string) (string, error) {
	svc := cloud.connect(region)
	request := &ec2.DescribeSecurityGroupsInput{}
	filters := []*ec2.Filter{
		newEc2Filter("group-name", sgName),
		newEc2Filter("vpc-id", vpcId),
	}

	//Attach filters
	request.Filters = filters
	decSecurityGroups, err := svc.DescribeSecurityGroups(request)
	if err != nil {
		return "", err
	}

	if len(decSecurityGroups.SecurityGroups) >= 1 {
		log.Warnf("Found %d security groups with name %s", len(decSecurityGroups.SecurityGroups), sgName)

        // Take the first security group with that name
        sg := decSecurityGroups.SecurityGroups[0]
        err := cloud.authorizeSecurityGroup(svc, *sg.GroupId)
        return *sg.GroupId, err
	}

	createRequest := &ec2.CreateSecurityGroupInput{
        VpcId:          &vpcId,
        GroupName:      &sgName,
        Description:    &sgName,
    }
	createResponse, err := svc.CreateSecurityGroup(createRequest)
	if err != nil {
        log.Error("Failed creating security group in %s", region.String())
        log.Error(err)
        return "", err
	}

	if *createResponse.GroupId == "" {
		log.Errorf("Created security group, but id was not returned")
		return "", errors.New("Emptry security group ID returned")
	}

    err = cloud.authorizeSecurityGroup(svc, *createResponse.GroupId)

    //TODO Capture Error of a failed Tag
    cloud.TagResources(region, []*string{ createResponse.GroupId }, TagKey , TagValue)
    cloud.TagResources(region, []*string{ createResponse.GroupId }, TagName, TagValue)

	return *createResponse.GroupId, err
}

// Authorize security group to allow all inbound TCP traffic
func (cloud *awsCloud) authorizeSecurityGroup(svc *ec2.EC2, sgId string) error {
    authReq := &ec2.AuthorizeSecurityGroupIngressInput{
        GroupId: aws.String(sgId),
        IpPermissions: []*ec2.IpPermission{
              &ec2.IpPermission {
                  IpProtocol: aws.String("tcp"),
                  FromPort: aws.Int64(0),
                  ToPort: aws.Int64(65535),
                  IpRanges: []*ec2.IpRange{
                      &ec2.IpRange{
                          CidrIp: aws.String("0.0.0.0/0"),
                      },
                  },
              },
        },
    }
    _, err := svc.AuthorizeSecurityGroupIngress(authReq)
    return err
}

func newEc2Filter(name string, value string) *ec2.Filter {
	filter := &ec2.Filter{
		Name: aws.String(name),
		Values: []*string{
			aws.String(value),
		},
	}
	return filter
}

func (cloud *awsCloud) TagResources(region Region, resources[] *string, tagKey string, tagValue string) (error) {
	//Note : Should pass in resource ID's for resrouces it wants to tag

	svc := cloud.connect(region)
	params := &ec2.CreateTagsInput{
		Resources: resources,
		Tags: []*ec2.Tag{
			&ec2.Tag{
				Key:   aws.String(tagKey),
				Value: aws.String(tagValue),
			},
		},
	}
	_, err := svc.CreateTags(params)
	if err != nil {
        err = lq.NewErrorf(err, "Failed tagging resource with (%s, %s)", tagKey, tagValue)
		log.Error(err)
		return err
	}
	return nil
}

type SpotPrices []*ec2.SpotPrice

func (slice SpotPrices) Len() int {
    return len(slice)
}

func (slice SpotPrices) Less(i, j int) bool {
    return slice[i].Timestamp.Before(*slice[j].Timestamp)
}

func (slice SpotPrices) Swap(i, j int) {
    slice[i], slice[j] = slice[j], slice[i]
}

func (cloud *awsCloud) GetCurrentSpotPrices(az AZ, instanceTypes []InstanceType) (map[InstanceType]float64, error) {
    now := time.Now()
    instancePrices := make(map[InstanceType]float64)

    instanceSpotPrices, err := cloud.getSpotPriceHistoryImpl(az, instanceTypes, now, now)
    if err != nil {
        return instancePrices, lq.NewError("Failed getting current spot prices", err)
    }

    for instance, spotPrices := range instanceSpotPrices {
        if len(spotPrices) == 0 {
            //log.Warnf("Could not get current spot price for instance %s in %s", string(instance), az.String())
        } else {
            price, err := strconv.ParseFloat(*spotPrices[0].SpotPrice, 64)
            if err != nil {
                log.Warnf("Recieved bad spot price from AWS %s", *spotPrices[0].SpotPrice)
            } else {
                instancePrices[instance] = price
            }
        }
    }

    return instancePrices, nil
}

func (cloud *awsCloud) GetSpotPriceHistory(az AZ, instance InstanceType, startTime, endTime time.Time) ([]*ec2.SpotPrice, error) {
    //log.Debugf("Getting spot price history for %s in %s. %s to %s", az.String(), instance.String(),
    //    startTime.UTC().String(), endTime.UTC().String())
    spotPrices, err := cloud.getSpotPriceHistoryImpl(az, []InstanceType{ instance }, startTime, endTime)
    if err != nil {
        return []*ec2.SpotPrice{}, err
    }

    prices, ok := spotPrices[instance]
    if !ok {
        return []*ec2.SpotPrice{}, fmt.Errorf("Failed getting spot price history for instance %s", instance.String())
    }

    sort.Stable(SpotPrices(prices))

//    coversStart := prices[0].Timestamp.Before(startTime)
//    upperBound := startTime
//    lowerBound := upperBound.Add(time.Duration(-1) * time.Hour)
//
//    for !coversStart {
//        log.Infof("Searching %s - %s", lowerBound.UTC().String(), upperBound.UTC().String())
//        spotPrices, err = cloud.getSpotPriceHistoryImpl(az, []InstanceType{ instance }, lowerBound, upperBound)
//        if err != nil {
//            return []*ec2.SpotPrice{}, err
//        }
//
//        lowerPrices, ok := spotPrices[instance]
//        if !ok {
//            return []*ec2.SpotPrice{}, fmt.Errorf("Failed getting spot price history for instance %s", instance.String())
//        }
//
//        coversStart = lowerPrices[len(lowerPrices) - 1].Timestamp.Before(startTime)
//
//        if coversStart {
//            prices = append([]*ec2.SpotPrice{ lowerPrices[len(lowerPrices) - 1] }, prices...)
//        } else {
//            upperBound = lowerBound
//            lowerBound = upperBound.Add(time.Duration(-1) * time.Hour)
//        }
//    }

    return prices, nil
}

//
func (cloud *awsCloud) getSpotPriceHistoryImpl(az AZ, instances []InstanceType, startTime, endTime time.Time) (map[InstanceType][]*ec2.SpotPrice, error) {
    svc := cloud.connect(az.GetRegion())

    instancePrices := make(map[InstanceType][]*ec2.SpotPrice)

    // We need to convert the []InstanceType -> []*string to pass to AWS api
    instancePtrs := make([]*string, len(instances))
    for i := 0; i < len(instances); i++ {
        instancePtrs[i] = instances[i].StringPtr()
        instancePrices[instances[i]] = []*ec2.SpotPrice{}
    }

    input := &ec2.DescribeSpotPriceHistoryInput{
        AvailabilityZone: az.StringPtr(),
        StartTime: &startTime,
        EndTime: &endTime,
        InstanceTypes: instancePtrs,
        ProductDescriptions: []*string { aws.String("Linux/UNIX") },
    }
    output, err := svc.DescribeSpotPriceHistory(input)
    if err != nil {
        log.Error(err)
        return instancePrices, lq.NewError("Failed getting spot price history", err)
    }
    for _, price := range output.SpotPriceHistory {
        instancePrices[InstanceType(*price.InstanceType)] = append(instancePrices[InstanceType(*price.InstanceType)], price)
    }

    // the AWS api is paginated and so we must keep track of the next token to ensure
    // that we get all of the results
    for output.NextToken != nil && *output.NextToken != "" {
        input = &ec2.DescribeSpotPriceHistoryInput{
            NextToken: output.NextToken,
        }
        output, err = svc.DescribeSpotPriceHistory(input)
        if err != nil {
            log.Error(err)
            return instancePrices, lq.NewError("Failed getting spot price history", err)
        }
        for _, price := range output.SpotPriceHistory {
            instancePrices[InstanceType(*price.InstanceType)] = append(instancePrices[InstanceType(*price.InstanceType)], price)
        }
    }

    return instancePrices, nil
}

func (cloud *awsCloud) CleanUpAwsAccount() {
    tagFilters := []*ec2.Filter{
        &ec2.Filter{
            Name: aws.String("tag-key"),
            Values: []*string{aws.String("Liquefy") },
        },
    }
    for region := range AWSRegionsToAZs {
        fmt.Printf("!!!! Cleaning region %s !!!!\n", region.String())
        svc := cloud.connect(region)

        // Delete all Liqefy instances
        instanceResp, err := svc.DescribeInstances(&ec2.DescribeInstancesInput{
            Filters: tagFilters,
        })
        if err != nil {
            log.Error(err)
        }

        fmt.Println("Cleaning instances")
        var wg sync.WaitGroup
        for _, reservation := range instanceResp.Reservations {
            for _, instance :=  range reservation.Instances {
                fmt.Printf("Destroying instance %s\n", *instance.InstanceId)
                wg.Add(1)
                go func() {
                    defer wg.Done()
                    _, err = svc.TerminateInstances(&ec2.TerminateInstancesInput{
                        InstanceIds: []*string{instance.InstanceId },
                    })
                    if err != nil {
                        log.Error(err)
                    }
                    instance, err = cloud.WaitForInstanceTerminated(region, instance)
                    if err != nil {
                        log.Error(err)
                    }
                }()
            }
        }
        log.Debugf("Waiting for instances to terminate")
        wg.Wait()

        resp, err := svc.DescribeVpcs(&ec2.DescribeVpcsInput{
            Filters: tagFilters,
        })
        if err != nil {
            log.Error(err)
        }

        for _, vpc := range resp.Vpcs {
            respIg, err := svc.DescribeInternetGateways(&ec2.DescribeInternetGatewaysInput{
                Filters: []*ec2.Filter {
                    &ec2.Filter {
                        Name: aws.String("attachment.vpc-id"),
                        Values: []*string{ vpc.VpcId },
                    },
                },
            })
            if err != nil {
                log.Error(err)
            }
            for _, ig := range respIg.InternetGateways {
                routeResp, err := svc.DescribeRouteTables(&ec2.DescribeRouteTablesInput{
                    Filters: []*ec2.Filter {
                        &ec2.Filter {
                            Name: aws.String("route.gateway-id"),
                            Values: []*string{ ig.InternetGatewayId },
                        },
                    },
                })
                if err != nil {
                    log.Error(err)
                }

                for _, routeTable := range routeResp.RouteTables {
                    log.Debugf("Destroying %s", *routeTable.RouteTableId)
                    _, err = svc.DeleteRouteTable(&ec2.DeleteRouteTableInput{
                        RouteTableId: routeTable.RouteTableId,
                    })
                    if err != nil {
                        log.Error(err)
                    }
                }

                _, err = svc.DetachInternetGateway(&ec2.DetachInternetGatewayInput{
                    InternetGatewayId: ig.InternetGatewayId,
                    VpcId: vpc.VpcId,
                })
                if err != nil {
                    log.Error(err)
                }

                log.Debugf("Destroying %s", *ig.InternetGatewayId)
                _, err = svc.DeleteInternetGateway(&ec2.DeleteInternetGatewayInput{
                    InternetGatewayId: ig.InternetGatewayId,
                })
                if err != nil {
                    log.Error(err)
                }
            }
        }

        respSg, err := svc.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
            Filters: tagFilters,
        })
        if err != nil {
            log.Error(err)
        }
        for _, sg := range respSg.SecurityGroups {
            log.Debugf("Destroying %s", *sg.GroupId)
            _, err = svc.DeleteSecurityGroup(&ec2.DeleteSecurityGroupInput{
                GroupId: sg.GroupId,
            })
            if err != nil {
                log.Error(err)
            }
        }
        respSubnets, err := svc.DescribeSubnets(&ec2.DescribeSubnetsInput{
            Filters: tagFilters,
        })
        if err != nil {
            log.Error(err)
        }
        for _, subnet := range respSubnets.Subnets {
            log.Debugf("Destroying %s", *subnet.SubnetId)
            _, err = svc.DeleteSubnet(&ec2.DeleteSubnetInput{
                SubnetId: subnet.SubnetId,
            })
            if err != nil {
                log.Error(err)
            }
        }

        for _, vpc := range resp.Vpcs {
            err := cloud.DestroyVPC(region, *vpc.VpcId)
            if err != nil {
                log.Error(err)
            } else {
                break
            }
        }

        // Delete the key pair if it exists
        deleteParams := &ec2.DeleteKeyPairInput{
            KeyName: aws.String(AwsSshKeyName),
        }
        log.Debugf("Destroying key pair %s", AwsSshKeyName)
        _, err = svc.DeleteKeyPair(deleteParams)
        if err != nil {
            log.Error("Failed deleting key pair")
            log.Error(err)
        }
    }
}

func (cloud *awsCloud) CreateSpotInstanceRequest(region Region, az string, imageId string, subnetId string,
    securityGroupId string, instanceType string, spotPrice float64, resourceId uint) (*ec2.SpotInstanceRequest, error) {
    log.Infof("Creating spot instance request for resource: %d", resourceId)
    svc := cloud.connect(region)

    // TODO figure out how to tag these instances
    params := &ec2.RequestSpotInstancesInput{
        DryRun:                 aws.Bool(false),
        Type:                   aws.String(ec2.SpotInstanceTypeOneTime),
        SpotPrice:              aws.String(fmt.Sprintf("%f", spotPrice)),
        AvailabilityZoneGroup:  aws.String(az),
        //ClientToken:            aws.String(fmt.Sprintf("%d", resourceId)), // ensures that request is idempotent
        InstanceCount:          aws.Int64(1),
        LaunchSpecification: &ec2.RequestSpotLaunchSpecification{
            ImageId:      aws.String(imageId),
            InstanceType: aws.String(instanceType),
            Monitoring: &ec2.RunInstancesMonitoringEnabled{
                Enabled: aws.Bool(true), // Required
            },
            Placement: &ec2.SpotPlacement{
                AvailabilityZone: aws.String(az),
            },
            KeyName: aws.String(AwsSshKeyName),
            NetworkInterfaces: []*ec2.InstanceNetworkInterfaceSpecification{
                &ec2.InstanceNetworkInterfaceSpecification{
                    AssociatePublicIpAddress: aws.Bool(true),
                    DeviceIndex: aws.Int64(0),
                    SubnetId: aws.String(subnetId),
                    Groups: []*string { aws.String(securityGroupId) },
                    DeleteOnTermination: aws.Bool(true),
                },
            },
            BlockDeviceMappings: []*ec2.BlockDeviceMapping{
                &ec2.BlockDeviceMapping{
                    DeviceName: aws.String("/dev/sda1"),
                    Ebs: &ec2.EbsBlockDevice{
                        DeleteOnTermination: aws.Bool(true),
                        VolumeSize: aws.Int64(50),
                    },
                },
            },
        },
    }

    resp, err := svc.RequestSpotInstances(params)

    // This should handle a user specific error like:
    // Cause :: MaxSpotInstanceCountExceeded: Max spot instance count exceeded\n\tstatus code: 400, request id: \n
    // And call MarkMarketUnavailable
    if err != nil {
        return &ec2.SpotInstanceRequest{}, err
    } else if len(resp.SpotInstanceRequests) == 0 {
        return &ec2.SpotInstanceRequest{}, fmt.Errorf("Wadafuq spot instance request lost")
    }

    return resp.SpotInstanceRequests[0], nil
}

func (cloud *awsCloud) GetSpotRequestById(region Region, spotReqId string) (*ec2.SpotInstanceRequest, error) {
    svc := cloud.connect(region)

    params := &ec2.DescribeSpotInstanceRequestsInput{
        SpotInstanceRequestIds: []*string{ aws.String(spotReqId) },
    }
    resp, err := svc.DescribeSpotInstanceRequests(params)
    if err != nil {
        return &ec2.SpotInstanceRequest{}, err
    } else if len(resp.SpotInstanceRequests) == 0 {
        return &ec2.SpotInstanceRequest{}, fmt.Errorf("Wadafuq! AWS did not send back spot request")
    } else {
        return resp.SpotInstanceRequests[0], nil
    }
}

func (cloud *awsCloud) GetSpotRequestByInstanceId(region Region, instanceId string) (*ec2.SpotInstanceRequest, error) {
    svc := cloud.connect(region)

    params := &ec2.DescribeSpotInstanceRequestsInput{
        Filters: []*ec2.Filter{
            &ec2.Filter{
                Name:   aws.String("instance-id"),
                Values: []*string{aws.String(instanceId)},
            },
        },
    }

    resp, err := svc.DescribeSpotInstanceRequests(params)
    if err != nil {
        return &ec2.SpotInstanceRequest{}, err
    } else if len(resp.SpotInstanceRequests) == 0 {
        return &ec2.SpotInstanceRequest{}, fmt.Errorf("Spot instance request not found")
    } else {
        return resp.SpotInstanceRequests[0], nil
    }
}

func (cloud *awsCloud) WaitForSpotRequestToFinish(spotReq *ec2.SpotInstanceRequest) (*ec2.SpotInstanceRequest, error) {
    region := Region(lq.AZtoRegion(*spotReq.AvailabilityZoneGroup))
    pollTime := time.Duration(5) * time.Second
    timeout := time.Duration(3) * time.Minute
    timeoutTimer := time.NewTimer(timeout)

    // If the spot request does not finish successfully, then mark the market as unavailable to trigger a
    // downstream backoff
    success := false
    defer func() {
        if !success {
            // Mark market as unavailable
            MarkMarketUnavailable(AZ(*spotReq.AvailabilityZoneGroup),
                InstanceType(*spotReq.LaunchSpecification.InstanceType))

            // Cancel the failed spot request
            err := cloud.cancelSpotInstanceRequestByRequestId(region, *spotReq.SpotInstanceRequestId)
            if err != nil {
                log.Error("Failed cancelling the failed spot request: ", err)
            }
        }
    }()

    for {
        select {
        case _ = <-timeoutTimer.C:
            return spotReq, fmt.Errorf("Spot request timed out with status %s", *spotReq.Status.Code)

        default:
            done, resetTimeout, err := cloud.handleSpotReqStatus(spotReq.Status)

            if done {
                success = true
                timeoutTimer.Stop()
                return spotReq, err
            }

            if err != nil {
                timeoutTimer.Stop()
                return spotReq, err
            }

            if resetTimeout {
                _ = timeoutTimer.Reset(timeout)
            }

            time.Sleep(pollTime)
            spotReq, err = cloud.GetSpotRequestById(region, *spotReq.SpotInstanceRequestId)
        }
    }

    success = true
    return spotReq, nil
}

func (cloud *awsCloud) handleSpotReqStatus(status *ec2.SpotInstanceStatus) (done bool, resetTimeout bool, err error) {
    done = false
    resetTimeout = true
    switch *status.Code {
        case "pending-evaluation":
            // the state right after creation, keep waiting
            break
        case "not-scheduled-yet":
            // the request can stay in this state perpetually
            resetTimeout = false
            break
        case "pending-fulfillment":
            // things are looking good!
            // constraints and spot price are acceptable
            // keep looping should be fulfilled soon
            break
        case "fulfilled":
            done = true
            break
        case "launch-group-constraint":
            fallthrough
        case "az-group-constraint":
            fallthrough
        case "capacity-not-available":
            fallthrough
        case "capacity-oversubscribed":
            fallthrough
        case "price-too-low":
            fallthrough
        case "schedule-expired":
            fallthrough
        case "canceled-before-fulfillment":
            fallthrough
        case "system-error":
            fallthrough
        case "request-canceled-and-instance-running":
            fallthrough
        case "marked-for-termination":
            fallthrough
        case "instance-terminated-by-price":
            fallthrough
        case "instance-terminated-by-user":
            fallthrough
        case "instance-terminated-no-capacity":
            fallthrough
        case "instance-terminated-capacity-oversubscribed":
            fallthrough
        case "instance-terminated-launch-group-constraint":
            fallthrough
        case "bad-parameters":
            err = fmt.Errorf("Code: %s, Msg: %s", *status.Code, *status.Message)
            break
        default:
            err = fmt.Errorf("Unknown spot request error: %s", status)
            break
    }
    return
}

func (cloud *awsCloud) CancelSpotInstanceRequest(region Region, instanceId string) error {
    spotReq, err := cloud.GetSpotRequestByInstanceId(region, instanceId)
    if err != nil {
        return err
    }

    // TODO: Fix making two separate connections to AWS (one from the above call and one from below)
    return cloud.cancelSpotInstanceRequestByRequestId(region, *spotReq.SpotInstanceRequestId)
}

func (cloud *awsCloud) cancelSpotInstanceRequestByRequestId(region Region, spotRequestId string) error {
    svc := cloud.connect(region)

    cancelParams := &ec2.CancelSpotInstanceRequestsInput{
        SpotInstanceRequestIds: []*string { aws.String(spotRequestId) },
    }
    _, err := svc.CancelSpotInstanceRequests(cancelParams)
    return err
}

func (cloud *awsCloud) TerminateInstance(region Region, instanceId string) error {
    svc := cloud.connect(region)

    resp, err := svc.TerminateInstances(&ec2.TerminateInstancesInput{
        InstanceIds: []*string{ aws.String(instanceId) },
    })
    if err != nil {
        return err
    }

    if len(resp.TerminatingInstances) != 1 {
        return fmt.Errorf("Terminating instance not returned")
    }
    return nil
}

func (cloud *awsCloud) GetInstance(region Region, instanceId string) (*ec2.Instance, error) {
    svc := cloud.connect(region)
    params := &ec2.DescribeInstancesInput{
        InstanceIds: []*string{ &instanceId },
    }
    resp, err := svc.DescribeInstances(params)
    if err != nil {
        return &ec2.Instance{}, err
    }

    if len(resp.Reservations) == 0 ||
        len(resp.Reservations[0].Instances) == 0 ||
        resp.Reservations[0].Instances[0] == nil {
        log.Error(resp)
        return &ec2.Instance{}, fmt.Errorf("Instance information not returned")
    }

    return resp.Reservations[0].Instances[0], nil
}

func (cloud *awsCloud) WaitForInstanceRunning(region Region, instance *ec2.Instance) (*ec2.Instance, error) {
    stateReached := func(state string) (done bool, err error) {
        switch state {
        // running should finish
        case "running":
            done = true
            break
        // pending and rebooting should keep waiting
        case "pending":
            fallthrough
        case "rebooting":
            done = false
            break
        // stopping, stopped, shutting down, and terminated should fail with an error
        case "stopping":
            fallthrough
        case "stopped":
            fallthrough
        case "shutting-down":
            fallthrough
        case "terminated":
            err = fmt.Errorf("Instance is stopped, will not start running")
            break
        default:
            err = fmt.Errorf("Unknown instance state: %s", state)
            break
        }
        return
    }

    instance, err := cloud.waitForInstanceState(region, instance, stateReached)
    if err != nil {
        return instance, lq.NewError("Failed to wait for instance running", err)
    }
    return instance, nil
}

func (cloud *awsCloud) WaitForInstanceTerminated(region Region, instance *ec2.Instance) (*ec2.Instance, error) {
    stateReached := func(state string) (done bool, err error) {
        switch state {
        // terminated should finish
        case "terminated":
            done = true
            break
        // all other cases should keep waiting
        case "pending":
            fallthrough
        case "rebooting":
            fallthrough
        case "stopping":
            fallthrough
        case "stopped":
            fallthrough
        case "shutting-down":
            fallthrough
        case "running":
            done = false
            break
        default:
            err = fmt.Errorf("Unknown instance state: %s", state)
            break
        }
        return
    }

    instance, err := cloud.waitForInstanceState(region, instance, stateReached)
    if err != nil {
        return instance, lq.NewError("Failed to wait for instance terminated", err)
    }
    return instance, nil
}

func (cloud *awsCloud) waitForInstanceState(region Region, instance *ec2.Instance, stateReached func(string) (bool, error)) (*ec2.Instance, error) {
    pollTime := time.Duration(5) * time.Second
    timeout := time.Duration(5) * time.Minute
    timeoutTimer := time.NewTimer(timeout)
    for {
        select {
        case _ = <-timeoutTimer.C:
            return instance, fmt.Errorf("Timedout out. Instance state is %s", *instance.State.Name)

        default:
            log.Debugf("Polling instance state: %s", *instance.State.Name)
            done, err := stateReached(*instance.State.Name)

            if done || err != nil {
                timeoutTimer.Stop()
                return instance, err
            }

            time.Sleep(pollTime)
            instance, err = cloud.GetInstance(region, *instance.InstanceId)
        }
    }

    return instance, nil
}

func (cloud *awsCloud) WaitForIpAllocation(region Region, instance *ec2.Instance) (*ec2.Instance, error) {
    var err error
    pollTime := time.Duration(2) * time.Second
    timeout := time.Duration(5) * time.Minute
    timeoutTimer := time.NewTimer(timeout)
    for {
        select {
        case _ = <-timeoutTimer.C:
            timeoutTimer.Stop()
            return instance, fmt.Errorf("Timedout before ip address was allocated for instance %s", *instance.InstanceId)

        default:
            if instance.PublicIpAddress != nil {
                log.Infof("Instance %s got ip address %s", *instance.InstanceId, *instance.PublicIpAddress)
                timeoutTimer.Stop()
                return instance, nil
            } else {
                log.Debugf("Waiting for ip to be allocated for instance %s", *instance.InstanceId)
                time.Sleep(pollTime)
                instance, err = cloud.GetInstance(region, *instance.InstanceId)
                if err != nil {
                    log.Errorf("Failed to wait for ip address for instance %s", *instance.InstanceId)
                    log.Error(err)
                    return instance, err
                }
            }
        }
    }

    return instance, err
}

func (cloud *awsCloud) GetAllActiveTaggedInstances(region Region, tag string) ([]*ec2.Instance, error) {
    svc := cloud.connect(region)
    instances := []*ec2.Instance{}

    resp, err := svc.DescribeInstances(&ec2.DescribeInstancesInput{
        Filters: []*ec2.Filter{
            &ec2.Filter{
                Name:   aws.String("tag:" + tag),
                Values: []*string{ aws.String(tag) },
            },
            &ec2.Filter{
                Name: aws.String("instance-state-name"),
                Values: []*string{
                    aws.String("pending"), aws.String("running"), aws.String("stopping"), aws.String("stopped"),
                },
            },
        },
    })

    if err != nil {
        return instances, err
    }

    for i := 0; i < len(resp.Reservations); i++ {
        for j := 0; j < len(resp.Reservations[i].Instances); j++ {
            instances = append(instances, resp.Reservations[i].Instances[j])
        }
    }

    for resp.NextToken != nil {
        resp, err = svc.DescribeInstances(&ec2.DescribeInstancesInput{
            NextToken: resp.NextToken,
        })

        if err != nil {
            return instances, err
        }

        for i := 0; i < len(resp.Reservations); i++ {
            for j := 0; j < len(resp.Reservations[i].Instances); j++ {
                instances = append(instances, resp.Reservations[i].Instances[j])
            }
        }
    }

    return instances, nil
}

func (cloud *awsCloud) TagInstance(region Region, instanceId string, tag string) error {
    svc := cloud.connect(region)
    _, err := svc.CreateTags(&ec2.CreateTagsInput{
        Resources: []*string{ aws.String(instanceId) },
        Tags: []*ec2.Tag{
            &ec2.Tag{
                Key:   aws.String(tag),
                Value: aws.String(tag),
            },
        },
    })
    return err
}

var GB = 1024.0

type InstanceInfo struct {
    Cpu     float64
    Memory  float64
    Disk    float64
    Gpu     float64
}

// TODO These should be autogenerated

func FindPossibleInstances(cpu, memory, gpu, disk float64) []InstanceType {
    instances := []InstanceType{}
    for instance, info := range AvailableInstances {
        if cpu <= info.Cpu &&
        memory <= info.Memory &&
        disk <= info.Disk &&
        gpu <= info.Gpu {
            instances = append(instances, InstanceType(instance))
        }
    }
    return instances
}


// We are missing the t2.nano unless we update the SDK
var AvailableInstances = map[InstanceType]InstanceInfo {
    ec2.InstanceTypeT2Micro: InstanceInfo{
        Cpu:    1.0,
        Memory: 0.5 * GB,
        Disk:   0.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeT2Small: InstanceInfo{
        Cpu:    1.0,
        Memory: 2.0 * GB,
        Disk:   0.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeT2Medium: InstanceInfo{
        Cpu:    2.0,
        Memory: 4.0 * GB,
        Disk:   0.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeT2Large: InstanceInfo{
        Cpu:    2.0,
        Memory: 8.0 * GB,
        Disk:   0.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeM4Large: InstanceInfo{
        Cpu:    2.0,
        Memory: 8.0 * GB,
        Disk:   0.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeM4Xlarge: InstanceInfo{
        Cpu:    4.0,
        Memory: 16.0 * GB,
        Disk:   0.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeM42xlarge: InstanceInfo{
        Cpu:    8.0,
        Memory: 32.0 * GB,
        Disk:   0.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeM44xlarge: InstanceInfo{
        Cpu:    16.0,
        Memory: 64.0 * GB,
        Disk:   0.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeM410xlarge: InstanceInfo{
        Cpu:    40.0,
        Memory: 160.0 * GB,
        Disk:   0.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeM3Medium: InstanceInfo{
        Cpu:    1.0,
        Memory: 3.75 * GB,
        Disk:   1 * 4.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeM3Large: InstanceInfo{
        Cpu:    2.0,
        Memory: 7.5 * GB,
        Disk:   1 * 32.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeM3Xlarge: InstanceInfo{
        Cpu:    4.0,
        Memory: 15.0 * GB,
        Disk:   2 * 40.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeM32xlarge: InstanceInfo{
        Cpu:    8.0,
        Memory: 30.0 * GB,
        Disk:   2 * 80.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeC4Large: InstanceInfo{
        Cpu:    2.0,
        Memory: 3.75 * GB,
        Disk:   0.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeC4Xlarge: InstanceInfo{
        Cpu:    4.0,
        Memory: 7.5 * GB,
        Disk:   0.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeC42xlarge: InstanceInfo{
        Cpu:    8.0,
        Memory: 15.0 * GB,
        Disk:   0.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeC44xlarge: InstanceInfo{
        Cpu:    16.0,
        Memory: 30.0 * GB,
        Disk:   0.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeC48xlarge: InstanceInfo{
        Cpu:    36.0,
        Memory: 60.0 * GB,
        Disk:   0.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeC3Large: InstanceInfo{
        Cpu:    2.0,
        Memory: 3.75 * GB,
        Disk:   2 * 16.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeC3Xlarge: InstanceInfo{
        Cpu:    5.0,
        Memory: 7.5 * GB,
        Disk:   2 * 40.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeC32xlarge: InstanceInfo{
        Cpu:    8.0,
        Memory: 15.0 * GB,
        Disk:   2 * 80.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeC34xlarge: InstanceInfo{
        Cpu:    16.0,
        Memory: 30.0 * GB,
        Disk:   2 * 160.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeC38xlarge: InstanceInfo{
        Cpu:    32.0,
        Memory: 60.0 * GB,
        Disk:   2 * 320.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeG22xlarge: InstanceInfo{
        Cpu:    8.0,
        Memory: 15.0 * GB,
        Disk:   1 * 60.0 * GB,
        Gpu:    1.0,
    },
    "g2.8xlarge": InstanceInfo{
        Cpu:    32.0,
        Memory: 60.0 * GB,
        Disk:   2 * 120.0 * GB,
        Gpu:    4.0,
    },
    ec2.InstanceTypeR3Large: InstanceInfo{
        Cpu:    4.0,
        Memory: 15.25 * GB,
        Disk:   1 * 32.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeR3Xlarge: InstanceInfo{
        Cpu:    4.0,
        Memory: 30.5 * GB,
        Disk:   1 * 80.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeR32xlarge: InstanceInfo{
        Cpu:    8.0,
        Memory: 61.0 * GB,
        Disk:   1 * 160.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeR34xlarge: InstanceInfo{
        Cpu:    16.0,
        Memory: 122.0 * GB,
        Disk:   1 * 320.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeR38xlarge: InstanceInfo{
        Cpu:    32.0,
        Memory: 244.0 * GB,
        Disk:   2 * 320.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeI2Xlarge: InstanceInfo{
        Cpu:    4.0,
        Memory: 30.5 * GB,
        Disk:   1 * 800.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeI22xlarge: InstanceInfo{
        Cpu:    8.0,
        Memory: 61.0 * GB,
        Disk:   2 * 800.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeI24xlarge: InstanceInfo{
        Cpu:    16.0,
        Memory: 122.0 * GB,
        Disk:   4 * 800.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeI28xlarge: InstanceInfo{
        Cpu:    32.0,
        Memory: 244.0 * GB,
        Disk:   8 * 800.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeD2Xlarge: InstanceInfo{
        Cpu:    4.0,
        Memory: 30.5 * GB,
        Disk:   3 * 2000.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeD22xlarge: InstanceInfo{
        Cpu:    8.0,
        Memory: 61.0 * GB,
        Disk:   6 * 2000.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeD24xlarge: InstanceInfo{
        Cpu:    16.0,
        Memory: 122.0 * GB,
        Disk:   12 * 2000.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeD28xlarge: InstanceInfo{
        Cpu:    36.0,
        Memory: 244.0 * GB,
        Disk:   24 * 2000.0 * GB,
        Gpu:    0.0,
    },

    /* OLD GENERATION INSTANCES */

    ec2.InstanceTypeM1Small: InstanceInfo{
        Cpu:    1.0,
        Memory: 1.7 * GB,
        Disk:   1.160 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeM1Medium: InstanceInfo{
        Cpu:    1.0,
        Memory: 3.75 * GB,
        Disk:   1 * 410.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeM1Large: InstanceInfo{
        Cpu:    2.0,
        Memory: 7.5 * GB,
        Disk:   2 * 420.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeM1Xlarge: InstanceInfo{
        Cpu:    4.0,
        Memory: 15.0 * GB,
        Disk:   4 * 420.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeC1Medium: InstanceInfo{
        Cpu:    2.0,
        Memory: 1.7 * GB,
        Disk:   1 * 350.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeC1Xlarge: InstanceInfo{
        Cpu:    8.0,
        Memory: 7.0 * GB,
        Disk:   4 * 420.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeCc28xlarge: InstanceInfo{
        Cpu:    32.0,
        Memory: 60.5 * GB,
        Disk:   4 * 480.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeCg14xlarge: InstanceInfo{
        Cpu:    16.0,
        Memory: 22.5 * GB,
        Disk:   2 * 840.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeM2Xlarge: InstanceInfo{
        Cpu:    2.0,
        Memory: 17.1 * GB,
        Disk:   1 * 420.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeM22xlarge: InstanceInfo{
        Cpu:    4.0,
        Memory: 34.2 * GB,
        Disk:   1 * 850.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeM24xlarge: InstanceInfo{
        Cpu:    8.0,
        Memory: 68.4 * GB,
        Disk:   2 * 840.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeCr18xlarge: InstanceInfo{
        Cpu:    32.0,
        Memory: 244.0 * GB,
        Disk:   2 * 120.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeHi14xlarge: InstanceInfo{
        Cpu:    16.0,
        Memory: 60.5 * GB,
        Disk:   2 * 1024.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeHs18xlarge: InstanceInfo{
        Cpu:    16.0,
        Memory: 117.0 * GB,
        Disk:   24 * 2000.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeT1Micro: InstanceInfo{
        Cpu:    1.0,
        Memory: 0.613 * GB,
        Disk:   0.0 * GB,
        Gpu:    0.0,
    },
    ec2.InstanceTypeCc14xlarge: InstanceInfo{
        Cpu:    0.0,
        Memory: 0.0 * GB,
        Disk:   0.0 * GB,
        Gpu:    0.0,
    },
}
