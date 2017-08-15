package main

import (
	"os"
	"flag"
	"fmt"

	"bargain/liquefy/awsutil"
	"bargain/liquefy/api"
)

var defaultUser = &api.ApiUser{
	Username:  "darth@vader.com",
	Password:  "test123",
	Firstname: "Darth",
	Lastname:  "Vader",
	Email:     "darth@vader.com",
}

func main() {
	ip := flag.String("ip", "", "the ip of the api server")
	link := flag.Bool("link", true, "whether or not to link an aws account or not")
	flag.Parse()

	if *ip == "" {
		fmt.Print("ip is required")
		return
	}

	apiClient := api.NewApiClient(fmt.Sprintf("http://%s:3030", *ip))
	apiKey, err := apiClient.CreateUser(defaultUser)
	if err != nil {
		fmt.Printf("Failed creating user\n%s\n", err.Error())
		return
	}
	fmt.Printf("Created user with api key\n%s\n", apiKey)

	if *link {
		home := os.Getenv("HOME")
		awsConfig := home + "/.aws/aws_config.ini"
		config, err := awsutil.LoadUserConfig(awsConfig)
		if err != nil {
			panic(err)
		}
		apiClient.RegisterAwsAccount(config.AccessKeyID, config.SecretAccessKey, apiKey)
	}
}
