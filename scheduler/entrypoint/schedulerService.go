package main

import (
    "flag"
	"errors"
	"runtime"
    "fmt"

    log "github.com/Sirupsen/logrus"

    . "bargain/liquefy/scheduler"
    "bargain/liquefy/common"
    "bargain/liquefy/db"
    "bargain/liquefy/logging"
)

func main() {
    log.SetLevel(log.DebugLevel)
    mesosMasterIp := flag.String("mesosMasterIp", "", "IP of the Mesos Master")
    schedIp := flag.String("schedIp", "", "IP where the scheduler should bind")
	executorIp := flag.String("executorIp", "", "IP of the Liquefy executor")
    dbIp := flag.String("dbIp", "", "IP of the DB")
    esIp := flag.String("esIp", "", "the ip of the es server for logs")

    flag.Parse()

    if *mesosMasterIp == "" {
        panic(errors.New("Provide a valid mesos master IP"))
    }
    if *schedIp == "" {
        panic(errors.New("Provide a valid sched IP"))
    }
    if *executorIp == "" {
        panic(errors.New("Provide a valid executor IP"))
    }
    if *dbIp == "" {
        panic(errors.New("Provide a valid DB IP"))
    }

    common.ValidateDeployment()
    if common.IsProductionDeployment() || common.IsStagingDeployment() {
        if *esIp == "" {
            panic("Production and staging deployments require the esIp flag")
        }
        log.AddHook(logging.NewElasticHook(*esIp, "9200", "liquefy-", "Provisioner", ""))
    }

    runtime.GOMAXPROCS(256)
    err := db.Connect(*dbIp)
    if err != nil {
        panic(err)
    }

    //SLAVE EXEC
    //TODO:: Inject ESPublic ip
    command :=  fmt.Sprintf("./executor --esIp=%s", *mesosMasterIp)
    lqScheduler := NewLqScheduler(*schedIp, *mesosMasterIp, *executorIp, command)

    status, err := lqScheduler.Run()
    if err != nil {
        log.Error("Framework failed to run with status %s", status.String())
        panic(err)
    }

    log.Infof("Framework terminating with status %s", status.String())
}
