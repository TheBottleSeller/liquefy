package executor

import (
	"fmt"
	"errors"
	"io"
	"encoding/json"
	"strconv"
	"time"
	"bytes"

	"github.com/fsouza/go-dockerclient"
	log "github.com/Sirupsen/logrus"

	lq "bargain/liquefy/models"
)

type ContainerState string

const Container_Stopped ContainerState = "stopped"
const Container_Running ContainerState = "running"
const Container_Failed ContainerState = "failed"
const Container_Killed ContainerState = "killed"
const Container_Unknown ContainerState = "unknown"

type DockerExecutor interface {
	Start(job *lq.ContainerJob) (string, error)
	CreateContainer(job *lq.ContainerJob) (string, error)
	ContainerStatus(job *lq.ContainerJob) (ContainerState, error)
	CleanUp(job *lq.ContainerJob) error
	AttachContainer(id string, stdIn io.Reader, stdOut, stdErr io.Writer) error
	WaitOnContainer(id string) (int, error)
	ListAllContainers() ([]docker.APIContainers, error)
	KillContainer(jobId uint) error
}

type dockerExecutor struct {
	ca          string
	cert        string
	key         string
	port        int
	client      *docker.Client
}

func NewDockerExecutor(dockerEndpoint string) DockerExecutor{

	//endpoint := strings.TrimSpace(string(dockerEndpoint))
	//	ca := fmt.Sprintf("%s/ca.pem", inspectOutput.StorePath)
	//	cert := fmt.Sprintf("%s/cert.pem", inspectOutput.StorePath)
	//	key := fmt.Sprintf("%s/key.pem", inspectOutput.StorePath)
	//client, err = docker.NewTLSClient(endpoint, cert, key, ca);

	client, err := docker.NewClient(dockerEndpoint)
	if err != nil {
		log.Errorf("Error instantiating Docker client: %s", err)
		panic(err)
	}

	return &dockerExecutor{ca: "", cert: "", key: "", port: 2375 , client: client}
}

func NewDockerExecutorFromEnv() DockerExecutor {
	client, err := docker.NewClientFromEnv()
	if err != nil {
		log.Errorf("Error instantiating Docker client: %s", err)
		panic(err)
	}

	return &dockerExecutor{ca: "", cert: "", key: "", port: 2375 , client: client}
}

func (e *dockerExecutor) Start(job *lq.ContainerJob) (string,error) {
	err := e.client.StartContainer(job.ContainerId, nil)
	if err != nil {
		log.Error("Failed to start Container :", job.ContainerId , " : ",err)
		return job.ContainerId,err
	}

	return job.ContainerId, nil
}

func (e *dockerExecutor) ContainerStatus(job *lq.ContainerJob) (ContainerState,error) {

	container, err := e.client.InspectContainer(job.ContainerId)
	if err != nil {
		return Container_Unknown, err
	}

	if container.State.Running == true {
		return Container_Running, nil
	}

	// This is the exit code of a container killed with a SIGKILL.
	// Calls to the function KillContainer will result in this exit code.
	if container.State.ExitCode == 137 {
		return Container_Killed, nil
	}

	if container.State.OOMKilled {
		return Container_Failed, errors.New("Container was OOMKilled")
	}

	if  container.State.ExitCode > 0 {
		return Container_Failed, errors.New("Container Exited with non zero exit code")
	}

	if container.State.Error != "" {
		return Container_Failed, errors.New(container.State.Error)
	}

	return Container_Stopped, nil
}

func (e *dockerExecutor) CleanUp(job *lq.ContainerJob) error {

	removeOpts := docker.RemoveContainerOptions{
		ID: job.ContainerId,
	}

	if err := e.client.RemoveContainer(removeOpts);err != nil {
		log.Error("Removing Container Image Failed")
		return err
	}

	return nil
}

