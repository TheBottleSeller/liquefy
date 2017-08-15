package main

import (
	"flag"
	"fmt"
	"math/rand"
	"math"
	"time"

	lq "bargain/liquefy/models"
	"bargain/liquefy/api"
	"sync"
)

func main() {
	ip := flag.String("ip", "", "the ip of the api server")
	apiKey := flag.String("apiKey", "", "the apiKey to use")
	numJobs := flag.Int("numJobs", 1, "the number of jobs to use")
	wait := flag.Bool("wait", false, "wait for the jobs to complete")
	flag.Parse()

	if *ip == "" {
		panic("ip is required")
	}

	if *apiKey == "" {
		panic("apiKey is required")
	}

	var wg sync.WaitGroup
	apiClient := api.NewApiClient(fmt.Sprintf("http://%s:3030", *ip))
	for i := 0; i < *numJobs; i++ {
		time.Sleep(time.Second)
		rand.Seed(time.Now().Unix())

		job := &api.ContainerJobPublic{
			Name:        fmt.Sprintf("TestServer:%d", 80),
			Command:     "",
			SourceImage: "nbatlivala/test",
			PortMappings: []lq.PortMapping{
				{
					HostPort: 80 + i,
					ContainerPort: 80,
				},
			},
			Environment: []lq.EnvVar{
				{
					Variable: "LQ_NUMBER",
					Value: "1234",
				}, {
					Variable: "LQ_STRING",
					Value: "abc",
				}, {
					Variable: "LQ_WEIRD_STRING",
					Value: "\"'$-/\\_",
				},
			},
			Ram: int(math.Floor(rand.Float64() * float64(16 * 1024))),
			Cpu: rand.Float64() * 8,
			Gpu: 0,
		}

		jobId := apiClient.CreateJob(job, *apiKey)
		fmt.Printf("Created job %d (Cpu: %f, Ram: %d, Gpu: %d)\n", jobId, job.Cpu, job.Ram, job.Gpu)

		if *wait {
			wg.Add(1)
			go func() {
				prevStatus := ""
				prevInstance := uint(0)
				for {
					job := apiClient.GetJob(jobId, *apiKey)
					if job.Status == "TASK_FINISHED" || job.Status == "TASK_KILLED" || job.Status == "TASK_FAILED" {
						wg.Done()
						fmt.Printf("Job %d terminated with status %s\n", job.ID, job.Status)
						return
					}
					if job.Status != prevStatus || job.InstanceID != prevInstance {
						fmt.Printf("Job %d. Status %s. Instance %d\n", job.ID, job.Status, job.InstanceID)
						prevStatus = job.Status
						prevInstance = job.InstanceID
					}
					time.Sleep(time.Duration(5) * time.Second)
				}
			}()
		}
	}

	if *wait {
		wg.Wait()
	}
}
