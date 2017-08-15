package cloudprovider

import (
	lqredis "bargain/liquefy/cache"

	"fmt"
	log "github.com/Sirupsen/logrus"
	"strings"
	"time"
)

var UnavailableDuration = time.Duration(15*60) * time.Second // 15 minutes
var PollTime = time.Duration(1) * time.Second

// An unssupported market is one which Amazon does not provide
var UnsupportedMarkets map[AZ]map[InstanceType]struct{}

// An unavailable market is one which Amazon provides, but is temporarily unavailable
func init() {
	log.SetLevel(log.DebugLevel)

	// Setup unsupported markets
	UnsupportedMarkets = make(map[AZ]map[InstanceType]struct{})
	for region, azs := range AWSRegionsToAZs {
		for _, az := range azs {
			UnsupportedMarkets[az] = make(map[InstanceType]struct{})
			for instance := range AvailableInstances {
				if _, supported := SupportedMarkets[region][instance]; !supported {
					UnsupportedMarkets[az][instance] = struct{}{}
				}
			}
		}
	}
}

// GetUnavailableMarkets returns a map of known unavailable markets by az
func GetUnavailableMarkets() map[AZ]map[InstanceType]struct{} {
	// Setup map
	markets := make(map[AZ]map[InstanceType]struct{})
	for _, azs := range AWSRegionsToAZs {
		for _, az := range azs {
			markets[az] = make(map[InstanceType]struct{})
		}
	}

	// Mark UnsupportedMarkets
	for az, instances := range UnsupportedMarkets {
		for instance := range instances {
			markets[az][instance] = struct{}{}
		}
	}

	// Mark UnavailableMarkets
	conn := lqredis.GetConnection()
	defer conn.Close()

	unavailableAZKeys := lqredis.GetMatchingKeys(conn, "unavailable|*")
	log.Debugf("Found %v unavailable keys", len(unavailableAZKeys))

	for _, key := range unavailableAZKeys {
		az, instance := parseUnavailableKey(key)
		markets[az][instance] = struct{}{}
	}
	return markets
}

// MarkMarketUnavailable stores an <az>:<instance_type> market as unavailable
func MarkMarketUnavailable(az AZ, instanceType InstanceType) {
	log.Debugf("Market Monitor: Marking as unavailable (%s, %s)", az.String(), instanceType.String())
	conn := lqredis.GetConnection()
	defer conn.Close()

	// Set the key with a TTL of UnavailableDuration (s)
	_, err := conn.Do(
		"SET",
		keyFromMarket("unavailable", az, instanceType),
		time.Now(),
	)
	if err != nil {
		log.Errorf("Failed to set market unavailble %s %s %s", az, instanceType, err)
	}
	_, err = conn.Do(
		"EXPIRE",
		keyFromMarket("unavailable", az, instanceType),
		int(UnavailableDuration),
	)
	if err != nil {
		log.Errorf("Failed to set ttl on market unavailable %s %s %s", az, instanceType, err)
	}
}

