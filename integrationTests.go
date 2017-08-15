package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
	log "github.com/Sirupsen/logrus"
	mesos "github.com/mesos/mesos-go/mesosproto"

	lq "bargain/liquefy/models"
	"bargain/liquefy/provisioner"
	"bargain/liquefy/test"
)

var serverUrl = "http://localhost:3030"

func main() {
	log.SetLevel(log.DebugLevel)
	master := flag.String("master", "vbox-master", "the master docker machine to use")
	setup := flag.Bool("setup", false, "setup the master")
	flag.Parse()

	api = test.NewApiServer(serverUrl)

	if *setup {
		setupMaster(*master)
	}

	masterIp := safeExecute(true, "docker-machine", "ip", *master)
	mesosMaster := waitForSchedulerToAttach(masterIp)
	log.Infof("Mesos Master: %v", mesosMaster)

	// TODO: Verify liquify framework id is persisted in postgres

	/* Create Test User */
	username := "TheDarkLord4"
	password := "LukeIAmYourFather"
	user := &test.ApiUser{
		Username:  username,
		Password:  password,
		Firstname: "Darth",
		Lastname:  "Vader",
		Email:     "darthvader@thedeathstar.com",
	}
	apiKey := api.CreateUser(user, "")
	defer func() {
		log.Info("Deleting user")
		api.DeleteUser(apiKey)
	}()

	/* Verify user fetched securely */
	fetchedUser := api.GetUser(apiKey)
	if fetchedUser.Password != "" {
		panic("Password should not be fetchable")
	}
	if fetchedUser.ApiKey != "" {
		panic("Api key should not be fetchable")
	}
	if user.Username != fetchedUser.Username ||
		user.Firstname != fetchedUser.Firstname ||
		user.Lastname != fetchedUser.Lastname ||
		user.Email != fetchedUser.Email {
		panic("Fetched user does not match created user")
	}

	/* Run Two Jobs At Once */

	testJobPublic1 := makeTestJob(80, provisioner.VBOX_RAM/2, provisioner.VBOX_CPU/2)
	testJobPublic2 := makeTestJob(81, provisioner.VBOX_RAM/2, provisioner.VBOX_CPU/2)

	log.Info("Starting jobs")
	testJob1 := startJob(testJobPublic1, apiKey)
	testJob1 = waitForJobState(testJob1, mesos.TaskState_TASK_RUNNING, apiKey)
	instance1 := api.GetInstance(testJob1.InstanceID, apiKey)
	time.Sleep(time.Second * time.Duration(5))

	testJob2 := startJob(testJobPublic2, apiKey)
	testJob2 = waitForJobState(testJob2, mesos.TaskState_TASK_RUNNING, apiKey)
	instance2 := api.GetInstance(testJob2.InstanceID, apiKey)

	time.Sleep(time.Second * time.Duration(5)) // Sleep 5 seconds to let server in job start up
	stopJob(testJobPublic1, testJob1.InstanceID, apiKey)
	testJob1 = waitForJobState(testJob1, mesos.TaskState_TASK_FINISHED, apiKey)
	stopJob(testJobPublic2, testJob2.InstanceID, apiKey)
	testJob2 = waitForJobState(testJob2, mesos.TaskState_TASK_FINISHED, apiKey)

	if instance1.ID != instance2.ID {
		panic("Jobs were run on different instances, when they could have been run on the same instance")
	}

	waitForInstanceStatus(instance1, lq.ResourceStatusDeprovisioned, apiKey)

//	if instance.CpuUsed != testJob.Cpu || instance.RamUsed != testJob.Ram {
//		panic("Instance ram and/or cpu used is not as expected")
//	}

	/* Run Second Job */

//	    testJob = startJob(testJobPublic, apiKey)
//	    testJob = waitForJobState(testJob, mesos.TaskState_TASK_RUNNING, apiKey)
//	    stopJob(testJobPublic, testJob.InstanceID, apiKey)
//	    testJob = waitForJobState(testJob, mesos.TaskState_TASK_FINISHED, apiKey)
//
//	    // The second job should be dispatched to the same resource as the first
//	    if instance.ID != testJob.InstanceID {
//	        panic("Second job not dispatched to same resource")
//	    }

	// Test two jobs will be run on the same machine
	//    log.Info("Starting 2 jobs on the same instance")
	//    var wg sync.WaitGroup
	//    apiJobs := splitNJobsAcrossResource(2, instance.ID, apiKey)
	//    log.Info(apiJobs)
	//    jobs := make([]*lq.ContainerJob, 2)
	//    for i := 0; i < 2; i++ {
	//        log.Infof("Running job %v", apiJobs[i])
	//        jobs[i] = startJob(apiJobs[i], apiKey)
	//        log.Infof("Created job %v", jobs[i])
	//        jobs[i] = waitForJobState(jobs[i], mesos.TaskState_TASK_RUNNING, apiKey)
	//    }
	//
	//    // Verify all are on the same instance
	//    for i := 0; i < 1; i++ {
	//        if jobs[i].InstanceID != jobs[i+1].InstanceID {
	//            panic("Expected jobs to be running on the same instance")
	//        }
	//    }
	//
	//    // Stop all the jobs
	//    for i := 0; i < 2; i++ {
	//        wg.Add(1)
	//        go func(index int) {
	//            stopJob(apiJobs[index], jobs[index].InstanceID, apiKey)
	//            jobs[index] = waitForJobState(jobs[index], mesos.TaskState_TASK_FINISHED, apiKey)
	//            wg.Done()
	//        }(i)
	//    }
	//    wg.Wait()
	//    log.Info("Successfully ran 2 jobs on the same instance")
}

