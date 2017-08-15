package main

import (
    "flag"

    log "github.com/Sirupsen/logrus"

    "bargain/liquefy/common"
    "bargain/liquefy/db"
    . "bargain/liquefy/provisioner"
    "bargain/liquefy/logging"
)

func main() {
    log.SetLevel(log.DebugLevel)

    mesosMasterIp := flag.String("mesosMasterIp", "", "the ip of the mesos master server")
    dbIp := flag.String("dbIp", "", "the ip of the db server")
    esIp := flag.String("esIp", "", "the ip of the es server for logs")

    flag.Parse()

    if *mesosMasterIp == "" {
        panic("Required parameter mesosMasterIp is missing")
    }

    if *dbIp == "" {
        panic("Required parameter dbIp is missing")
    }

    common.ValidateDeployment()
    if common.IsProductionDeployment() || common.IsStagingDeployment() {
        if *esIp == "" {
            panic("Production and staging deployments require the esIp flag")
        }
        log.AddHook(logging.NewElasticHook(*esIp, "9200", "liquefy-", "Provisioner", ""))
    }

    err := db.Connect(*dbIp)
    if err != nil {
        panic(err)
    }

    log.Info("Connected to Database , Starting Provisioner")
    provisioner := NewProvisioner(*mesosMasterIp)
    err = provisioner.Run()
    if err != nil {
        log.Error("Provisioner failed")
        log.Error(err)
    }
}