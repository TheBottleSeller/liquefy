package main

import (
	"fmt"
	"net/http"
)

func main() {
	stopChan := make(chan struct{}, 1)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Stopping test server")
		var s struct{}
		stopChan <- s
	})

	fmt.Println("Starting test server")
	go http.ListenAndServe(":80", nil)

	<-stopChan
	fmt.Println("Exiting")
}
