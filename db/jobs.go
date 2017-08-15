package db

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	mesos "github.com/mesos/mesos-go/mesosproto"

	lq "bargain/liquefy/models"
	"time"
)

const MAX_RETRIES = 3

type ContainerJobsTable interface {
	Create(job *lq.ContainerJob) error

	Get(jobID uint) (*lq.ContainerJob, error)
	GetActiveJobsOnResource(resourceID uint) ([]*lq.ContainerJob, error)
	GetAssignedJobsByInstances(instanceIDs []uint) ([]*lq.ContainerJob, error)
	GetUnassignedJobsByUser(userID uint) ([]*lq.ContainerJob, error)
	GetAllJobsByUser(userID uint) ([]*lq.ContainerJob, error)
	GetNonTerminatedUserTerminatedJobs() ([]*lq.ContainerJob, error)

	GetAllNonCompletedJobs() ([]*lq.ContainerJob, error)

	SetStatus(jobId uint, status string, statusMsg string) (err error)
	SetTotalCost(jobID uint, cost float64) error
	SetContainerId(jobID uint, containerId string) error

	MarkUserTerminated(jobId uint) error
	Delete(jobID uint) error
}

type containerJobsTable struct{}

func Jobs() ContainerJobsTable {
	return &containerJobsTable{}
}

func (table *containerJobsTable) Create(job *lq.ContainerJob) (err error) {
	tx := db.Begin()
	defer TxCommitOrRollback(tx, &err, "Failed creating job: %v", job)

	job.Status = mesos.TaskState_TASK_STAGING.String()
	if err = tx.Create(job).Error; err != nil {
		return
	}

	jobEvent := &lq.ContainerJobTracker{
		ContainerJobID: job.ID,
		Time:           time.Now().UTC().UnixNano(),
		InstanceID:     0,
		Status:         mesos.TaskState_TASK_STAGING.String(),
		Attempt:        job.RetryCount,
	}
	if err = tx.Create(&jobEvent).Error; err != nil {
		return
	}

	return
}

func (table *containerJobsTable) Get(jobID uint) (*lq.ContainerJob, error) {
	var job lq.ContainerJob
	query := db.Find(&job, jobID)
	if query.Error != nil {
		err := lq.NewErrorf(query.Error, "Failed getting job %d", jobID)
		log.Error(err)
		return &job, err
	}
	return &job, query.Error
}

// Get all jobs that are staging, starting, or running on resource
func (table *containerJobsTable) GetActiveJobsOnResource(resourceID uint) ([]*lq.ContainerJob, error) {
	jobs := []*lq.ContainerJob{}
	query := db.Where("(status = ? OR status = ? OR status = ? OR status = ?) AND instance_id = ?",
		mesos.TaskState_TASK_STAGING.String(),
		lq.ContainerJobStatusLaunched,
		mesos.TaskState_TASK_STARTING.String(),
		mesos.TaskState_TASK_RUNNING.String(),
		resourceID).Find(&jobs)
	if query.Error != nil {
		return jobs, lq.NewErrorf(query.Error, "Failed getting active jobs on resource %d", resourceID)
	}
	return jobs, nil
}

func (table *containerJobsTable) GetAssignedJobsByInstances(instanceIDs []uint) ([]*lq.ContainerJob, error) {
	jobs := []*lq.ContainerJob{}
	if len(instanceIDs) == 0 {
		return jobs, nil
	}
	query := db.Where("status = ? AND instance_id IN (?) AND user_terminated = false",
		mesos.TaskState_TASK_STAGING.String(), instanceIDs).Find(&jobs)
	if query.Error != nil {
		err := lq.NewErrorf(query.Error, "Failed getting assigned jobs by instances: %v", instanceIDs)
		log.Error(err)
		return jobs, err
	}
	return jobs, nil
}

func (table *containerJobsTable) GetUnassignedJobsByUser(userID uint) ([]*lq.ContainerJob, error) {
	jobs := []*lq.ContainerJob{}
	query := db.Where("status = ? AND instance_id = 0 AND owner_id = ? AND user_terminated = false",
		mesos.TaskState_TASK_STAGING.String(), userID).Find(&jobs)
	if query.Error != nil {
		err := lq.NewErrorf(query.Error, "Failed getting unassigned jobs by user %s", userID)
		log.Error(err)
		return jobs, err
	}
	return jobs, nil
}

