package scheduler

import (
	"fmt"
	"net"
	"strconv"
	"errors"
	"time"

	"github.com/gogo/protobuf/proto"
	log "github.com/Sirupsen/logrus"

	mesos "github.com/mesos/mesos-go/mesosproto"
	"github.com/mesos/mesos-go/mesosutil"
	sched "github.com/mesos/mesos-go/scheduler"

	"bargain/liquefy/db"
	lq "bargain/liquefy/models"
	aws "bargain/liquefy/cloudprovider"
	lqEngine "bargain/liquefy/scheduler/liquidengine"
)

var MasterPort = 5050
var SchedulerPort = 9090
var ExecutorPort = 4949

var FetcherTimeoutUserTerminatedJobs = time.Duration(15) * time.Second
var FetcherTimeoutResourceTerminations = time.Duration(15) * time.Second

type AssignEvent struct {
	jobId           uint
	resource        *lq.ResourceInstance
	createResource  bool
}

type LaunchEvent struct {
	jobId   uint
	offer   *mesos.Offer
}

type UserTerminationEvent struct {
	jobId   uint
}

type LqScheduler interface {
	Run() (mesos.Status, error)
}

type lqScheduler struct {
	executor        *mesos.ExecutorInfo
	engine          lqEngine.CostEngine
	driver          *sched.MesosSchedulerDriver
	eventChan       chan interface{}
}

//
// How the LqScheduler works
// Once a job is launched on mesos, we can rely on mesos internals to handle the sychronosity of job status changes.
// Before a job gets into mesos, we need to consider that the job can race between the following events:
// - AssignEvent
//      - when a job is assigned to an existing resource OR when a job is assigned to a to-be-created resource
// - LaunchEvent
//      - mesos has recieved the offer from this resource, and the job is launched there
// - UserTerminationEvent
//      - when a user terminates a job, if the job is not yet terminated, it is sent this event
//
// The scheduler has a single thread event handler that processes the above event.
//
// The events are generated by:
// - Mesos Resource Offers
//      - Assign Event
//      - Launch Event
// - User termination thread: looks for all non-terminated jobs that have been user terminated
//      - User termination event
//
func NewLqScheduler(bindIp, mesosMasterIp, executorIp string, executorLaunch string ) LqScheduler {
	// Setup Executor Info
	executorInfo := &mesos.ExecutorInfo{
		ExecutorId: mesosutil.NewExecutorID("Liquefy"),
		Name:       proto.String("Liquefy"),
		Source:     proto.String("Liquefy-Master"),
		Command: &mesos.CommandInfo{
			Value: proto.String(executorLaunch),
			Uris:  []*mesos.CommandInfo_URI{
				&mesos.CommandInfo_URI{
					Value: proto.String(fmt.Sprintf("http://%s:%d/executor", executorIp, ExecutorPort)),
					Executable: proto.Bool(true),
				},
			},
		},
	}

	scheduler := &lqScheduler{
		executor: executorInfo,
		engine: lqEngine.NewCostEngine(),
		eventChan: make(chan interface{}, 10 * 1024),
	}

	// Setup Driver Config
	// Set failover timeout to a  week in seconds (i.e. the master will never lose the framework)
	timeout := float64(60 * 60 * 24 * 7)
	frameworkInfo := &mesos.FrameworkInfo{
		User: proto.String(""), // Mesos-go will fill in user.
		Name: proto.String("Liquefy"),
		FailoverTimeout: &timeout,
	}

	if frameworkId, err := db.Mesos().GetFrameworkId(); err != nil {
		panic(err)
	} else if frameworkId != "" {
		log.Debugf("Registering with framework id: %s", frameworkId)
		frameworkInfo.Id = &mesos.FrameworkID{
			Value: &frameworkId,
		}
	}

	config := sched.DriverConfig{
		Scheduler:      scheduler,
		Framework:      frameworkInfo,
		Master:         fmt.Sprintf("%s:%d", mesosMasterIp, MasterPort),
		BindingAddress: net.ParseIP(bindIp),
		BindingPort:    uint16(SchedulerPort),
	}

	driver, err := sched.NewMesosSchedulerDriver(config)
	if err != nil {
		panic(err)
	}

	scheduler.driver = driver

	// Start the event handler thread
	go scheduler.eventHandlerThread()

	// Start fetcher thread for user terminated jobs
	go scheduler.fetchUserTerminatedJobs()

	// Start thread that handlers terminated resources with assigned jobs
	go scheduler.handleResourceTerminations()

	return scheduler
}

