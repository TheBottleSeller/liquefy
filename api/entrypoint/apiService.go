package main

import (
    "flag"

    log "github.com/Sirupsen/logrus"

    "bargain/liquefy/common"
    "bargain/liquefy/db"
    . "bargain/liquefy/api"
    "bargain/liquefy/logging"
)

func main() {
    log.SetLevel(log.DebugLevel)
    dbIp := flag.String("dbIp", "localhost", "the ip of the db server")
    esIp := flag.String("esIp", "localhost", "the ip of the elastic search server")

    flag.Parse()

    common.ValidateDeployment()
    if common.IsProductionDeployment() || common.IsStagingDeployment() {
        if *esIp == "" {
            panic("Production and staging deployments require the esIp flag")
        }
        log.AddHook(logging.NewElasticHook(*esIp, "9200", "liquefy-", "Provisioner", ""))
    }
    log.AddHook(logging.NewElasticHook(*esIp,"9200","liquefy-","APIServer",""))

    err := db.Connect(*dbIp)
    if err != nil {
        panic(err)
    }

    apiServer := NewApiServer()
    apiServer.Start()
}