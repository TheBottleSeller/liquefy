package main

import (
    "flag"
    "fmt"

    "bargain/liquefy/api"
)

func main() {
    ip := flag.String("ip", "", "the ip of the api server")
    jobId := flag.Int("jobId", 0, "the id of the job to kill")
    apiKey := flag.String("apiKey", "", "the apiKey to use")
    flag.Parse()

    if *ip == "" {
        panic("ip is required")
    }

    if *jobId == 0 {
        panic("jobId is required")
    }

    if *apiKey == "" {
        panic("apiKey is required")
    }

    apiClient := api.NewApiClient(fmt.Sprintf("http://%s:3030", *ip))
    err := apiClient.DeleteJob(uint(*jobId), *apiKey)
    if err != nil {
        fmt.Println(err)
    }
    fmt.Printf("Killed job: %d\n", *jobId)
}
