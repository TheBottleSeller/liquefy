package main

import (
    "os"
    log "github.com/Sirupsen/logrus"
    "bargain/liquefy/awsutil"
    cloud "bargain/liquefy/cloudprovider"
)

func fetchCreds() (*string, *string) {
    home := os.Getenv("HOME")

    awsCreds, err := awsutil.LoadUserConfig(home + "/.aws/aws_config.ini")

    if (err != nil){
        panic(err)
    }

    return &awsCreds.AccessKeyID, &awsCreds.SecretAccessKey
}

func main() {
    log.SetLevel(log.DebugLevel)
    cloud.NewAwsCloud(fetchCreds()).CleanUpAwsAccount()
}
