package main

import (
    "flag"
    "fmt"

    "bargain/liquefy/api"
)

func main() {
    ip := flag.String("ip", "", "the ip of the api server")
    instanceId := flag.Int("instanceId", 0, "the id of the instance to kill")
    apiKey := flag.String("apiKey", "", "the apiKey to use")
    flag.Parse()

    if *ip == "" {
        panic("ip is required")
    }

    if *instanceId == 0 {
        panic("instanceId is required")
    }

    if *apiKey == "" {
        panic("apiKey is required")
    }

    apiClient := api.NewApiClient(fmt.Sprintf("http://%s:3030", *ip))
    err := apiClient.DeleteInstance(uint(*instanceId), *apiKey)
    if err != nil {
        fmt.Println(err)
    }
    fmt.Printf("Deprovisioning instance: %d\n", *instanceId)
}
