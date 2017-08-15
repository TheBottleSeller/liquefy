package main

import (
    "os"
    "fmt"

    "bargain/liquefy/awsutil"
    "bargain/liquefy/cloudprovider"
    "bargain/liquefy/test"
)

func main() {
    home := os.Getenv("HOME")
    awsConfig := home + "/.aws/aws_config.ini"
    config, err := awsutil.LoadUserConfig(awsConfigPath)
    if err != nil {
        log.Errorf("Failed to load aws config file %s", awsConfigPath)
        panic(err)
    }

    awsCloud := cloudprovider.NewAwsCloud(&config.AccessKeyID, &config.SecretAccessKey)

    for region, azs := range cloudprovider.AWSRegionsToAZs {
        for _, az := range azs {
            image := getImage(region.String())
            awsCloud.CreateSpotInstanceRequest(region, az.String(), , subnetId string, securityGroupName string,
            instanceType string, spotPrice float64, resourceId uint)
        }
    }
    awsCloud.Create
    apiServer := test.NewApiServer(fmt.Sprintf("http://%s:3030", *ip))
    apiKey := apiServer.CreateUser(defaultUser, awsConfig)
    fmt.Print("Created user with api key:")
    fmt.Print(apiKey)

    if *withJob {
        jobId := apiServer.CreateJob(defaultJob, apiKey)
        fmt.Printf("Created job: %d", jobId)
    }
}

func getImage(region string) string {
    if region == "us-east-1" {
        return "ami-6889d200"
    }
    if region == "us-west-1" {
        return "ami-c37d9987"
    }

    if region == "us-west-2" {
        return "ami-35143705"
    }
}