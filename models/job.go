package models

import (
    "bytes"
    "fmt"
    "strings"
    "crypto/md5"
    "encoding/gob"

    log "github.com/Sirupsen/logrus"
    mesos "github.com/mesos/mesos-go/mesosproto"
    "github.com/jinzhu/gorm"
)

// Status represents the different statuses a job can have.

const (
    ContainerJobStatusLaunched = "TASK_LAUNCHED"
)

type ContainerJobGroup struct {
    gorm.Model
    Name          string
    OwnerID       uint
    ContainerJobs []ContainerJob
    Status        string //Aggregate Status
    Mode          string //Waterfall || Parallel
}

type ContainerJob struct {
    ID              uint            `gorm:"primary_key" json:"id"`
    Name            string          `json:"name"`
    Command         string          `json:"command"`
    OwnerID         uint            `json:"owner_id"`
    Status          string          `json:"status"`
    SourceImage     string          `json:"source_image"`
    SourceType      string          `json:"source_type"` // "code" or "image"
    Environment     string          `json:"environment"` // array of env vars
    PortMappings    string          `json:"port_mappings"` // array of port mappings

    Ram             int             `json:"ram"`
    Cpu             float64         `json:"cpu"`
    Gpu             int             `json:"gpu"`

    //Internal
    InstanceID      uint            `json:"instance_id"`
    ContainerId     string          `json:"container_id"`
    RetryCount      int             `json:"retry_count"`
    UserTerminated  bool            `json:"user_terminated"`

    //Detail Tracking
    StartTime       int64           `json:"start_time"`
    EndTime         int64           `json:"end_time"`
    TotalCost       float64         `json:"total_cost"`

    Output          string          `json:"output"`
    BeginDelimiter  string          `json:"begin_delimiter"`
    EndDelimiter    string          `json:"end_delimiter"`
}

func (job *ContainerJob) IsTerminated() bool {
    return job.Status == mesos.TaskState_TASK_KILLED.String() ||    // finished via user termination
        job.Status == mesos.TaskState_TASK_FAILED.String() ||       // finished via failure
        job.Status == mesos.TaskState_TASK_FINISHED.String()        // finished via success
}

type PortMapping struct {
    HostPort      int `json:"host_port"`
    ContainerPort int `json:"container_port"`
}

type ContainerJobTracker struct {
    gorm.Model
    ContainerJobID  uint        `sql:"not null"`
    Time            int64       `sql:"not null"`
    InstanceID      uint
    Status          string      `sql:"not null"`
    Attempt         int         `sql:"not null"`
    Msg             string      `sql:"not null"`
}

type ContainerJobLog struct {
    Index int      `json:"index,omitempty"`
    Lines []string `json:"lines"`
}

// EnvVar represents an environment variable and its associated value.
type EnvVar struct {
    Variable string `json:"variable"`
    Value    string `json:"value"`
}

func (js ContainerJob) UsesStdOutPipe() bool {
    return js.Output == "stdout" || js.Output == ""
}

func (js ContainerJob) UsesStdErrPipe() bool {
    return js.Output == "stderr"
}

func (js ContainerJob) UsesFilePipe() bool {
    return strings.HasPrefix(js.Output, "/")
}

func (js ContainerJob) FilePipePath() string {
    return fmt.Sprintf("/tmp/%x", md5.Sum([]byte(js.SourceImage)))
}

func (js ContainerJob) UsesDelimitedOutput() bool {
    return len(js.BeginDelimiter) > 0 && len(js.EndDelimiter) > 0
}

func SerializeJob(job *ContainerJob) ([]byte, error) {
    buf := bytes.Buffer{}
    enc := gob.NewEncoder(&buf)
    err := enc.Encode(job)
    if err != nil {
        log.Error(err)
    }
    return buf.Bytes(), err
}

func DeserializeJob(content []byte) (*ContainerJob, error) {
    job := ContainerJob{}
    buf := bytes.Buffer{}
    buf.Write(content)
    dec := gob.NewDecoder(&buf)
    err := dec.Decode(&job)
    if err != nil {
        log.Error(err)
    }
    return &job, err
}