func (sched *lqScheduler) Run() (mesos.Status, error) {
	return sched.driver.Run()
}

func (sched *lqScheduler) eventHandlerThread() {
	for event := range sched.eventChan {
		if assignEvent, ok := event.(*AssignEvent); ok {
			if assignEvent.createResource {
				log.Debug("Recieved assign event for job to create a new resource")
			} else {
				log.Debugf("Recieved assign event for job %d to resource %d", assignEvent.jobId, assignEvent.resource.ID)
			}

			if err := sched.handleAssignEvent(assignEvent); err != nil {
				log.Error(lq.NewErrorf(err, "Failed assigning job %d to resource %d", assignEvent.jobId, assignEvent.resource.ID))
			}
		} else if launchEvent, ok := event.(*LaunchEvent); ok {
			log.Debugf("Recieved launch event for job %d", launchEvent.jobId)

			if err := sched.handleLaunchEvent(launchEvent); err != nil {
				log.Error(lq.NewErrorf(err, "Failed launching job %d", launchEvent.jobId))
			}
		} else if userTermEvent, ok := event.(*UserTerminationEvent); ok {
			log.Debugf("Recieved user termination event for job %d", userTermEvent.jobId)

			if err := sched.handleUserTerminationEvent(userTermEvent); err != nil {
				log.Error(lq.NewErrorf(err, "Failed user terminating job %d", userTermEvent.jobId))
			}
		} else {
			log.Errorf("Recieved invalid event %v", event)
		}
	}
}

// Assign Event
// This event assigns a job to a resource, and potentially creates the resource if it does not exist
// The creation of a resource is due to the resourc being provisioned specifically for this job
//
// Expected Modes:
//  - job should be in state staging and the call to AssignJob will create the resource if necessary and do the
//    the bookkeeping on cpu, ram, gpu resources
//
// Failure Modes:
//  - job is not staging
//      - do not assign the job to the resource (this could be due to a user termination for example)
//  - resource being assigned to is not running
//      - do not assign the job
func (sched *lqScheduler) handleAssignEvent(event *AssignEvent) error {
	job, err := db.Jobs().Get(event.jobId)
	if err != nil {
		return lq.NewErrorf(err, "Failed assigning job %d to resource %d", event.jobId, event.resource.ID)
	}

	if job.Status != mesos.TaskState_TASK_STAGING.String() {
		return lq.NewErrorf(err, "Failed assigning job %d to resource %d because job is in state %s",
			event.jobId, event.resource.ID, job.Status)
	}

	if ! event.createResource {
		// Verify that the resource being assigned to is running
		resource, err := db.Resources().Get(event.resource.ID)
		if err != nil {
			return lq.NewErrorf(err, "Failed assigning job %d to resource %d", event.jobId, event.resource.ID)
		}

		if resource.Status != lq.ResourceStatusRunning {
			return lq.NewErrorf(nil, "Failed assigning job %d to resource %d with status %s",
				event.jobId, resource.ID, resource.Status)
		}
	}

	err = db.Assignments().AssignJob(event.jobId, event.resource, event.createResource)
	if err != nil {
		return lq.NewErrorf(err, "Failed assigning job %d to instance %d", event.jobId, event.resource.ID)
	}

	return nil
}

