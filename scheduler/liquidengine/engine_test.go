package liquidengine

import(
    "fmt"
    "time"
    ."github.com/smartystreets/goconvey/convey"
    "testing"

    mesos "github.com/mesos/mesos-go/mesosproto"

    lq "bargain/liquefy/models"
    "bargain/liquefy/db"
)

//func TestEngineMatchingGpu(t *testing.T) {
//    engine, err := NewCostEngine()
//    if err != nil {
//        panic(err)
//    }
//    Convey("Test price matcher works for gpu", t, func() {
//        stats := &JobStats{
//            Cpu: 0.0,
//            Memory: 0.0,
//            Disk: 0.0,
//            Gpu: 1.0,
//        }
//        spotPrice, err := engine.Match(stats)
//        if err != nil {
//            panic(err)
//        }
//        fmt.Println(spotPrice)
//    })
//}
//
//func TestEngineMatchingCpu(t *testing.T) {
//    engine, err := NewCostEngine()
//    if err != nil {
//        panic(err)
//    }
//    Convey("Test price matcher works for cpu", t, func() {
//        stats := &JobStats{
//            Cpu: 32.0,
//            Memory: 1.0,
//            Disk: 0.0,
//            Gpu: 0.0,
//        }
//        spotPrice, err := engine.Match(stats)
//        if err != nil {
//            panic(err)
//        }
//        fmt.Println(spotPrice)
//    })
//}
//
//func TestEngineMatchingMemory(t *testing.T) {
//    engine, err := NewCostEngine()
//    if err != nil {
//        panic(err)
//    }
//    Convey("Test price matcher works for memory", t, func() {
//        stats := &JobStats{
//            Cpu: 1.0,
//            Memory: 32.0 * 1024.0,
//            Disk: 0.0,
//            Gpu: 0.0,
//        }
//        spotPrice, err := engine.Match(stats)
//        if err != nil {
//            panic(err)
//        }
//        fmt.Println(spotPrice)
//    })
//}
//
//func TestEngineMatchingDisk(t *testing.T) {
//    engine, err := NewCostEngine()
//    if err != nil {
//        panic(err)
//    }
//    Convey("Test price matcher works for disk", t, func() {
//        stats := &JobStats{
//            Cpu: 1.0,
//            Memory: 1.0,
//            Disk: 1024 * 1024.0,
//            Gpu: 0.0,
//        }
//        spotPrice, err := engine.Match(stats)
//        if err != nil {
//            panic(err)
//        }
//        fmt.Println(spotPrice)
//    })
//}

func TestTrackingResourceCost(t *testing.T) {
    engine, err := NewCostEngine()
    db.Connect("52.90.208.13")
    if err != nil {
        panic(err)
    }
    Convey("Track the cost of a mock resource", t, func() {
        launchTime := time.Now().Add(-20 * time.Second)
        resource := &lq.ResourceInstance{
            AwsAvailabilityZone:    "us-east-1a",
            AwsInstanceType:        "g2.2xlarge",
            AwsSpotPrice:           0.50,
            RamTotal:               1024,
            CpuTotal:               2.0,
            RamUsed:                0,
            CpuUsed:                0.0,
            Status:                 lq.ResourceStatusNew,
            OwnerId:                0,
            LaunchTime:             launchTime.Unix(),
        }

        err := db.Resources().Create(resource)
        if err != nil {
            panic(err)
        }
        defer func() {
            db.Resources().Delete(resource.ID)
        }()

        cost, err := engine.TrackResourceCost(resource.ID)
        if err != nil {
            panic(err)
        }

        fmt.Printf("Total cost: %f\n", cost)
    })
}

func TestTrackingJobCost(t *testing.T) {
    engine, err := NewCostEngine()
    db.Connect("52.90.208.13")
    if err != nil {
        panic(err)
    }
    Convey("Track the cost of a mock resource", t, func() {
        launchTime := time.Now().Add(-20 * time.Hour)
        jobStartTime := time.Now().Add(-2 * time.Hour)
        jobEndTime := time.Now()

        resource := &lq.ResourceInstance{
            AwsAvailabilityZone:    "us-east-1a",
            AwsInstanceType:        "g2.2xlarge",
            LaunchTime:             launchTime.Unix(),
        }

        err := db.Resources().Create(resource)
        if err != nil {
            panic(err)
        }
        defer func() {
            db.Resources().Delete(resource.ID)
        }()

        job := &lq.ContainerJob{
            InstanceID: resource.ID,
            Status: mesos.TaskState_TASK_FINISHED.String(),
            StartTime: jobStartTime.Unix(),
            EndTime: jobEndTime.Unix(),
        }

        err = db.Jobs().Create(job)
        if err != nil {
            panic(err)
        }
        defer func() {
           db.Jobs().Delete(job.ID)
        }()

        cost, err := engine.GetResourceCostWithAwsApi(resource, jobStartTime, jobEndTime)
        if err != nil {
            panic(err)
        }

        fmt.Printf("Total cost: %f\n", cost)
    })
}