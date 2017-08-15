package executor

import (
	"io"
	"os"
	"sync"
	"strconv"
	"bufio"
	"bytes"
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"
	exec "github.com/mesos/mesos-go/executor"
	mesos "github.com/mesos/mesos-go/mesosproto"

	lq "bargain/liquefy/models"
)

func init() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
}

type liquidExecutor struct {
	tasksLaunched       int
	containerExecutor   DockerExecutor
}

func NewLiquidExecutor(dockerEndpoint string) *liquidExecutor {
	return &liquidExecutor{
		tasksLaunched:      0,
		containerExecutor:  NewDockerExecutor(dockerEndpoint),
	}
}

func (exec *liquidExecutor) Registered(driver exec.ExecutorDriver, execInfo *mesos.ExecutorInfo, fwinfo *mesos.FrameworkInfo, slaveInfo *mesos.SlaveInfo) {
	log.Info("Registered executor on slave: ", slaveInfo.GetHostname())
	log.Info("Slave attributes: ", slaveInfo.Attributes)
}

func (exec *liquidExecutor) Reregistered(driver exec.ExecutorDriver, slaveInfo *mesos.SlaveInfo) {
	log.Info("Re-registered Executor on slave ", slaveInfo.GetHostname())
}

func (exec *liquidExecutor) Disconnected(exec.ExecutorDriver) {
	log.Info("Executor disconnected.")
}

func (exec *liquidExecutor) LaunchTask(driver exec.ExecutorDriver, taskInfo *mesos.TaskInfo) {
	log.Infof("Launch task called with mesos task %s", taskInfo.GetName())

	ctjob, err := lq.DeserializeJob(taskInfo.Data)
	if err != nil {
		log.Info("Failed to deserialize container job data")
		exec.sendStatusUpdate(driver, taskInfo, mesos.TaskState_TASK_FAILED,
			fmt.Sprintf("Failed to deserialize container job data %s ", err))
		return
	}

	// Create Container
	log.Infof("Creating container for job: %d", ctjob.ID)
	containerId, err := exec.containerExecutor.CreateContainer(ctjob)
	if err != nil {
		log.Error("Container Create Failed :", err)
		exec.sendStatusUpdate(driver, taskInfo, mesos.TaskState_TASK_FAILED, err.Error())
		return
	}

	// Update the task data to send back
	ctjob.ContainerId = containerId
	bytData, err := lq.SerializeJob(ctjob)
	if err != nil {
		log.Error("Container Serialized Failed :", err)
		exec.sendStatusUpdate(driver, taskInfo, mesos.TaskState_TASK_FAILED, err.Error())
		return
	}

	taskInfo.Data = bytData
	log.Info("Container ID ", containerId)
	exec.sendStatusUpdate(driver, taskInfo, mesos.TaskState_TASK_STARTING, "")

	var wg sync.WaitGroup
	var outBuffer, errBuffer io.Writer
	var stdIn io.Reader

	stdOutReader, stdOutWriter := io.Pipe()
	stdErrReader, stdErrWriter := io.Pipe()

	//WTF IS THIS / JUST REMOVE IT
	if ctjob.UsesFilePipe() {
		f, err := os.Create(ctjob.FilePipePath())
		if err != nil {
			panic(err)
			return
		}

		f.Close()
		defer os.Remove(ctjob.FilePipePath())
	} else {
		buffer := &bytes.Buffer{}

		if ctjob.UsesStdOutPipe() {
			outBuffer = buffer
		} else if ctjob.UsesStdErrPipe() {
			errBuffer = buffer
		}
	}

	// Attach Container and Track
	go func() {
		defer stdOutWriter.Close()
		defer stdErrWriter.Close()
		err = exec.containerExecutor.AttachContainer(ctjob.ContainerId, stdIn, stdOutWriter, stdErrWriter)
		if err != nil {
			panic(err)
		}
	}()

	// Start Running
	_, err = exec.containerExecutor.Start(ctjob)
	if err != nil {
		log.Error("Container Start Failed :", err)
		exec.sendStatusUpdate(driver, taskInfo, mesos.TaskState_TASK_FAILED, err.Error())
		return
	}
	exec.sendStatusUpdate(driver, taskInfo, mesos.TaskState_TASK_RUNNING, "")

	go func() {
		// Wait for job to finish asynchronously by capturing stdout and stderr
		wg.Add(2)

		go func() {
			defer wg.Done()
			exec.capture(ctjob, stdOutReader, outBuffer, driver)
		}()

		go func() {
			defer wg.Done()
			exec.capture(ctjob, stdErrReader, errBuffer, driver)
		}()

		wg.Wait()

		// Introduce an artifical sleep to every job that is terminated to ensure that we capture all of the logs
		time.Sleep(time.Duration(5) * time.Second)

		// Report the status of the completed job
		if status, err := exec.containerExecutor.ContainerStatus(ctjob); err != nil {
			exec.sendStatusUpdate(driver, taskInfo, mesos.TaskState_TASK_ERROR, err.Error())
		} else if status == Container_Failed {
			exec.sendStatusUpdate(driver, taskInfo, mesos.TaskState_TASK_FAILED, "")
		} else if status == Container_Stopped {
			exec.sendStatusUpdate(driver, taskInfo, mesos.TaskState_TASK_FINISHED, "")
		} else if status == Container_Killed {
			exec.sendStatusUpdate(driver, taskInfo, mesos.TaskState_TASK_KILLED, "")
		}
	}()
}