// Launch events
//
// Expected mode:
// - threadA: staging task is assigned to job and sends launch event
// - threadB: launch event is recieved, status is set to launched, job is launched on mesos
//
// Failure modes:
//  - Job is terminated
//      This could be due to a user termination or a resource termination, in either case, do not launch the job
//  - Job is not correctly assigned to the mesos offer
//      Do not launch the job on this offer and return
func (sched *lqScheduler) handleLaunchEvent(event *LaunchEvent) error {
	job, err := db.Jobs().Get(event.jobId)
	if err != nil {
		return lq.NewErrorf(err, "Failed processing launch event for job %d", event.jobId)
	}

	// If job was terminated, do nothing
	if job.IsTerminated() {
		log.Debugf("Skipping launch of job %d. Is already terminated with status %s", job.ID, job.Status)
		return nil
	}

	// If job is not correctly assigned to offer, do not launch
	resourceId := sched.parseInstanceIDFromOffer(event.offer)
	if job.InstanceID != resourceId {
		log.Debugf("Skipping launch of job %d on resource %d because job is assigned to resource %d",
			job.ID, resourceId, job.InstanceID)
		return nil
	}

	// Set status to launched, safe to do because it is not running on mesos yet
	err = db.Jobs().SetStatus(job.ID, lq.ContainerJobStatusLaunched, "")
	if err != nil {
		return err
	}

	err =  sched.launchJob(job, event.offer)
	if err != nil {
		return lq.NewErrorf(err, "Failed launching job %d", job.ID)
	}

	return nil
}

// User Termination Event
// Job statuses:
//  - job is staging and unassigned to a resource
//      - set status to killed
//  - job is staging and assigned to a resource
//      - unassign job from resource
//      - set status to killed
//  - job is launched
//      - may not be launched to mesos yet, do nothing and will receive another User Term Event.
//		- wait until job is starting (guaranteed to be on executor and can thus be killed by mesos driver)
//  - job is terminated (finished, failed, or killed already)
//      - do nothing
//  - Job is starting or running
//      - kill the job via the mesos driver
//      - wait for the killde update status to be recieved from mesos
func (sched *lqScheduler) handleUserTerminationEvent(event *UserTerminationEvent) error {
	job, err := db.Jobs().Get(event.jobId)
	if err != nil {
		return lq.NewErrorf(err, "Failed processing user termination event for job %d", event.jobId)
	}

	log.Debugf("Killing job %d at users request", job.ID)
	if job.Status == mesos.TaskState_TASK_STAGING.String() {
		if job.InstanceID != 0 {
			err := db.Assignments().UnassignJob(job.ID)
			if err != nil {
				return lq.NewErrorf(err, "Failed unassigning job after failed mesos launching of job %d", job.ID)
			}
		}

		err = db.Jobs().SetStatus(job.ID, mesos.TaskState_TASK_KILLED.String(), "Job killed by user")
		if err != nil {
			return lq.NewErrorf(err, "Failed setting job %d status back to staging after failed mesos launch", job.ID)
		}
	} else if job.Status == lq.ContainerJobStatusLaunched {
		log.Debugf("Waiting for user-terminated, launched job %d to be running on mesos before killing the task", job.ID)
		// Do nothing
	} else if job.IsTerminated() {
		log.Debugf("User terminated an already finished job %d", event.jobId)
		// Do nothing
	} else {
		// The update to TASK_KILL will come from Mesos
		if _, err = sched.driver.KillTask(sched.getMesosTaskId(job)); err != nil {
			return lq.NewErrorf(err, "Failed reaping job %d marked for termination by user", job.ID)
		}
	}

	return nil
}

func (sched *lqScheduler) fetchUserTerminatedJobs() {
	clock := time.NewTicker(FetcherTimeoutUserTerminatedJobs)
	for range clock.C {
		userTerminatedJobs, err := db.Jobs().GetNonTerminatedUserTerminatedJobs()
		if err != nil {
			log.Error(lq.NewErrorf(err, "Failed getting non-terminated, user terminated jobs"))
			continue
		}

		for _, job := range userTerminatedJobs {
			userTermEvent := &UserTerminationEvent{
				jobId: job.ID,
			}
			sched.eventChan <- userTermEvent
		}
	}
}

