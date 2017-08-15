package main

import (
	"bargain/liquefy/executor"
	log "github.com/Sirupsen/logrus"
	exec "github.com/mesos/mesos-go/executor"
	"runtime"
)

func main() {
	log.Info("Starting Liquefy Executor")

	runtime.GOMAXPROCS(256)

	dconfig := exec.DriverConfig{
		Executor: executor.NewLiquidExecutor("tcp://192.168.99.100:2376"),
	}

	driver, err := exec.NewMesosExecutorDriver(dconfig)
	if err != nil {
		log.Error("Unable to create a ExecutorDriver ", err.Error())
	}

	_, err = driver.Start()
	if err != nil {
		log.Error("Got error:", err)
		return
	}

	log.Error("Executor process has started and running.")
	_, err = driver.Join()
	if err != nil {
		log.Error("driver failed:", err)
	}
	log.Error("executor terminating")
}