func (executor *dockerExecutor) CreateContainer(ctJob *lq.ContainerJob) (string, error) {
	//Default
	var image string

	//Build Image or ensure
	if (ctJob.SourceType == "code") {
		if err := executor.buildImageFromRepo(ctJob); err != nil {
			log.Error("Container image build failed")
			return "", err
		}
		image = "localbuild" + strconv.Itoa(int(ctJob.ID))
	} else if (ctJob.SourceType == "image") {
		// if the pull fails, then send status FAILED for this job
		err := executor.pullImage(ctJob.SourceImage, false)
		if err != nil {
			log.Error("Container image ensure failed")
			return "", err
		}
		image = ctJob.SourceImage
	} else {
		return "", errors.New("Invalid Source Type")
	}

	// Setup environment variables
	var envVars []lq.EnvVar
	if err := json.Unmarshal([]byte(ctJob.Environment), &envVars); err != nil {
		return "", lq.NewErrorf(err, "Failed parsing env vars from string %s", ctJob.Environment)
	}

	env := make([]string, len(envVars))
	for i, envVar := range envVars {
		env[i] = fmt.Sprintf("%s=%s", envVar.Variable, envVar.Value)
	}

	config := &docker.Config{
		Image:     image,
		Env:       env,
		OpenStdin: true,
		StdinOnce: true,
	}

	if ctJob.Command != "" {
		config.Cmd = []string{ "sh", "-c", ctJob.Command }
	}

	// Add port bindings if they exist
	var portMappings []lq.PortMapping
	if err := json.Unmarshal([]byte(ctJob.PortMappings), &portMappings); err != nil {
		return "", lq.NewErrorf(err, "Failed parsing port mappings from string %s", ctJob.Environment)
	}

	portBindings := make(map[docker.Port][]docker.PortBinding)
	for _, binding := range portMappings {
		hostPort := fmt.Sprintf("%d/tcp", binding.ContainerPort)
		portBindings[docker.Port(hostPort)] = []docker.PortBinding {
			docker.PortBinding {
				HostIP: "",
				HostPort: strconv.Itoa(binding.HostPort),
			},
		}
	}

	networkMode := "default"

	hostConfig := &docker.HostConfig{
		PortBindings: portBindings,
		NetworkMode: networkMode,
	}

	//If the container is GPU container, perform mount devnodes
	if (ctJob.Gpu > 0 ) {
		cGroupPerms := "rwm"
		uvm := docker.Device{
			PathOnHost: "/dev/nvidia-uvm",
			PathInContainer: "/dev/nvidia-uvm",
			CgroupPermissions: cGroupPerms,
		}
		ctl := docker.Device{
			PathOnHost: "/dev/nvidiactl",
			PathInContainer: "/dev/nvidiactl",
			CgroupPermissions: cGroupPerms,
		}
		gpu1 := docker.Device{
			PathOnHost: "/dev/nvidia0",
			PathInContainer: "/dev/nvidia0",
			CgroupPermissions: cGroupPerms,
		}
		hostConfig.Devices = []docker.Device{uvm, ctl, gpu1}

		//Fetch Cuda libs and mount them
		// TODO Fix this, it does not work
		// Mesos gives this message:
		// "Container Create Failed :fork/exec ls /usr/lib/x86_64-linux-gnu/libcuda* : no such file or directory"
		//output, err := exec.Command("ls /usr/lib/x86_64-linux-gnu/libcuda*").Output()

		cudaLibs := []string{
			"/usr/local/cuda-7.0/:/usr/local/cuda-7.0:ro",
			//"/usr/lib/x86_64-linux-gnu/libcuda.so:/usr/lib/x86_64-linux-gnu/libcuda.so:ro",
			//"/usr/lib/x86_64-linux-gnu/libcuda.so.346.46:/usr/lib/x86_64-linux-gnu/libcuda.so.346.46:ro",
			//"/usr/lib/x86_64-linux-gnu/libcuda.so.1:/usr/lib/x86_64-linux-gnu/libcuda.so.1:ro",
		}

		hostConfig.Binds = cudaLibs
	}

	opts := docker.CreateContainerOptions{
		Name: executor.getContainerName(ctJob.ID),
		Config: config,
		HostConfig: hostConfig,
	}

	container, err := executor.client.CreateContainer(opts)
	if err != nil {
		log.Error("Failed to create container")
		return "", err
	}

	return container.ID, nil
}