func (sched *lqScheduler) handleResourceTerminations() {
	clock := time.NewTicker(FetcherTimeoutResourceTerminations)
	for range clock.C {
		terminatedResourceIds, err := db.Resources().GetTerminatedResourceIdsWithAssignedJobs()
		if err != nil {
			log.Error(lq.NewErrorf(err, "Failed getting terminated resources with assigned jobs"))
			continue
		}

		for _, resourceId := range terminatedResourceIds {
			// for all jobs that are staging and intended to run on this resource, unassign the job
			var stagedJobs []lq.ContainerJob
			err := db.GetDB().Where("instance_id = ? AND status = ? AND user_terminated = false",
				resourceId, mesos.TaskState_TASK_STAGING.String()).Find(&stagedJobs).Error
			if err != nil {
				log.Error(lq.NewErrorf(err, "Failed finding jobs staging and assigned to resource %d", resourceId))
				continue
			}

			// TODO: FIX THE HACK OF NOT SHARING MEMORY OF AWS MARKET MONITOR MAP
			if resource, err := db.Resources().Get(resourceId); err != nil {
				log.Error(lq.NewErrorf(err, "Failed getting resource to mark market as unavailable"))
			} else {
				aws.MarkMarketUnavailable(aws.AZ(resource.AwsAvailabilityZone), aws.InstanceType(resource.AwsInstanceType))
			}

			for _, job := range stagedJobs {
				err = db.Assignments().UnassignJob(job.ID)
				if err != nil {
					log.Error(lq.NewErrorf(err, "Failed unassigning job %d from resource %d due to resource termination",
						job.ID, job.InstanceID))
				}
			}
		}
	}
}

func (sched *lqScheduler) Registered(driver sched.SchedulerDriver, frameworkId *mesos.FrameworkID,
		masterInfo *mesos.MasterInfo) {
	log.Info("Framework Registered with Master ", masterInfo)

	dbFrameworkId, err := db.Mesos().GetFrameworkId()
	if err != nil {
		log.Error("FAILED GETTING FRAMEWORK ID FROM DB!!!!")
		driver.Abort()
		return
	}

	// Persist the framework id if we dont have one
	if dbFrameworkId == "" {
		if err := db.Mesos().SetFrameworkId(frameworkId.GetValue()); err != nil {
			log.Error("FAILED PERSISTING FRAMEWORK ID!!!!!")
			driver.Abort()
			return
		}
	}

	//Find all non Killed / Failed / Finished tasks and reconcile
	jobs,err := db.Jobs().GetAllNonCompletedJobs()
	if (err != nil ){
		log.Error("FAILED to reconcile jobs on register :" +  err.Error())
		driver.Abort()
	}

	//TODO: Handle FUcking trash states we have added : EG Launched
	tasksStatuses := []*mesos.TaskStatus{}
	for _,e := range jobs {
		id := strconv.Itoa(int(e.ID))
		state := mesos.TaskState(mesos.TaskState_value[e.Status])
		tasksStatuses = append(tasksStatuses,&mesos.TaskStatus{
			TaskId: &mesos.TaskID{
				Value: &id,
			},
			State: &state,
		})
	}
	driver.ReconcileTasks(tasksStatuses)

}

func (sched *lqScheduler) Reregistered(driver sched.SchedulerDriver, masterInfo *mesos.MasterInfo) {
	log.Info("Framework Re-Registered with Master ", masterInfo)

}

func (sched *lqScheduler) Disconnected(sched.SchedulerDriver) {
	log.Error("disconnected from master, aborting")
	log.Error("disconnected from master, aborting")
	log.Error("disconnected from master, aborting")
	log.Error("disconnected from master, aborting")

	//What happend now ?
}

