package influxdb

import(
    ."github.com/smartystreets/goconvey/convey"
    "testing"

    awsCloud "bargain/liquefy/cloudprovider"
    "time"
    "fmt"
    "strings"
)

func TestGetCurrentSpotPrices(t *testing.T) {
    Convey("Test get spot prices", t, func() {
        db, err := NewInfluxDBClient()
        if err != nil {
            panic(err)
        }

        i := 0
        instances := make([]string, len(awsCloud.AvailableInstances))
        for instance := range awsCloud.AvailableInstances {
            instances[i] = instance
            i++
        }

        fmt.Println(db.GetCurrentSpotPrices(instances))
    })
}

func TestGetSpotPriceHistory(t *testing.T) {
    Convey("Test get spot price history", t, func() {
        db, err := NewInfluxDBClient()
        if err != nil {
            panic(err)
        }
        now := time.Now()
        oneHourBefore := now.Add(time.Duration(-1) * time.Hour)
        now.Unix()

        instances := []string{}
        for instance := range awsCloud.AvailableInstances {
            if strings.HasPrefix(instance, "t2") {
                continue
            }
            instances = append(instances, instance)
        }

        for _, instance := range instances {
            history, err := db.GetSpotPriceHistory("us-east-1a", instance, oneHourBefore, now)
            if err != nil {
                fmt.Printf("Failed with instance %s\n", instance)
            } else {
                fmt.Printf("Got %d prices for instances %s\n", len(history), instance)
            }
        }
    })
}
