package main

import (
    "os"
    "fmt"
    log "github.com/Sirupsen/logrus"

    "bargain/liquefy/awsutil"
    lq "bargain/liquefy/models"
    awsCloud "bargain/liquefy/cloudprovider"
)

func main() {
    log.SetLevel(log.DebugLevel)

    home := os.Getenv("HOME")
    awsConfig := home + "/.aws/aws_config.ini"
    config, err := awsutil.LoadUserConfig(awsConfig)
    if err != nil {
        log.Errorf("Failed to load aws config file %s", awsConfig)
        panic(err)
    }

    awsAccount := &lq.AwsAccount{
        AwsAccessKey:           &config.AccessKeyID,
        AwsSecretKey:           &config.SecretAccessKey,
    }

    awsCloud := awsCloud.NewAwsCloud(awsAccount.AwsAccessKey, awsAccount.AwsSecretKey)
    spotReq, err := awsCloud.CreateSpotInstanceRequest(config.Region, config.AvailabilityZone, "ami-398bdc53",
        config.Subnet, config.SecurityGroup, "g2.2xlarge", 0.10, 3)
    if err != nil {
        panic(err)
    }
    fmt.Println(spotReq)

    spotReq, err = awsCloud.WaitForSpotRequestToFinish(spotReq)
    if err != nil {
        panic(err)
    }
    fmt.Println(spotReq)

    instance, err := awsCloud.GetInstance(config.Region, *spotReq.InstanceId)
    if err != nil {
        panic(err)
    }
    fmt.Println(instance)

    instance, err = awsCloud.WaitForIpAllocation(config.Region, instance)
    if err != nil {
        panic(err)
    }
    fmt.Println(instance)
}