package main

import (
	lqExecutor "bargain/liquefy/executor"
	log "github.com/Sirupsen/logrus"
	mesosExecutor "github.com/mesos/mesos-go/executor"
	"bargain/liquefy/logging"
	"runtime"
	"time"
	"flag"

	"os"
)

func main() {
	log.Info("Starting Liquefy Executor")
	log.SetLevel(log.DebugLevel)

	esIp := flag.String("esIp", "", "the ip of the es server for logs")
	flag.Parse()

	if *esIp != "" {
		identifier := "slave-" + os.Getenv("RESOURCE_ID")
		log.AddHook(logging.NewElasticHook(*esIp, "9200", "liquefyslave-",identifier,""))
	}

	runtime.GOMAXPROCS(256)

	config := mesosExecutor.DriverConfig{
		Executor: lqExecutor.NewLiquidExecutor("unix:///var/run/docker.sock"),
	}

	driver, err := mesosExecutor.NewMesosExecutorDriver(config)
	if err != nil {
		log.Error("Unable to create a ExecutorDriver ", err.Error())

		//  to ensure all the logs gets picked up, sleep 5 seconds
		time.Sleep(time.Duration(5) * time.Second)
		return
	}

	_, err = driver.Start()
	if err != nil {
		log.Error("Unable to start driver: ", err)

		//  to ensure all the logs gets picked up, sleep 5 seconds
		time.Sleep(time.Duration(5) * time.Second)
		return
	}

	log.Error("Executor process has started and running.")
	_, err = driver.Join()
	if err != nil {
		log.Error("Driver failed: ", err)

		//  to ensure all the logs gets picked up, sleep 5 seconds
		time.Sleep(time.Duration(5) * time.Second)
		return
	}
	log.Error("Terminating Liquefy Executor")
}