func (sched *lqScheduler) ResourceOffers(driver sched.SchedulerDriver, offers []*mesos.Offer) {

	//
	// Process offers: sort by users and create unused offer set
	//
	offersByUser := make(map[uint][]*mesos.Offer)
	unusedOffersByUser := make(map[uint]map[*mesos.Offer]struct{})

	// Always try to release unused offers
	defer func() {
		offers := []*mesos.Offer{}
		for _, unusedOffers := range unusedOffersByUser {
			for offer := range unusedOffers {
				offers = append(offers, offer)
			}
		}
		sched.releaseUnusedOffers(offers)
	}()

	for _, offer := range offers {
		// Sort by users
		ownerId, err := sched.getOffersOwnerID(offer)
		if err != nil {
			sched.releaseUnusedOffers([]*mesos.Offer{ offer })
			continue
		}

		// Create if structs if first time visiting user
		if _, ok := offersByUser[ownerId]; !ok {
			offersByUser[ownerId] = []*mesos.Offer{}
			unusedOffersByUser[ownerId] = make(map[*mesos.Offer]struct{})
		}

		offersByUser[ownerId] = append(offersByUser[ownerId], offer)
		unusedOffersByUser[ownerId][offer] = struct{}{}
	}

	//
	// Process jobs for each user
	// - process assigned jobs
	// - process unassigned jobs
	//
	users, err := db.Users().GetAllWithPendingJobs()
	if err != nil {
		log.Error("Failed to retrieve all users")
		return
	}

	for _, user := range users {
		userOffers, ok := offersByUser[user.ID]
		if !ok {
			userOffers = []*mesos.Offer{}
		}

		instanceIds := make([]uint, len(userOffers))
		instanceIDToOffer := make(map[uint]*mesos.Offer)
		for i, offer := range userOffers {
			instanceId := sched.parseInstanceIDFromOffer(offer)
			instanceIDToOffer[instanceId] = offer
			instanceIds[i] = instanceId
		}

		// Handle users assigned jobs: find and launch on each jobs associated offer
		assignedJobs, err := db.Jobs().GetAssignedJobsByInstances(instanceIds)
		if err == nil {
			for _, assignedJob := range assignedJobs {
				if assignedJob.InstanceID == 0 {
					log.Errorf("Job %d does not have an instance id but we think it is assigned", assignedJob.ID)
					continue
				}

				offer, found := instanceIDToOffer[assignedJob.InstanceID]
				if !found {
					log.Debugf("Job %d was assigned to instance %d, but we could not find an offer. Keep waiting for offer",
					assignedJob.ID, assignedJob.InstanceID)
					continue
				}

				launchEvent := &LaunchEvent{
					jobId: assignedJob.ID,
					offer: offer,
				}

				sched.eventChan <- launchEvent
				delete(unusedOffersByUser[user.ID], offer)
			}
		} else {
			log.Errorf("Failed getting users %d already assigned jobs", user.ID)
		}

		// Handle users unassigned jobs: try to fit into unused offers and if not provision an instance
		unassignedJobs, err := db.Jobs().GetUnassignedJobsByUser(user.ID)
		if err != nil {
			log.Errorf("Failed getting users %d unassigned jobs", user.ID)
		}

		for _, unassignedJob := range unassignedJobs {
			if unassignedJob.InstanceID != 0 {
				log.Errorf("Job %d is unassigned but has a non-zero instance id %d",
					unassignedJob.ID, unassignedJob.InstanceID)
				continue
			}

			// See if the job fits into an existing, unused offer
			jobLaunched := false
			for offer := range unusedOffersByUser[user.ID] {
				if sched.offerSatisfiesJob(offer, unassignedJob) {
					// TODO: Cache this instance retrieval
					instanceID := sched.parseInstanceIDFromOffer(offer)
					instance, err := db.Resources().Get(instanceID)
					if err != nil {
						log.Error(lq.NewErrorf(err, "Failed fetching instance for offer %s", offer.GetId().GetValue()))
						continue
					}
					if instance.Status == lq.ResourceStatusRunning {
						log.Infof("Found existing offer for job %d", unassignedJob.ID)

						assignEvent := &AssignEvent{
							jobId:          unassignedJob.ID,
							resource:       instance,
							createResource: false,
						}

						launchEvent := &LaunchEvent{
							jobId: unassignedJob.ID,
							offer: offer,
						}

						// These need to be two separate events because assign is distinct from launch.
						// This decision is driven by the need to assign when a new resource is created, and
						// launch when that resource has been provisioned
						sched.eventChan <- assignEvent
						sched.eventChan <- launchEvent
						delete(unusedOffersByUser[user.ID], offer)
						break
					}
				}
			}

			if !jobLaunched {
				log.Infof("Provisioning resource for job %d", unassignedJob.ID)
				req := &lqEngine.SpotRequest{
					Cpu:    unassignedJob.Cpu,
					Memory: float64(unassignedJob.Ram),
					Gpu:    float64(unassignedJob.Gpu),
					Disk:   0.0, // We do not currently support disk matching
				}

				// Calculate the optimal spot price match for this request
				spotMatch, err := sched.engine.Match(user.ID, req)
				if err != nil {
					log.Error(lq.NewErrorf(err, "Failed to provision resource for job %d", unassignedJob.ID))
					continue
				}

				instanceInfo := aws.AvailableInstances[spotMatch.AwsInstanceType]
			    optimalResource := &lq.ResourceInstance{
				    OwnerId:             user.ID,
					AwsAvailabilityZone: spotMatch.AwsAvailabilityZone.String(),
					AwsInstanceType:     spotMatch.AwsInstanceType.String(),
					AwsSpotPrice:        spotMatch.AwsSpotPrice,
					RamTotal:            int(instanceInfo.Memory),
					CpuTotal:            instanceInfo.Cpu,
					GpuTotal:            int(instanceInfo.Gpu),
					RamUsed:             0,
					CpuUsed:             0.0,
					GpuUsed:             0,
					Status:              lq.ResourceStatusNew,
				}

				log.Infof("Assigning job %d to optimal resource", unassignedJob.ID)
				assignEvent := &AssignEvent{
					jobId:          unassignedJob.ID,
					resource:       optimalResource,
					createResource: true,
				}

				sched.eventChan <- assignEvent
			}
		}
	}
}

