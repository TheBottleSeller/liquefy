package influxdb

import (
    influxdb "github.com/influxdb/influxdb/client/v2"
    log "github.com/Sirupsen/logrus"
    "fmt"
    "time"
    "encoding/json"

    lq "bargain/liquefy/models"
    aws "bargain/liquefy/cloudprovider"
)

const (
    AwsSpotPricesDBName = "aws_spot_prices"
    InfluxDBHost = "52.9.16.175"
)

type spotPriceDb struct {
    Client influxdb.Client
}

type SpotPriceDB interface {
    GetCurrentSpotPrices(markets map[aws.AZ]map[aws.InstanceType]struct{}) (map[aws.AZ]map[aws.InstanceType]float64, error)
    GetSpotPriceHistory(az, instance string, startTime, endTime time.Time) ([]*SpotPrice, error)
}

func NewInfluxDBClient() (SpotPriceDB, error) {
    client, err := influxdb.NewHTTPClient(influxdb.HTTPConfig{
        Addr: fmt.Sprintf("http://%s:8086", InfluxDBHost),
        Timeout: time.Duration(5) * time.Second,
    })
    return &spotPriceDb{ client }, err
}

// Returns a map of az -> map of instance -> spotPrice
func (db *spotPriceDb) GetCurrentSpotPrices(markets map[aws.AZ]map[aws.InstanceType]struct{}) (map[aws.AZ]map[aws.InstanceType]float64, error) {
    spotPricesByAz := make(map[aws.AZ]map[aws.InstanceType]float64)

    for az, instances := range markets {
        spotPricesByAz[az] = make(map[aws.InstanceType]float64)

        for instance := range instances {
            key := fmt.Sprintf("exec_%s_%s_%s", lq.AZtoRegion(string(az)), string(az), string(instance))
            results, err := queryDB(db.Client,
                fmt.Sprintf("SELECT \"value\" FROM \"%s\"..\"%s\" LIMIT 1", AwsSpotPricesDBName, key))
            if err != nil {
                log.Errorf("Failed querying db for az %s and instance %s", az, instance)
                log.Error(err)
                continue
            }

            if len(results) == 1 && len(results[0].Series) == 1 && len(results[0].Series[0].Values) == 1 {
                row := results[0].Series[0].Values[0]
                value := row[1]
                if priceJson, ok := value.(json.Number); ok {
                    price, err := priceJson.Float64()
                    if err != nil {
                        log.Error(err)
                        continue
                    }
                    spotPricesByAz[az][instance] = price
                }
            }
        }
    }

    return spotPricesByAz, nil
}

type SpotPrice struct {
    Price   float64
    Time    time.Time
}

func (price *SpotPrice) String() string {
    return fmt.Sprintf("Price: %f, Time: %s", price.Price, price.Time)
}

func (db *spotPriceDb) GetSpotPriceHistory(az, instance string, startTime, endTime time.Time) ([]*SpotPrice, error) {
    // Remove last letter of az to get region
    region := lq.AZtoRegion(az)
    key := fmt.Sprintf("exec_%s_%s_%s", region, az, instance)

    // TODO Handle getting the timestamp just before the start time and just after the end time
    query := fmt.Sprintf("SELECT \"value\" FROM \"%s\"..\"%s\" WHERE time > %d AND time < %d",
        AwsSpotPricesDBName, key, startTime.UTC().UnixNano(), endTime.UTC().UnixNano())
    results, err := queryDB(db.Client, query)
    if err != nil {
        return []*SpotPrice{}, err
    }

    if len(results) != 1 || len(results[0].Series) != 1 {
        return []*SpotPrice{}, fmt.Errorf("Did not get 1 result and 1 series")
    }

    history := results[0].Series[0].Values

    // Originally create a list the total size of the pricing history
    prices := make([]*SpotPrice, len(history))
    i := 0
    for _, row :=  range history {
        price, err := getPriceFromRow(row)
        if err != nil {
            log.Error(err)
            return prices, err
        }
        timestamp, err := getTimeFromRow(row)
        if err != nil {
            log.Error(err)
            return prices, err
        }

        // If the price is the same as the subsequent timestamp, do not include it
        if i > 0 && price == prices[i - 1].Price {
            continue
        }

        prices[i] = &SpotPrice{
            Price: price,
            Time: timestamp,
        }
        i++
    }

    // Because there may be duplicate prices (consecutive timestamps where prices do not change)
    // We can return a shortened list of just the changed prices and timestamps
    shortenedList := make([]*SpotPrice, i)
    for j := 0; j < i; j++ {
        shortenedList[j] = prices[j]
    }

    return shortenedList, nil
}

func getPriceFromRow(row []interface{}) (float64, error) {
    if priceJson, ok := row[1].(json.Number); !ok {
        return 0.0, fmt.Errorf("Row price was not of type json.Number")
    } else {
        return priceJson.Float64()
    }
}

func getTimeFromRow(row []interface{}) (time.Time, error) {
    if timeString, ok := row[0].(string); !ok {
        return time.Now(), fmt.Errorf("Timestamp was not of type string")
    } else {
        return time.Parse(time.RFC3339, timeString)
    }
}

// queryDB convenience function to query the database
func queryDB(client influxdb.Client, cmd string) (res []influxdb.Result, err error) {
    q := influxdb.Query{
        Command:  cmd,
        Database: AwsSpotPricesDBName,
    }
    if response, err := client.Query(q); err == nil {
        if response.Error() != nil {
            return res, response.Error()
        }
        res = response.Results
    }
    return res, nil
}