package main

import (
    "net/http"
    "flag"
    "fmt"

    log "github.com/Sirupsen/logrus"
)

const (
    ExecutorPort int = 4949
)

func main() {
    publicIp := flag.String("publicIp", "", "Public IP of host")
    privateIp := flag.String("privateIp", "", "Private IP of host")
    executorPath := flag.String("executor", "", "Path to executor binary")
    flag.Parse()

    if *publicIp == "" {
        log.Error("Please provide a public ip")
        return
    }
    if *privateIp == "" {
        log.Error("Please provide a private ip")
        return
    }
    if *executorPath == "" {
        log.Error("No executor path provided")
        return
    }

    // Setup http route
    route := "executor"
    http.HandleFunc("/" + route, func(w http.ResponseWriter, r *http.Request) {
        http.ServeFile(w, r, *executorPath)
    })

    log.Infof("Hosting executor '%s' at http://%s:%d/%s", *executorPath, *publicIp, ExecutorPort, route)
    log.Info("Starting http server")
    err := http.ListenAndServe(fmt.Sprintf("%s:%d", *privateIp, ExecutorPort), nil)
    if err != nil {
        log.Error("Error starting http server")
        log.Error(err)
    }
    log.Info("Executor http server terminating")
}