// TODO: ADD SUPPORT FOR NON HVM INSTANCE TYPES
// Map of supported markets that is used to construct the unsupported markets map above
var SupportedMarkets = map[Region]map[InstanceType]struct{}{
	"us-east-1": map[InstanceType]struct{}{
		//        "c1.medium": struct{}{},
		//        "c1.xlarge": struct{}{},
		//        "m1.large": struct{}{},
		//        "m1.medium": struct{}{},
		//        "m1.small": struct{}{},
		//        "m1.xlarge": struct{}{},
		//        "m2.2xlarge": struct{}{},
		//        "m2.4xlarge": struct{}{},
		//        "m2.xlarge": struct{}{},
		//        "t1.micro": struct{}{},
		"c3.2xlarge":  struct{}{},
		"c3.4xlarge":  struct{}{},
		"c3.8xlarge":  struct{}{},
		"c3.large":    struct{}{},
		"c3.xlarge":   struct{}{},
		"c4.2xlarge":  struct{}{},
		"c4.4xlarge":  struct{}{},
		"c4.8xlarge":  struct{}{},
		"c4.large":    struct{}{},
		"c4.xlarge":   struct{}{},
		"cc2.8xlarge": struct{}{},
		"cg1.4xlarge": struct{}{},
		"cr1.8xlarge": struct{}{},
		"d2.2xlarge":  struct{}{},
		"d2.4xlarge":  struct{}{},
		"d2.8xlarge":  struct{}{},
		"d2.xlarge":   struct{}{},
		"g2.2xlarge":  struct{}{},
		"g2.8xlarge":  struct{}{},
		"hi1.4xlarge": struct{}{},
		"i2.2xlarge":  struct{}{},
		"i2.4xlarge":  struct{}{},
		"i2.8xlarge":  struct{}{},
		"i2.xlarge":   struct{}{},
		"m3.2xlarge":  struct{}{},
		"m3.large":    struct{}{},
		"m3.medium":   struct{}{},
		"m3.xlarge":   struct{}{},
		"m4.10xlarge": struct{}{},
		"m4.2xlarge":  struct{}{},
		"m4.4xlarge":  struct{}{},
		"m4.large":    struct{}{},
		"m4.xlarge":   struct{}{},
		"r3.2xlarge":  struct{}{},
		"r3.4xlarge":  struct{}{},
		"r3.8xlarge":  struct{}{},
		"r3.large":    struct{}{},
		"r3.xlarge":   struct{}{},
	},
	"us-west-1": map[InstanceType]struct{}{
		//        "c1.medium": struct{}{},
		//        "c1.xlarge": struct{}{},
		//        "m1.large": struct{}{},
		//        "m1.medium": struct{}{},
		//        "m1.small": struct{}{},
		//        "m1.xlarge": struct{}{},
		//        "m2.2xlarge": struct{}{},
		//        "m2.4xlarge": struct{}{},
		//        "m2.xlarge": struct{}{},
		//        "t1.micro": struct{}{},
		"c3.2xlarge":  struct{}{},
		"c3.4xlarge":  struct{}{},
		"c3.8xlarge":  struct{}{},
		"c3.large":    struct{}{},
		"c3.xlarge":   struct{}{},
		"c4.2xlarge":  struct{}{},
		"c4.4xlarge":  struct{}{},
		"c4.8xlarge":  struct{}{},
		"c4.large":    struct{}{},
		"c4.xlarge":   struct{}{},
		"cc2.8xlarge": struct{}{},
		"cg1.4xlarge": struct{}{},
		"cr1.8xlarge": struct{}{},
		"d2.2xlarge":  struct{}{},
		"d2.4xlarge":  struct{}{},
		"d2.8xlarge":  struct{}{},
		"d2.xlarge":   struct{}{},
		"g2.2xlarge":  struct{}{},
		"g2.8xlarge":  struct{}{},
		"hi1.4xlarge": struct{}{},
		"i2.2xlarge":  struct{}{},
		"i2.4xlarge":  struct{}{},
		"i2.8xlarge":  struct{}{},
		"i2.xlarge":   struct{}{},
		"m3.2xlarge":  struct{}{},
		"m3.large":    struct{}{},
		"m3.medium":   struct{}{},
		"m3.xlarge":   struct{}{},
		"m4.10xlarge": struct{}{},
		"m4.2xlarge":  struct{}{},
		"m4.4xlarge":  struct{}{},
		"m4.large":    struct{}{},
		"m4.xlarge":   struct{}{},
		"r3.2xlarge":  struct{}{},
		"r3.4xlarge":  struct{}{},
		"r3.8xlarge":  struct{}{},
		"r3.large":    struct{}{},
		"r3.xlarge":   struct{}{},
	},
	"us-west-2": map[InstanceType]struct{}{
		//        "c1.medium": struct{}{},
		//        "c1.xlarge": struct{}{},
		//        "m1.large": struct{}{},
		//        "m1.medium": struct{}{},
		//        "m1.small": struct{}{},
		//        "m1.xlarge": struct{}{},
		//        "m2.2xlarge": struct{}{},
		//        "m2.4xlarge": struct{}{},
		//        "m2.xlarge": struct{}{},
		//        "t1.micro": struct{}{},
		"c3.2xlarge":  struct{}{},
		"c3.4xlarge":  struct{}{},
		"c3.8xlarge":  struct{}{},
		"c3.large":    struct{}{},
		"c3.xlarge":   struct{}{},
		"c4.2xlarge":  struct{}{},
		"c4.4xlarge":  struct{}{},
		"c4.8xlarge":  struct{}{},
		"c4.large":    struct{}{},
		"c4.xlarge":   struct{}{},
		"cc2.8xlarge": struct{}{},
		"cg1.4xlarge": struct{}{},
		"cr1.8xlarge": struct{}{},
		"d2.2xlarge":  struct{}{},
		"d2.4xlarge":  struct{}{},
		"d2.8xlarge":  struct{}{},
		"d2.xlarge":   struct{}{},
		"g2.2xlarge":  struct{}{},
		"g2.8xlarge":  struct{}{},
		"hi1.4xlarge": struct{}{},
		"i2.2xlarge":  struct{}{},
		"i2.4xlarge":  struct{}{},
		"i2.8xlarge":  struct{}{},
		"i2.xlarge":   struct{}{},
		"m3.2xlarge":  struct{}{},
		"m3.large":    struct{}{},
		"m3.medium":   struct{}{},
		"m3.xlarge":   struct{}{},
		"m4.10xlarge": struct{}{},
		"m4.2xlarge":  struct{}{},
		"m4.4xlarge":  struct{}{},
		"m4.large":    struct{}{},
		"m4.xlarge":   struct{}{},
		"r3.2xlarge":  struct{}{},
		"r3.4xlarge":  struct{}{},
		"r3.8xlarge":  struct{}{},
		"r3.large":    struct{}{},
		"r3.xlarge":   struct{}{},
	},
}

// keyFromMarket returns a redis-friendly key from az, instance_type
// the key looks like "unavailable|us-east-1a|d3.xlarge"
func keyFromMarket(prefix string, az AZ, instanceType InstanceType) string {
	return fmt.Sprintf("%s|%s|%s", prefix, az, instanceType)
}

// parseUnavailableKey returns az, instance_type from redis "unavailable|" key
func parseUnavailableKey(key string) (AZ, InstanceType) {
	split := strings.Split(key[12:len(key)], "|")
	az, instance := split[0], split[1]
	return AZ(az), InstanceType(instance)
}
