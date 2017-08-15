package api

import (
	"fmt"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	mesos "github.com/mesos/mesos-go/mesosproto"

	"bargain/liquefy/db"
	lq "bargain/liquefy/models"
	lqCloud "bargain/liquefy/cloudprovider"
)

type omit bool

type ContainerJobPublic struct {
	ID           uint             `json:"id,omitempty"`
	Name         string           `json:"name,omitempty"`
	Command      string           `json:"command"`
	OwnerID      uint             `json:"owner_id,omitempty"`
	Status       string           `json:"status,omitempty"`
	SourceType   string           `json:"source_type,omitempty"`
	SourceImage  string           `json:"source_image,omitempty"`
	Environment  []lq.EnvVar      `json:"environment,omitempty"`
	PortMappings []lq.PortMapping `json:"port_mappings,omitempty"`
	Ram          int              `json:"ram,omitempty"`
	Cpu          float64          `json:"cpu,omitempty"`
	Gpu          int              `json:"gpu"`
}

// Helper function to parse user from context //

func fetchUserFromContext(c *gin.Context) *lq.User {
	user, ok := c.Keys["user"].(*lq.User)
	if ok {
		return user
	}
	c.JSON(402, errors.New("User was invalid for the given token"))
	return nil
}

//------------------USER--------------------//

func GetUser(c *gin.Context) {
	user := fetchUserFromContext(c)

	c.JSON(http.StatusOK, struct {
		*lq.User
		Password omit `json:"password,omitempty"`
		ApiKey   omit `json:"apiKey,omitempty"`
		ID       omit `json:"id,omitempty"`
	}{
		User: user,
	})
}

func LinkAwsAccount(c *gin.Context) {
	//Get user from the context
	user := fetchUserFromContext(c)
	account := &lq.AwsAccount{}
	if err := c.BindJSON(&account); err != nil {
		c.JSON(http.StatusBadRequest, err)
		return
	}

	if user.AwsAccountID != 0 {
		c.JSON(http.StatusNotAcceptable, lq.NewErrorf(nil, "User %s already has AWS account linked", user.Email).Error())
		return
	}

	//Verify Policies
	awsCloud := lqCloud.NewAwsCloud(account.AwsAccessKey, account.AwsSecretKey)
	if err := awsCloud.VerifyPolicy(); err != nil {
		c.JSON(http.StatusNotAcceptable, lq.NewErrorf(err, "Failed linking AWS account for user %s", user.Email).Error())
		return
	}

	if err := db.AwsAccounts().Create(user.ID, account); err != nil {
		c.JSON(http.StatusInternalServerError, lq.NewErrorf(err, "Failed creating aws account for user %s", user.Email).Error())
		return
	}

	c.JSON(http.StatusCreated, "")
}

func SetupAwsAccount(c *gin.Context) {
	user := fetchUserFromContext(c)

	if user.AwsAccountID == 0 {
		c.JSON(http.StatusNotAcceptable, lq.NewErrorf(nil, "User %s does not have an AWS account linked", user.Email).Error())
		return
	}

	account, err := db.AwsAccounts().Get(user.AwsAccountID)
	if err != nil {
		c.JSON(http.StatusNotAcceptable, lq.NewError("Failed to get user aws account info", err).Error())
		return
	}

	awsCloud := lqCloud.NewAwsCloud(account.AwsAccessKey, account.AwsSecretKey)

	account, setupError := awsCloud.SetupAwsAccountResources(account)
	dbErr := db.AwsAccounts().Update(account)
	if setupError != nil {
		c.JSON(http.StatusPreconditionFailed, setupError.Error())
	}

	if dbErr != nil {
		c.JSON(http.StatusInternalServerError, lq.NewErrorf(dbErr, "Failed setting up aws account").Error())
		return
	}

	c.JSON(http.StatusCreated, "")
}

//-----------------JOBS----------------------//

func ListJobs(c *gin.Context) {

	user := fetchUserFromContext(c)

	var jobs []*lq.ContainerJob
	jobs, err := db.Jobs().GetAllJobsByUser(user.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, err.Error())
	}
	c.JSON(http.StatusOK, &jobs)
}

func GetJob(c *gin.Context) {
	user := fetchUserFromContext(c)
	jobID := c.Param("jobid")
	jid, err := strconv.Atoi(jobID)

	ctJob, err := db.Jobs().Get(uint(jid))

	if err != nil {
		c.JSON(http.StatusNotFound, nil)
	}

	if ctJob.OwnerID != user.ID {
		c.JSON(http.StatusNotFound, nil)
	}

	//TODO: Sanitize fields that should not be public
	c.JSON(http.StatusOK, &ctJob)
}