func (table *containerJobsTable) GetAllJobsByUser(userID uint) ([]*lq.ContainerJob, error) {
	jobs := []*lq.ContainerJob{}
	query := db.Where("owner_id = ?", userID).Find(&jobs)
	if query.Error != nil {
		err := lq.NewErrorf(query.Error, "Failed get all jobs by user %s", userID)
		log.Error(err)
		return jobs, err
	}
	return jobs, nil
}

// Get all jobs that are marked for user termination and not terminated (failed, finished, or killed)
func (table *containerJobsTable) GetNonTerminatedUserTerminatedJobs() ([]*lq.ContainerJob, error) {
	jobs := []*lq.ContainerJob{}
	query := db.Where("user_terminated = true AND status != ? AND status != ? AND status != ?",
		mesos.TaskState_TASK_FAILED.String(),
		mesos.TaskState_TASK_FINISHED.String(),
		mesos.TaskState_TASK_KILLED.String()).Find(&jobs)
	if query.Error != nil {
		return jobs, lq.NewErrorf(query.Error, "Failed getting all non-terminated, user-terminated jobs")
	}
	return jobs, nil
}

func (table *containerJobsTable) SetTotalCost(jobID uint, cost float64) error {
	sql := fmt.Sprintf("UPDATE container_job SET total_cost = %f WHERE id = %d", cost, jobID)
	query := db.Exec(sql)
	if query.Error != nil {
		err := lq.NewErrorf(query.Error, "Unable to set total cost %f for job %s", cost, jobID)
		log.Error(err)
		return err
	}
	return nil
}

// Get all jobs that are  not  (failed, finished, or killed)
func (table *containerJobsTable) GetAllNonCompletedJobs() ([]*lq.ContainerJob, error) {
	jobs := []*lq.ContainerJob{}
	query := db.Where("status != ? AND status != ? AND status != ?",
		mesos.TaskState_TASK_FAILED.String(),
		mesos.TaskState_TASK_FINISHED.String(),
		mesos.TaskState_TASK_KILLED.String()).Find(&jobs)
	if query.Error != nil {
		return jobs, lq.NewErrorf(query.Error, "Failed getting all non completed jobs")
	}
	return jobs, nil
}

func (table *containerJobsTable) SetContainerId(jobID uint, containerId string) error {
	sql := fmt.Sprintf("UPDATE container_job SET container_id = '%s' WHERE id = %d", containerId, jobID)
	query := db.Exec(sql)
	if query.Error != nil {
		err := lq.NewErrorf(query.Error, "Failed updating job %d with containerId %s", jobID, containerId)
		log.Error(err)
		return err
	}
	return nil
}

func (table *containerJobsTable) MarkUserTerminated(jobId uint) error {
	sql := fmt.Sprintf("UPDATE container_job SET user_terminated = true WHERE id = %d", jobId)
	query := db.Exec(sql)
	if query.Error != nil {
		err := lq.NewErrorf(query.Error, "Failed updating job %d as user terminated", jobId)
		return err
	}
	return nil
}

func (table *containerJobsTable) SetStatus(jobId uint, status string, statusMsg string) (err error) {
	// Run in transaction
	tx := db.Begin()
	defer TxCommitOrRollback(tx, &err, "Failed setting job %d status to %s", jobId, status)

	now := time.Now().UTC().UnixNano()
	var job lq.ContainerJob
	if err = tx.Find(&job, jobId).Error; err != nil {
		return
	}

	if err = validateStateTransition(job.Status, status); err != nil {
		return
	}

	// Create job status event
	jobEvent := &lq.ContainerJobTracker{
		ContainerJobID: jobId,
		Time:           now,
		InstanceID:     job.InstanceID,
		Status:         status,
		Attempt:        job.RetryCount,
		Msg:            statusMsg,
	}
	if err = tx.Create(&jobEvent).Error; err != nil {
		return
	}

	// Update start time if necessary
	if lq.ContainerJobStatusLaunched == status {
		sql := fmt.Sprintf("UPDATE container_job SET start_time = %d WHERE id = %d", now, jobId)
		if err = tx.Exec(sql).Error; err != nil {
			return
		}
	}

	// Update the retry count if the status is failed or lost and reset the status to staging
	if mesos.TaskState_TASK_ERROR.String() == status || mesos.TaskState_TASK_LOST.String() == status {
		if job.RetryCount < MAX_RETRIES {
			// increment the retry count
			job.RetryCount += 1
			sql := fmt.Sprintf("UPDATE container_job SET retry_count = %d WHERE id = %d", job.RetryCount, jobId)
			if err = tx.Exec(sql).Error; err != nil {
				return
			}

			// retry job by setting status to staging
			status = mesos.TaskState_TASK_STAGING.String()
			statusMsg = "Retrying"
		} else {
			// dont retry job and set job to failed
			status = mesos.TaskState_TASK_FAILED.String()
			statusMsg = "Failed, no more retries"
		}

		// persist the new status change event
		jobEvent := &lq.ContainerJobTracker{
			ContainerJobID: jobId,
			Time:           time.Now().UTC().UnixNano(),
			InstanceID:     job.InstanceID,
			Status:         status,
			Attempt:        job.RetryCount,
			Msg:            statusMsg,
		}
		if err = tx.Create(&jobEvent).Error; err != nil {
			return
		}
	}

	// Update end time if necessary
	if status == mesos.TaskState_TASK_KILLED.String() ||
		status == mesos.TaskState_TASK_FAILED.String() ||
		status == mesos.TaskState_TASK_FINISHED.String() {
		sql := fmt.Sprintf("UPDATE container_job SET end_time = %d WHERE id = %d", now, jobId)
		if err = tx.Exec(sql).Error; err != nil {
			return
		}
	}

	// Persist the status in the job
	sql := fmt.Sprintf("UPDATE container_job SET status = '%s' WHERE id = %d", status, jobId)
	if err = tx.Exec(sql).Error; err != nil {
		return
	}

	return
}