func (sched *lqScheduler) releaseUnusedOffers(offers []*mesos.Offer) {
	for _, offer := range offers {
		sched.driver.DeclineOffer(offer.Id, &mesos.Filters{RefuseSeconds: proto.Float64(20)})
	}
}

func (sched *lqScheduler) StatusUpdate(driver sched.SchedulerDriver, status *mesos.TaskStatus) {
	log.Infof("Status Update\nTask %s in state %s\nSource: %s\nReason: %s\nMessage: %s",
		status.GetTaskId().GetValue(), status.GetState(), status.GetSource(), status.GetReason(), status.GetMessage())
	jobIdString := *status.GetTaskId().Value
	jobId, err := strconv.Atoi(jobIdString)
	if err != nil {
		log.Error("Could not get job id from task id to update status")
		return
	}

	//TODO : Swtich this to protobufs and make life easy
	// There will be no status message if the message is lost (i.e sent directly from mesos)
	statusMsg := &lq.StatusMessage{
		ContainerJob: lq.ContainerJob{ID: uint(jobId)},
		StatusMessage: "",
	}
	extractedMsg , err := lq.DeserializeStatusMessage(status.GetData())
	if err != nil {
		log.Error("Failed to Deserialize for task : " + err.Error())
		statusMsg.StatusMessage = "No valid status Message message form the executor , maybe mesos called this status"
	}else{
		statusMsg = extractedMsg
	}

	job, err := db.Jobs().Get(uint(jobId))
	if err != nil {
		log.Error(lq.NewErrorf(err, "Failed processing update event for job %d", uint(jobId)))
		return
	}

	// If the task transitions to starting, store the container id
	if status.GetState() == mesos.TaskState_TASK_STARTING {
		if statusMsg.ContainerJob.ContainerId == "" {
			log.Error(fmt.Errorf("Failed recieving container id when setting job %d status to TASK_STARTING", job.ID))
		} else {
			log.Infof("Recieved container id: %s", statusMsg.ContainerJob.ContainerId)
			err := db.Jobs().SetContainerId(job.ID, statusMsg.ContainerJob.ContainerId)
			if err != nil {
				log.Error("Failed setting container id: ", err)
				return
			}
		}
	}

	err = db.Jobs().SetStatus(job.ID, status.GetState().String(), statusMsg.StatusMessage)
	if err != nil {
		log.Error(lq.NewErrorf(err, "Failed setting status of job %d to %s", job.ID, status.String()))
		return
	}

	// Re-fetch the job to get the current state after the state transition
	job, err = db.Jobs().Get(job.ID)
	if err != nil {
		log.Error(lq.NewErrorf(err, "Could not get re-fetch job %d to get current status", job.ID))
		return
	}

	// If the job is finished, calculate and update the cost of the job
	if job.IsTerminated() {
		// if a job does not have a launch time, then it was never sent to the resource and thus we can assume that
		// the resource was not properly provisioned and there was no cost incurred for the job
		if job.StartTime > 0 {
			err = sched.updateCostOfJob(job.ID)
			if err != nil {
				log.Error(lq.NewErrorf(err, "Failed to update cost of job %d", job.ID))
				return
			}
		}
	}

	// TODO: This should be in the same transaction as set status
	// If the job failed, unassign the job from the resource and
	// deprovision the resource if it has no other running jobs on it
	jobFailed := (status.GetState() == mesos.TaskState_TASK_LOST ||
		status.GetState() == mesos.TaskState_TASK_KILLED ||
		status.GetState() == mesos.TaskState_TASK_FAILED ||
		status.GetState() == mesos.TaskState_TASK_ERROR ||
		status.GetState() == mesos.TaskState_TASK_FINISHED)
	if jobFailed && job.InstanceID != 0 {
		log.Infof("Unregistering job %d from resource %d", job.ID, job.InstanceID)
		err = db.Assignments().UnassignJob(job.ID)
		if err != nil {
			log.Error(err)
		}
	}

	return
}