// Setting of status to be killed will be handled by the thread spawned at the end of LaunchTask
func (executor *liquidExecutor) KillTask(driver exec.ExecutorDriver, taskId *mesos.TaskID) {
	log.Error("Killing task %s", taskId.GetValue())
	jobId, err := strconv.Atoi(taskId.GetValue())
	if err != nil {
		err = lq.NewErrorf(err, "Failed to parse task id to get a job id. Task id: %s", taskId.GetValue())
		log.Error(err)
		return
	}

	err = executor.containerExecutor.KillContainer(uint(jobId))
	if err != nil {
		err = lq.NewErrorf(err, "Failed killing task for job %d", jobId)
		log.Error(err)
	}
}

func (exec *liquidExecutor) FrameworkMessage(driver exec.ExecutorDriver, msg string) {
	log.Error("Got framework message: ", msg)
}

func (exec *liquidExecutor) Shutdown(exec.ExecutorDriver) {
	log.Error("Shutting down the executor")
}

func (exec *liquidExecutor) Error(driver exec.ExecutorDriver, err string) {
	log.Error("Got error message:", err)
}

// ----------------- Helper Methods ----------------------- //

func (exec *liquidExecutor) sendStatusUpdate(driver exec.ExecutorDriver, taskInfo *mesos.TaskInfo, state mesos.TaskState, message string) {
	log.Infof("Updating task %s with status %s", taskInfo.GetName(), state.String())

	log.Infof("TASK INFO : %s", taskInfo)
	job, err := lq.DeserializeJob(taskInfo.GetData())
	if err != nil {
		exec.forceSendFail(driver, taskInfo)
		return
	}

	//Send the correct task status to master
	im := lq.StatusMessage{*job, message}
	statusMsg, err := lq.SerializeStatusMessage(&im)
	if err != nil {
		log.Error("Failed to serialize Status message " + err.Error())
		exec.forceSendFail(driver, taskInfo)
		return
	}

	status := &mesos.TaskStatus{
		TaskId: taskInfo.GetTaskId(),
		State:  state.Enum(),
		Data:   statusMsg,
	}

	if _, err := driver.SendStatusUpdate(status); err != nil {
		log.Error("Failed to update status of task %s", taskInfo.GetName())
		log.Error(err)
	}
}

func (exec *liquidExecutor) forceSendFail(driver exec.ExecutorDriver, taskInfo *mesos.TaskInfo) {
	if _, err := driver.SendStatusUpdate(&mesos.TaskStatus{
		TaskId: taskInfo.GetTaskId(),
		State:  mesos.TaskState_TASK_FAILED.Enum(),
		Data:   nil,
	}); err != nil {
		log.Error("Failed to update status of task %s", taskInfo.GetName())
		log.Error(err)
	}
}

func (exec *liquidExecutor) capture(cjob *lq.ContainerJob, r io.Reader, w io.Writer, driver exec.ExecutorDriver) {

	scanner := bufio.NewScanner(r)
	capture := !cjob.UsesDelimitedOutput()

	//TODO :: We should just intgreate filebeat.go as a lib into our executor
	//Put it behind an executor startup flag
	//Make it deal WITH HA Elastic Search
	//Deal

	for scanner.Scan() {
		line := scanner.Text()

		//TODO: Probably have some filebeat spooler.
		fmt.Println(line)

		if w != nil {
			if cjob.UsesDelimitedOutput() && line == cjob.EndDelimiter {
				capture = false
			}
			if capture {
				w.Write(append([]byte(line), '\n'))
			}
			if cjob.UsesDelimitedOutput() && line == cjob.BeginDelimiter {
				capture = true
			}
		}
	}
}
