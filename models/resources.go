package models

const (
	ResourceStatusNew               = "new"
	ResourceProvisioning            = "provisioning"
	ResourceSpotBidding             = "bidding"
	ResourceSpotBidAccepted         = "bid-accepted"
	ResourceStatusProvisioned       = "provisioned"
	ResourceStatusRunning           = "running"
	ResourceStatusDeprovisioning    = "deprovisioning"
	ResourceStatusDeprovisioned     = "deprovisioned"
)

type ResourceInstance struct {
	ID          uint            `gorm:primary_key json:"id"`
	OwnerId     uint            `json:"owner_id"`
	RamTotal    int             `json:"ram_total"`
	RamUsed     int             `json:"ram_used"`
	CpuTotal    float64         `json:"cpu_total"`
	CpuUsed     float64         `json:"cpu_used"`
	GpuTotal    int             `json:"gpu_total"`
	GpuUsed     int             `json:"gpu_used"`
	Status      string          `json:"status"`
	LaunchTime  int64           `json:"launch_time"`
	IP          string          `json:"ip"`

	// Internal
	SlaveID         string      `json:"slave_id"`
	UserTerminated  bool        `json:"user_terminated"`

	// Amazon specific
	AwsInstanceId       string  `json:"aws_instance_id"`
	AwsAvailabilityZone string  `json:"aws_availability_zone"`
	AwsInstanceType     string  `json:"aws_instance_type"`
	AwsSpotPrice        float64 `json:"aws_spot_price"`
}

type ResourceEvent struct {
	ID          uint            `gorm:primary_key json:"id"`
	InstanceID  uint            `json:"instance_id" sql:"not null"`
	Status      string          `json:"status" sql:"not null"`
	Time        int64           `json:"time" sql:"not null"`
	Msg         string          `json:"msg" sql:"type:varchar(1024);not null"`
}