func (e *dockerExecutor) AttachContainer(id string, stdIn io.Reader, stdOut, stdErr io.Writer) error {
	attachOpts := docker.AttachToContainerOptions{
		Container:    id,
		InputStream:  stdIn,
		OutputStream: stdOut,
		ErrorStream:  stdErr,
		Stream:       true,
		Stdin:        true,
		Stdout:       true,
		Stderr:       true,
		RawTerminal:  false,
	}

	log.Info("Docker executor attaching")
	return e.client.AttachToContainer(attachOpts)
}

func (e *dockerExecutor) WaitOnContainer(id string) (int,error){
	 return e.client.WaitContainer(id)
}

func (e *dockerExecutor) ListAllContainers() ([]docker.APIContainers, error) {
	return e.client.ListContainers(docker.ListContainersOptions{All:true})
}

// If the container exists, it will be killed.
// This results in a SIGKILL being sent to the container. The exit code is used by
// ContainerStatus to detect that the container is in the ContainerKilled state
func (executor *dockerExecutor) KillContainer(jobId uint) error {
	containerName := executor.getContainerName(jobId)
	containers, err := executor.client.ListContainers(docker.ListContainersOptions{
		Filters: map[string][]string{
			"name": []string { containerName },
		},
	})
	if err != nil {
		return lq.NewErrorf(err, "Failed looking for container with name %s", containerName)
	}

	log.Debugf("Found containers to kill: %v", containers)
	if len(containers) == 0 {
		return fmt.Errorf("No container found to kill for job %d", jobId)
	}

	containerId := containers[0].ID
	log.Debugf("Killing container %s", containerId)
	err = executor.client.KillContainer(docker.KillContainerOptions{
		ID: containerId,
	})
	if err != nil {
		return lq.NewErrorf(err, "Failed to kill container %s", containerId)
	}
	return nil
}

//func (e *dockerExecutor) ensureImage(name string, force bool) error {
//	image, err := e.client.InspectImage(name)
//	if err == docker.ErrNoSuchImage || force {
//		log.Infof("Pulling image %s", name)
//		if err = e.pullImage(name); err != nil {
//			return err
//		}
//	}
//
//	if force && image != nil {
//		newImage, err := e.client.InspectImage(name)
//		if err != nil {
//			return err
//		}
//		// Only remove image if new ID is different than old ID
//		if newImage.ID != image.ID {
//			e.removeImage(image.ID)
//		}
//	}
//	return err
//}

func (executor *dockerExecutor) pullImage(name string, force bool) error {
	log.Infof("Pulling image %s", name)
	_, err := executor.client.InspectImage(name)

	// If image does not exist or force pull, then pull the image
	if err == docker.ErrNoSuchImage || force {
		opts := docker.PullImageOptions{
			Repository: name,
		}
		err = executor.client.PullImage(opts, docker.AuthConfiguration{})
		if err != nil {
			return lq.NewError(fmt.Sprintf("Failed pulling image %s", name), err)
		}
	} else if err != nil {
		return lq.NewError(fmt.Sprintf("Failed inspecting image %s", name), err)
	} else {
		log.Infof("Image %s already exists", name)
	}

	return nil
}

func (e *dockerExecutor) buildImageFromRepo(ctJob *lq.ContainerJob) error{
	//TODO: Maybe chroot here for security
	//TODO :Cleanup io reader

	log.Info("Cloning Repo")

	buf := new(bytes.Buffer)
	r := new(bytes.Buffer)

	name:= "localbuild" + strconv.Itoa(int(ctJob.ID))

	opts := docker.BuildImageOptions{
		Name: name,
		Remote: ctJob.SourceImage,
		Memory: int64(ctJob.Ram * 1024 * 1024),
		InputStream: r,
		OutputStream: buf,
	}

	go func(){
		for true {
			log.Info(buf.String())
			time.Sleep(10000 * time.Duration(time.Millisecond))
		}
	}()

	log.Info("Starting Build Process")
	err := e.client.BuildImage(opts)
	log.Info("Build Complete")

	return err
}

func (executor *dockerExecutor) removeImage(name string) error {
	log.Infof("Removing image %s", name)
	err := executor.client.RemoveImage(name)
	if err != nil {
		log.Error("Failed to remove image %s", name)
	}
	return err
}

func (executor *dockerExecutor) getContainerName(jobId uint) string {
	return fmt.Sprintf("lq-job-%d", jobId)
}