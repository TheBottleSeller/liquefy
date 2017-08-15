package main

import (
    "flag"
    "fmt"

    lq "bargain/liquefy/models"
    clouds "bargain/liquefy/cloudprovider"
    "strconv"
    "encoding/json"

)

// This IAM Roles only has AmazonEC2ReadOnlyAccess
// Probably should do this a different way though
// YOLO
var AwsAccessKey = ""
var AwsSecretKey = ""

func main() {
    region := flag.String("region", "", "the ip of the api server")
    flag.Parse()

    if *region == "" {
        fmt.Print("region is required")
        return
    }

    awsAccount := &lq.AwsAccount{
        AwsAccessKey: &AwsAccessKey,
        AwsSecretKey: &AwsSecretKey,
    }

    aws := clouds.NewAwsCloud(awsAccount.AwsAccessKey, awsAccount.AwsSecretKey)
    spotPriceInfo, err := aws.GetCurrentSpotPrices(clouds.Region(*region))
    if err != nil {
        fmt.Println(err)
        return
    }

    // map of az -> instance -> price
    pricesByAZ := make(map[string]map[string]float32)
    for _, spotPrice := range spotPriceInfo {
        instancePrices, ok := pricesByAZ[*spotPrice.AvailabilityZone]
        if !ok {
            instancePrices = make(map[string]float32)
            pricesByAZ[*spotPrice.AvailabilityZone] = instancePrices
        }

        _, ok = instancePrices[*spotPrice.InstanceType]
        if ok {
            fmt.Println("Got two prices for the same instance in the same AZ")
            continue
        }

        price, err :=  strconv.ParseFloat(*spotPrice.SpotPrice, 32)
        if err != nil {
            fmt.Println("AWS returned a bad formated price!!!!")
            continue
        }

        instancePrices[*spotPrice.InstanceType] = float32(price)
    }

    jsonString, err := json.MarshalIndent(pricesByAZ, "", "  ")
    if err != nil {
        fmt.Println("error:", err)
    }
    fmt.Println(string(jsonString))
}