func CreateJob(c *gin.Context) {
	var err error
	user := fetchUserFromContext(c)
	job := ContainerJobPublic{}
	if err = c.BindJSON(&job); err != nil {
		log.Error(err)
		c.JSON(http.StatusBadRequest, err.Error())
		return
	}

	// Ensure the users account info is correctly Linked
	if awsAccount,err := db.AwsAccounts().Get(user.AwsAccountID); err != nil{
		log.Error(fmt.Sprintf("Coudld not fetch aws for %s , err : %s",user.ID,err))
		c.JSON(http.StatusBadRequest, "Unable to verify users AWS Account Link")
		return
	}else{
		if (awsAccount.GetAwsSecurityGroupIdUsEast1() == "" || awsAccount.GetAwsSecurityGroupIdUsWest1() == "" || awsAccount.GetAwsSshPrivateKeyUsWest2() == "" ){
			c.JSON(http.StatusBadRequest, "Unable to verify users AWS Account Setup , please re-calibrate")
			return
		}

		//TODO : Validate this fact :
		//Atleast 4 subnets to being using
	}

	// Convert ContainerJobPublic into ContainerJob
	portMappingByteString := []byte("[]") // default to empty array
	if job.PortMappings != nil {
		portMappingByteString, err = json.Marshal(job.PortMappings)
		if err != nil {
			log.Error(err)
			c.JSON(http.StatusBadRequest, err.Error())
			return
		}
	}

	// Validate the environment mappings
	for _, envVar := range job.Environment {
		if envVar.Variable == "" {
			msg := "Environment variables cannot have empty keys"
			log.Error(msg)
			c.JSON(http.StatusBadRequest, msg)
			return
		}
		if envVar.Value == "" {
			msg := "Environment variables cannot have empty values"
			log.Error(msg)
			c.JSON(http.StatusBadRequest, msg)
			return
		}
	}

	environmentByteString := []byte("[]") // default to empty array
	if job.Environment != nil {
		environmentByteString, err = json.Marshal(job.Environment)
		if err != nil {
			log.Error(err)
			c.JSON(http.StatusBadRequest, err.Error())
			return
		}
	}

	ctjob := lq.ContainerJob{
		Name:           job.Name,
		Command:        job.Command,
		SourceImage:    job.SourceImage,
		Ram:            job.Ram,
		Cpu:            job.Cpu,
		Gpu:            job.Gpu,
		PortMappings:   string(portMappingByteString),
		Environment:    string(environmentByteString),
		Status:         mesos.TaskState_TASK_STAGING.Enum().String(),
		OwnerID:        user.ID,
		InstanceID:     0,
		UserTerminated: false,
	}

	// Validate that a possible instance can fit this job
	possibleInstances := lqCloud.FindPossibleInstances(job.Cpu, float64(job.Ram), float64(job.Gpu), 0.0)
	if len(possibleInstances) == 0 || job.Cpu == 0 || job.Ram == 0{
		msg := "Job cpu/mem/gpu requirements invalid for any AWS instance"
		log.Error(msg)
		c.JSON(http.StatusBadRequest, msg)
		return
	}

	// Infer source type from image name
	if strings.Contains(ctjob.SourceImage, "https://github.com") {
		ctjob.SourceType = "code"
	} else {
		ctjob.SourceType = "image"
	}

	if err := db.Jobs().Create(&ctjob); err != nil {
		log.Error(lq.NewErrorf(err, "Failed creating job due to internal server error"))
		c.JSON(http.StatusInternalServerError, "Failed creating job due to internal server error")
	}

	c.JSON(http.StatusCreated, ctjob.ID)
}

func DeleteJob(context *gin.Context) {
	user := fetchUserFromContext(context)

	jobId, err := strconv.Atoi(context.Param("jobid"))
	if err != nil {
		context.JSON(http.StatusNotFound, err)
	}

	job, err := db.Jobs().Get(uint(jobId))
	if err != nil {
		context.JSON(http.StatusNotFound, err)
	}

	// Verify that the job is owned by this user
	if job.OwnerID != user.ID {
		context.JSON(http.StatusUnauthorized, fmt.Sprintf("User %s does not own the job %d", user.Email, jobId))
		return
	}

	err = db.Jobs().MarkUserTerminated(uint(jobId))
	if err != nil {
		context.JSON(http.StatusInternalServerError, fmt.Sprintf("Failed marking job %d for termination", jobId))
	} else {
		context.JSON(http.StatusOK, jobId)
	}
}

//----------------- INSTANCES ----------------------//

func ListInstances(c *gin.Context) {
	user := fetchUserFromContext(c)
	instances, err := db.Resources().GetUsersResources(user.ID)

	if err != nil {
		c.JSON(http.StatusNotFound, err.Error())
	}

	c.JSON(http.StatusOK, &instances)
}

func GetInstance(c *gin.Context) {

	user := fetchUserFromContext(c)
	instanceID := c.Param("instanceid")

	iid, err := strconv.Atoi(instanceID)
	if err != nil {
		c.JSON(http.StatusNotFound, err)
	}

	instance, err := db.Resources().Get(uint(iid))
	if err != nil || (instance.OwnerId != user.ID) {
		c.JSON(http.StatusNotFound, "Unable to find instance")
	}

	c.JSON(http.StatusOK, &instance)
}

func DeleteInstance(context *gin.Context) {
	user := fetchUserFromContext(context)

	instanceId, err := strconv.Atoi(context.Param("instanceid"))
	if err != nil {
		context.JSON(http.StatusNotFound, err)
		return
	}

	instance, err := db.Resources().Get(uint(instanceId))
	if err != nil {
		context.JSON(http.StatusNotFound, err.Error())
		return
	}

	// Verify that the instance is owned by this user
	if instance.OwnerId != user.ID {
		msg := fmt.Sprintf("User %s does not own the instance %d", user.Email, instanceId)
		log.Warn(msg)
		context.JSON(http.StatusUnauthorized, msg)
		return
	}

	// Mark resource as user terminated
	if err := db.Resources().MarkUserTerminated(uint(instanceId)); err != nil {
		err = lq.NewErrorf(err, "Failed updating instance %d as being marked for termination", instanceId)
		log.Error(err)
		context.JSON(http.StatusInternalServerError, err.Error())
	} else {
		context.JSON(http.StatusOK, instanceId)
	}
}