func (table *containerJobsTable) Delete(jobID uint) error {
	query := db.Where("id = ?", jobID).Delete(&lq.ContainerJob{})
	if query.Error != nil {
		err := lq.NewErrorf(query.Error, "Failed deleting job %d", jobID)
		log.Error(err)
		return err
	}
	return nil
}

func validateStateTransition(currentState string, targetState string) error {
	validStartStates := []string{
		mesos.TaskState_TASK_STAGING.String(),
		lq.ContainerJobStatusLaunched,
		mesos.TaskState_TASK_STARTING.String(),
		mesos.TaskState_TASK_RUNNING.String(),
	}

	successTransition := make(map[string]string)
	failureTransition := make(map[string][]string)

	successTransition[mesos.TaskState_TASK_STAGING.String()] = lq.ContainerJobStatusLaunched
	successTransition[lq.ContainerJobStatusLaunched] = mesos.TaskState_TASK_STARTING.String()
	successTransition[mesos.TaskState_TASK_STARTING.String()] = mesos.TaskState_TASK_RUNNING.String()
	successTransition[mesos.TaskState_TASK_RUNNING.String()] = mesos.TaskState_TASK_FINISHED.String()

	failureTransition[mesos.TaskState_TASK_STAGING.String()] = []string{
		mesos.TaskState_TASK_ERROR.String(), // ex: trying to provision a resource for a staged job fails
		mesos.TaskState_TASK_KILLED.String(),
	}
	failureTransition[lq.ContainerJobStatusLaunched] = []string{
		mesos.TaskState_TASK_STAGING.String(), // this can happen if the launching of a job fails from the mesos driver
		mesos.TaskState_TASK_LOST.String(),
		mesos.TaskState_TASK_FAILED.String(),
		mesos.TaskState_TASK_ERROR.String(),
		mesos.TaskState_TASK_KILLED.String(),
	}
	failureTransition[mesos.TaskState_TASK_STARTING.String()] = []string{
		mesos.TaskState_TASK_LOST.String(),
		mesos.TaskState_TASK_FAILED.String(),
		mesos.TaskState_TASK_ERROR.String(),
		mesos.TaskState_TASK_KILLED.String(),
	}
	failureTransition[mesos.TaskState_TASK_RUNNING.String()] = []string{
		mesos.TaskState_TASK_LOST.String(),
		mesos.TaskState_TASK_FAILED.String(),
		mesos.TaskState_TASK_ERROR.String(),
		mesos.TaskState_TASK_KILLED.String(),
	}

	// Verify that we start in an expected state
	if !stateInList(currentState, validStartStates) {
		return fmt.Errorf("%s is not a valid starting state", currentState)
	}

	if targetState == successTransition[currentState] || stateInList(targetState, failureTransition[currentState]) {
		return nil
	}

	return fmt.Errorf("Invalid job state transition: %s to %s", currentState, targetState)
}

func stateInList(state string, states []string) bool {
	for _, validState := range states {
		if validState == state {
			return true
		}
	}
	return false
}