func (sched *lqScheduler) OfferRescinded(_ sched.SchedulerDriver, oid *mesos.OfferID) {
	//TODO : Mark resource as Deprovisionable
	log.Errorf("offer rescinded: %v", oid)
}

func (sched *lqScheduler) FrameworkMessage(driver sched.SchedulerDriver, eid *mesos.ExecutorID, sid *mesos.SlaveID, msg string) {
	log.Infof("Recieved framework message: %s", msg)
}

func (sched *lqScheduler) SlaveLost(_ sched.SchedulerDriver, sid *mesos.SlaveID) {
	log.Errorf("Slave lost: %v", sid)

	//If we loose a slave we want to recover all jobs / make some updates

	//TODO : Implement Rcover Slave Jobs // Jobs should usually report lost automatically
	//TODO : Implement HEALTH / BILLING / OTHER

}

func (sched *lqScheduler) ExecutorLost(_ sched.SchedulerDriver, eid *mesos.ExecutorID, sid *mesos.SlaveID,
		code int) {
	log.Errorf("Executor %s lost on slave %s. Exit code %d", eid.GetValue(), sid.GetValue(), code)
}

func (sched *lqScheduler) Error(_ sched.SchedulerDriver, err string) {
	log.Errorf("Scheduler received error:", err)
	log.Error("Scheduler received error:", err)
}

// ----------------- Custom DataBase Methods ------------------ //

func (sched *lqScheduler) getOffersOwnerID(offer *mesos.Offer) (uint, error) {
	resourceID := sched.parseInstanceIDFromOffer(offer)
	if resourceID == 0 {
		return 0, errors.New("Dummy slave does not have an owner")
	}

	resource, err := db.Resources().Get(resourceID)
	if err != nil {
		log.Error(lq.NewErrorf(err, "Failed getting owner of offer %s", offer.Id.GetValue()))
		return 0, err
	}

	return resource.OwnerId, nil
}

/*
 * Finds the total cost of running a job on an AWS instance
 */
