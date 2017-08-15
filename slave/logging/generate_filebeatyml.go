package main

import (
	"errors"
	"flag"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
)

//filebeat:
//# List of prospectors to fetch data.
//prospectors:
//# Each - is a prospector. Below are the prospector specific configurations
//-
//# Paths that should be crawled and fetched. Glob based paths.
//# For each file found under this path, a harvester is started.
//paths:
//- "/var/lib/docker/containers/*/*.log"
//# - c:\programdata\elasticsearch\logs\*
//
//# Type of the files. Based on this the way the file is read is decided.
//# The different types cannot be mixed in one prospector
//#
//# Possible options are:
//# * log: Reads every line of the log file (default)
//# * stdin: Reads the standard in
//type: log

type Generator struct {
	Filebeat
	Output
}

type Filebeat struct {
	Prospectors []Prospector
}

type Prospector struct {
	Paths           []string    `yaml:"paths"`
	Logtype         string      `yaml:"type"`
	ScanFrequency   string      `yaml:"scan_frequency"`
}

type Output struct {
	Elasticsearch
}

type Elasticsearch struct {
	Hosts    []string `yaml:"hosts,flow"`
	Protocol string   `yaml:"protocol"`
	Path     string   `yaml:"path"`
}

func main() {

	hostname := flag.String("masterip", "", "Elastic search IP")
	flag.Parse()

	if *hostname == "" {
		panic(errors.New("Invalid Hostname"))
	}

	hosts := []string{*hostname + ":9200"}

	fb := Generator{
		Filebeat: Filebeat{
			Prospectors: []Prospector{{
				Paths:   []string{
					"/var/lib/docker/containers/*/*.log",
					"/var/lib/mesos/slave/slaves/*/frameworks/*/executors/Liquefy/runs/latest/stdout",
					"/var/lib/mesos/slave/slaves/*/frameworks/*/executors/Liquefy/runs/latest/stderr",
				},
				Logtype: "log",
				ScanFrequency: "1s",
			}},
		},
		Output: Output{
			Elasticsearch: Elasticsearch{hosts, "http", "/"},
		},
	}

	d, err := yaml.Marshal(&fb)
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	fmt.Printf("--- output dump:\n%s\n\n", string(d))
	fmt.Println(string(d))

	//Generate file and place at location

	f, err := os.Create("/etc/filebite.yml")
	if err != nil {
		panic(err)
	}

	err = ioutil.WriteFile("/etc/filebite.yml", d, 0644)

	if err != nil {
		panic(err)
	}

	f.Sync()

	defer func() {
		if err := f.Close(); err != nil {
			panic(err)
		}
	}()
}
