package db

import (
    lq "bargain/liquefy/models"
)

type AssignmentsTable interface {
    AssignJob(jobID uint, resource *lq.ResourceInstance, createResource bool) error
    UnassignJob(jobID uint) error
}

type assignmentsTable struct{}

func Assignments() AssignmentsTable {
    return &assignmentsTable{}
}

func (table *assignmentsTable) AssignJob(jobID uint, resource *lq.ResourceInstance, createResource bool) (err error) {
    var job lq.ContainerJob

    tx := db.Begin()
    defer TxCommitOrRollback(tx, &err, "Failed assigning job %d to resource %v", jobID, resource)

    if createResource {
        if err = Resources().CreateInTx(tx, resource); err != nil {
            return
        }
    }

    if err = db.Find(&job, jobID).UpdateColumn("instance_id", resource.ID).Error; err != nil {
        return
    }

    if err = tx.Model(&resource).UpdateColumn("ram_used", resource.RamUsed + job.Ram).Error; err != nil {
        return
    }

    if err = tx.Model(&resource).UpdateColumn("cpu_used", resource.CpuUsed + job.Cpu).Error; err != nil {
        return
    }

    if err = tx.Model(&resource).UpdateColumn("gpu_used", resource.GpuUsed + job.Gpu).Error; err != nil {
        return
    }

    return nil
}

func (table *assignmentsTable) UnassignJob(jobID uint) (err error) {
    var job lq.ContainerJob
    var instance lq.ResourceInstance

    tx := db.Begin()
    defer TxCommitOrRollback(tx, &err, "Failed unassigning job %d", jobID)

    if err = tx.Find(&job, jobID).Error; err != nil {
        return
    }

    if job.InstanceID == 0 {
        return
    }

    if err = tx.Find(&instance, job.InstanceID).Error; err != nil {
        return
    }

    if err = tx.Model(&job).UpdateColumn("instance_id", 0).Error; err != nil {
        return
    }

    if err = tx.Model(&instance).UpdateColumn("ram_used", instance.RamUsed - job.Ram).Error; err != nil {
        return
    }

    if err = tx.Model(&instance).UpdateColumn("cpu_used", instance.CpuUsed - job.Cpu).Error; err != nil {
        return
    }

    if err = tx.Model(&instance).UpdateColumn("gpu_used", instance.GpuUsed - job.Gpu).Error; err != nil {
        return
    }

    return
}