func (sched *lqScheduler) updateCostOfJob(jobId uint) (error) {
	log.Debugf("Tracking cost of job %d", jobId)
	job, err := db.Jobs().Get(jobId)
	if err != nil {
		return lq.NewErrorf(err, "Failed updating cost of job %d", jobId)
	}

	resource, err := db.Resources().Get(job.InstanceID)
	if err != nil {
		return lq.NewErrorf(err, "Failed updating cost of job %d", jobId)
	}

	cost, err := sched.engine.GetResourceCostWithAwsApi(resource, time.Unix(0, job.StartTime), time.Unix(0, job.EndTime))
	if err != nil {
		return lq.NewErrorf(err, "Failed updating cost of job %d", jobId)
	}

	log.Infof("Job %d: Total cost $%f", jobId, cost)
	err = db.Jobs().SetTotalCost(job.ID, job.TotalCost + cost)
	if err != nil {
		return lq.NewErrorf(err, "Failed updating cost of job %d", jobId)
	}
	return nil
}

//------------------ STUPID RANDOM UTILITY HELPER FUNCTIONS -------------------- //

func (sched *lqScheduler) parseInstanceIDFromOffer(offer *mesos.Offer) uint {
	for _, attribute := range offer.Attributes {
		if *attribute.Name == "liquefyid" {
			return uint(*attribute.Scalar.Value)
		}
	}

	return 0 // IDs start from 1 so this is an invalid ID
}

func (sched *lqScheduler) offerSatisfiesJob(offer *mesos.Offer, job *lq.ContainerJob) bool {
	cpuResources := mesosutil.FilterResources(offer.Resources, func(res *mesos.Resource) bool {
		return res.GetName() == "cpus"
	})
	cpus := 0.0
	for _, res := range cpuResources {
		cpus += res.GetScalar().GetValue()
	}

	memResources := mesosutil.FilterResources(offer.Resources, func(res *mesos.Resource) bool {
		return res.GetName() == "mem"
	})
	mems := 0.0
	for _, res := range memResources {
		mems += res.GetScalar().GetValue()
	}

	gpuResources := mesosutil.FilterResources(offer.Resources, func(res *mesos.Resource) bool {
		return res.GetName() == "gpus"
	})
	gpus := 0.0
	for _, res := range gpuResources {
		gpus += float64(res.GetSet().Size())
	}

	log.Info(fmt.Sprintf("Cpu: offer = %f, job = %f\nRam: offer = %f, job = %d\nGpus: offer = %f, job = %d",
		cpus, job.Cpu, mems, job.Ram, gpus, job.Gpu))
	if job.Cpu <= cpus && job.Ram <= int(mems) && job.Gpu <= int(gpus) {
		return true
	}

	return false
}

func (sched *lqScheduler) launchJob(job *lq.ContainerJob, offer *mesos.Offer) error {
	log.Infof("Launching job %d onto offer %s", job.ID, offer.Id.GetValue())

	jobData, err := lq.SerializeJob(job)
	if err != nil {
		err = lq.NewErrorf(err, "Failed serializing the job %d", job.ID)
		log.Error(err)
		return err
	}

	//TODO :: Fix GPU Use
	//Encode a system of marking the GPU number under use
	// EG []string{gpu0:use,gpu1:free}

	task := &mesos.TaskInfo{
		Name:     &job.Name,
		TaskId:   sched.getMesosTaskId(job),
		SlaveId:  offer.SlaveId,
		Executor: sched.executor,
		Resources: []*mesos.Resource{
			mesosutil.NewScalarResource("cpus", float64(job.Cpu)),
			mesosutil.NewScalarResource("mem", float64(job.Ram)),

			//TODO fix gpu use
			//mesosutil.NewSetResource("gpus", []string{"0"}),
		},
		Data: jobData,
	}

	// Launch via mesos driver
	_, err = sched.driver.LaunchTasks([]*mesos.OfferID{offer.Id}, []*mesos.TaskInfo{task},
		&mesos.Filters{RefuseSeconds: proto.Float64(20)})
	return err
}

func (sched *lqScheduler) getMesosTaskId(job *lq.ContainerJob) *mesos.TaskID {
	return &mesos.TaskID{
		Value: proto.String(strconv.Itoa(int(job.ID))),
	}
}