func splitNJobsAcrossResource(n int, instanceId uint, apiKey string) []*test.ContainerJobPublic {
	var jobs []*test.ContainerJobPublic
	instance := api.GetInstance(instanceId, apiKey)
	log.Infof("CPU: total=%f, used=%f", instance.CpuTotal, instance.CpuUsed)
	log.Infof("Ram: total=%d, used=%d", instance.RamTotal, instance.RamUsed)
	cpuRemaining := instance.CpuTotal - instance.CpuUsed
	ramRemaining := instance.RamTotal - instance.RamUsed

	for i := 0; i < n; i++ {
		job := makeTestJob(80+i, ramRemaining/n, cpuRemaining/float32(n))
		jobs = append(jobs, job)
	}
	return jobs
}

func makeTestJob(hostPort int, ram int, cpu float32) *test.ContainerJobPublic {
	return &test.ContainerJobPublic{
		Name:        fmt.Sprintf("TestServer:%d", hostPort),
		Command:     "",
		Environment: lq.Environment{},
		SourceImage: "nbatlivala/test",
		PortMappings: []lq.PortMapping{
			{
				HostPort:      hostPort,
				ContainerPort: 80,
			},
		},
		Ram: ram,
		Cpu: cpu,
	}
}

func startJob(job *test.ContainerJobPublic, apiKey string) *lq.ContainerJob {
	id := api.CreateJob(job, apiKey)
	log.Infof("Started job with id %d", id)
	return api.GetJob(id, apiKey)
}

func stopJob(job *test.ContainerJobPublic, instanceId uint, apiKey string) {
	instance := api.GetInstance(instanceId, apiKey)
	jobUrl := fmt.Sprintf("%s:%d", instance.IP, job.PortMappings[0].HostPort)
	err := fmt.Errorf("placeholder")
	for err != nil {
		log.Infof("Attempting to stop job %d", job.ID)
		_, err = execute(false, "curl", "-X", "GET", jobUrl)
		if err != nil {
			log.Info(err)
			time.Sleep(time.Second)
		}
	}
	log.Infof("Stopped job %d", job.ID)
}

func waitForJobState(job *lq.ContainerJob, state mesos.TaskState, apiKey string) *lq.ContainerJob {
	log.Infof("Waiting for job %d to be %s", job.ID, state.String())
	for job.Status != state.Enum().String() {
		log.Debugf(fmt.Sprintf("Job state: %s", job.Status))
		time.Sleep(time.Second * time.Duration(5))
		job = api.GetJob(job.ID, apiKey)
	}
	log.Infof("Job %d is in state %s", job.ID, state.String())
	return job
}

func waitForInstanceStatus(resource *lq.ResourceInstance, status string, apiKey string) *lq.ResourceInstance {
	log.Infof("Waiting for resource %d to be %s", resource.ID, status)
	for resource.Status != status {
		log.Debugf(fmt.Sprintf("Resource state: %s", resource.Status))
		time.Sleep(time.Second * time.Duration(5))
		resource = api.GetInstance(resource.ID, apiKey)
	}
	log.Infof("Resource %d is in state %s", resource.ID, status)
	return resource
}

func waitForSchedulerToAttach(masterIp string) *MesosMasterState {
	state, err := getMesosMasterState(masterIp)
	log.Infof("Waiting for liquefy framework")
	for len(state.Frameworks) == 0 {
		if err != nil {
			panic(err)
		}
		time.Sleep(time.Second)
		log.Debugf("Waiting for liquefy framework")
		state, err = getMesosMasterState(masterIp)
	}
	log.Info("Liquify framework attached")
	time.Sleep(time.Second)
	return state
}

func initDB(masterIp string) {
	safeExecute(true, "go", "run", "../initDB.go", fmt.Sprintf("--masterip=%s", masterIp))
}

func deployScheduler(master string) {
	safeExecute(false, "bash", "../setup/deploy_vbox_scheduler.sh", master)
}

func setupMaster(master string) {
	safeExecute(true, "bash", "../setup/setup_vbox_master.sh", master)
}

func stopAllSlaves() {
	stopCmd := "/usr/local/bin/docker-machine ls | grep Running | grep vbox-slave- | awk '{ print $1 }' | while read slave; do /usr/local/bin/docker-machine stop $slave; done"
	safeExecute(true, stopCmd)
}

type Framework struct {
	Id string
}

type MesosMasterState struct {
	Frameworks []Framework
}

func getMesosMasterState(masterIp string) (*MesosMasterState, error) {
	targetUrl := fmt.Sprintf("http://%s:5050/master/state.json", masterIp)
	var state MesosMasterState

	data, err := api.Get(targetUrl, "")
	if err != nil {
		return &state, err
	}
	err = json.Unmarshal(data, &state)
	if err != nil {
		return &state, err
	}

	return &state, nil
}

func safeExecute(print bool, name string, arg ...string) string {
	out, err := execute(print, name, arg...)
	if err != nil {
		panic(err)
	}
	return out
}

func execute(print bool, name string, arg ...string) (string, error) {
	cmd := exec.Command(name, arg...)

	var out string
	var err error
	var stdout io.ReadCloser
	var stderr io.ReadCloser

	stdout, err = cmd.StdoutPipe()
	if err != nil {
		log.Error("Failed creating stdout pipe")
		panic(err)
	}
	stderr, err = cmd.StderrPipe()
	if err != nil {
		log.Error("Failed creating stderr pipe")
		panic(err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	stdoutScanner := bufio.NewScanner(stdout)
	stderrScanner := bufio.NewScanner(stderr)
	out = ""
	go func() {
		for stdoutScanner.Scan() {
			line := stdoutScanner.Text()
			if out == "" {
				out = line
			} else {
				out = out + line + "\n"
			}
			if print {
				log.Debugf(line)
			}
		}
		for stderrScanner.Scan() {
			line := stderrScanner.Text()
			if out == "" {
				out = line
			} else {
				out = out + line + "\n"
			}
			if print {
				log.Debugf(line)
			}
		}
		wg.Done()
	}()

	log.Infof(fmt.Sprintf("Running: %s %v", name, arg))
	err = cmd.Start()
	if err != nil {
		log.Error(err)
		panic(err)
	}

	err = cmd.Wait()
	if err != nil {
		log.Error(err)
	}

	wg.Wait()
	return strings.TrimSpace(out), err
}
