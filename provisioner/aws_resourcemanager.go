package provisioner

import (
	"fmt"
	"time"
	"bytes"
	"strings"
	"golang.org/x/crypto/ssh"

	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/service/ec2"

	"bargain/liquefy/db"
	lq "bargain/liquefy/models"
	aws "bargain/liquefy/cloudprovider"
	"strconv"
)

const (
	AwsTag                    = "Liquefy"
	AwsUser                   = "ubuntu"
	DelayTillReconcillitation = time.Duration(10) * time.Minute
)

type ResourceManager interface {
	ProvisionResource(resource *lq.ResourceInstance) error
	SetupMesos(resource *lq.ResourceInstance, masterIp string) error
	DeprovisionResource(resource *lq.ResourceInstance) error
	CheckHealth(resourceId uint) error
	ReconcileResources(userId uint, knownResources []*lq.ResourceInstance) ([]*lq.ResourceInstance, error)
}

type awsManager struct {}

func NewAwsManager() (ResourceManager) {
	return awsManager{}
}

func (manager awsManager) getAwsAccount(userId uint) (*lq.AwsAccount, error) {
	user, err := db.Users().Get(userId)
	if err != nil {
		log.Errorf("Failed getting user %d when fetching aws account", userId)
		return &lq.AwsAccount{}, err
	}
	return db.AwsAccounts().Get(user.AwsAccountID)
}

func (manager awsManager) ProvisionResource(resource *lq.ResourceInstance) error {
	awsAccount, err := manager.getAwsAccount(resource.OwnerId)
	if err != nil {
		return lq.NewError("Failed to provision resource ", err)
	}

	az := resource.AwsAvailabilityZone
	region := aws.Region(lq.AZtoRegion(az))
	awsCloud := aws.NewAwsCloud(awsAccount.AwsAccessKey, awsAccount.AwsSecretKey)

	if err = db.Resources().SetStatus(resource.ID, lq.ResourceSpotBidding, ""); err != nil {
		return err
	}

	log.Infof("Provisioning resource %d via AWS API", resource.ID)
	spotReq, err := awsCloud.CreateSpotInstanceRequest(region, az,
		manager.getImageId(region.String(), resource.AwsInstanceType),
		awsAccount.GetSubnetId(az), awsAccount.GetSecurityGroupId(region.String()),
		resource.AwsInstanceType, resource.AwsSpotPrice, resource.ID)
	if err != nil {
		return lq.NewError("Creating spot instance request failed ", err)
	}

	log.Debug("Waiting for spot request to complete :: ")
	log.Debug(spotReq)

	spotReq, err = awsCloud.WaitForSpotRequestToFinish(spotReq)
	if err != nil {
		return lq.NewErrorf(err, "Spot request failed")
	}

	log.Debugf("Tagging resource %d", resource.ID)
	if err = awsCloud.TagInstance(region, *spotReq.InstanceId, AwsTag); err != nil {
		return lq.NewError(fmt.Sprintf("Failed tagging resource %d ", resource.ID), err)
	}

	if err = db.Resources().SetStatus(resource.ID, lq.ResourceSpotBidAccepted, ""); err != nil {
		return err
	}

	var instance *ec2.Instance
	if instance, err = awsCloud.GetInstance(region, *spotReq.InstanceId); err != nil {
		return lq.NewError("Provisioner : Failed to get instance : "+*spotReq.InstanceId, err)
	}

	//At this point have an instance with its info
	resource.LaunchTime = instance.LaunchTime.UnixNano()
	if err = db.Resources().SetLaunchTime(resource.ID, resource.LaunchTime); err != nil {
		return lq.NewError(fmt.Sprintf("Provisioner : Failed setting launch time for resource %d", resource.ID), err)
	}

	resource.AwsInstanceId = *instance.InstanceId
	err = db.Resources().SetInstanceId(resource.ID, resource.AwsInstanceId)
	if err != nil {
		return lq.NewError(fmt.Sprintf("Provisioner : Failed setting aws instance ID for %d", resource.ID), err)
	}

	instance, err = awsCloud.WaitForInstanceRunning(region, instance)
	if err != nil {
		return lq.NewError(fmt.Sprintf("Provisioner : Failed waiting for resource %d to be running ", resource.ID), err)
	}

	// Complete Instance Networking
	instance, err = awsCloud.WaitForIpAllocation(region, instance)
	if err != nil {
		return lq.NewError(fmt.Sprintf("Provisioner : Failed waiting for resource %d to be allocated ip ", resource.ID), err)
	}

	resource.IP = *instance.PublicIpAddress
	if err = db.Resources().SetIP(resource.ID, resource.IP); err != nil {
		return lq.NewError(fmt.Sprintf("Provisioner : Failed setting public ip for resource: %d", resource.ID), err)
	}

	log.Debugf("Successfully provisioned instance:\n%v", instance)
	return nil
}

func (manager awsManager) SetupMesos(resource *lq.ResourceInstance, masterIp string) error {
	log.Infof("Setting up mesos on resource %d", resource.ID)
	// Get SSH private key to use
	awsAccount, err := manager.getAwsAccount(resource.OwnerId)
	if err != nil {
		return lq.NewErrorf(err, "Failed to setup mesos on resource %d", resource.ID)
	}

	region := aws.AzToRegion(resource.AwsAvailabilityZone)
	keyString := awsAccount.GetSshPrivateKey(region)
	privateKey, err := ssh.ParsePrivateKey([]byte(keyString))
	if err != nil {
		return lq.NewErrorf(err, "Failed to parse ssh key to reach resource %d in region %s",
			resource.ID, region)
	}

	// Setup ssh session
	config := &ssh.ClientConfig{
		User: AwsUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(privateKey),
		},
	}

	// Create a new ssh connection
	// The ssh daemon may not be up, and so will attempt connecting for a max of 5 minutes
	publicIp := resource.IP
	sshIp := fmt.Sprintf("%s:22", publicIp)
	var client *ssh.Client
	timeoutTimer := time.NewTicker(time.Duration(7) * time.Minute)

	connected := false
	for ! connected {
		select {
		case _ = <-timeoutTimer.C:
			return lq.NewErrorf(err, "Timed out waiting for ssh daemon to start")

		default:
			log.Debugf("Attempting to start ssh connection for resource %d at %s", resource.ID, sshIp)
			time.Sleep(time.Duration(1) * time.Second)

		// ssh.Dial will block for 1 minute, so do not need to sleep when retrying
			client, err = ssh.Dial("tcp", sshIp, config)
			if err == nil {
				log.Debugf("Successfully connected")
				connected = true
				break
			}
		}
	}

	// Run the command over ssh, which requires opening a new ssh session.
	// The output is returned
	runCommandOverSshImpl := func (cmd string) (string, error) {
		var output bytes.Buffer
		session, err := client.NewSession()
		if err != nil {
			return "", lq.NewErrorf(err, "Failed starting ssh session")
		}
		defer session.Close()

		session.Stdout = &output
		err = session.Run(cmd)
		outputString := strings.TrimSpace(output.String())

		if err != nil {
			log.Error(outputString)
			return outputString, lq.NewErrorf(err, "Failed executing command over ssh:\n%s", cmd)
		}
		return outputString, nil
	}

	// This will rety the command, retry times
	retryCount := 5
	runCommandOverSsh := func(cmd string) (output string, err error) {
		for i := 0; i < retryCount; i++ {
			// If ssh execution is successful, then return
			if output, err = runCommandOverSshImpl(cmd); err == nil {
				return
			}
		}
		return
	}

	log.Debugf("Getting private ip of resource %d", resource.ID)
	cmdGetPrivateIp := "ifconfig eth0 | grep 'inet addr:' | cut -d: -f2 | awk '{ print $1}'"
	privateIp, err := runCommandOverSsh(cmdGetPrivateIp)
	if err != nil {
		return lq.NewErrorf(err, "Failed running command on resource %d:\n%s", resource.ID, cmdGetPrivateIp)
	}
	log.Infof("Recieved private ip %s for resource %d", privateIp, resource.ID)

	// These commands for setting the hostname were taken directly from what docker-machine runs on the host
	hostname := fmt.Sprintf("liquefy-slave-%d", resource.ID)
	cmdSetHostname := fmt.Sprintf("sudo hostname %s && echo \"%s\" | sudo tee /etc/hostname", hostname, hostname)
	cmdUpdateEtcHosts := fmt.Sprintf("if grep -xq 127.0.1.1.* /etc/hosts; " +
		"then sudo sed -i 's/^127.0.1.1.*/127.0.1.1 %s/g' /etc/hosts; " +
		"else echo '127.0.1.1 %s' | sudo tee -a /etc/hosts; fi",
		hostname, hostname)
	log.Debugf("Setting resource hostname to %s", hostname)
	output, err := runCommandOverSsh(cmdSetHostname)
	if err != nil {
		log.Error(lq.NewErrorf(err, "Failed setting hostname %d:\n%s", resource.ID, cmdSetHostname))
		return lq.NewErrorf(nil, "Failed setting hostname %d:\n%s", resource.ID, cmdSetHostname)
	}
	output, err = runCommandOverSsh(cmdUpdateEtcHosts)
	if err != nil {
		log.Error(lq.NewErrorf(err, "Failed updating /etc/hosts for resource %d:\n%s", resource.ID, cmdUpdateEtcHosts))
		return lq.NewErrorf(nil, "Failed updating /etc/hosts for resource %d:\n%s", resource.ID, cmdUpdateEtcHosts)
	}
	log.Debugf("Set hostname to %s", hostname)

	// Setup command that will create mesos-slave container
	mesosAttributes := fmt.Sprintf("liquefyid:%d", resource.ID)
	mesosResources := fmt.Sprintf("cpus:%f;mem:%d", resource.CpuTotal, resource.RamTotal)
	cmdStartMesosSlave := []string {
		"docker run -d",
		"--name=mesos-slave",
		"--net=host",
		"--privileged",
		"-e RESOURCE_ID=" + strconv.Itoa(int(resource.ID)),
		"-e MESOS_LOG_DIR=/var/log",
		"-e MESOS_WORK_DIR=/var/lib/mesos/slave",
		"-e MESOS_MASTER=zk://" + masterIp + ":2181/mesos",
		"-e MESOS_ISOLATOR=cgroups/cpu,cgroups/mem",
		"-e MESOS_CONTAINERIZERS=mesos",
		"-e MESOS_DOCKER_MESOS_IMAGE=mesosphere/mesos-slave:0.25.0-0.2.70.ubuntu1404",
		"-e MESOS_PORT=5051",
		"-e LIBPROCESS_ADVERTISE_IP=" + publicIp,
		"-e MESOS_IP=" + privateIp,
		"-e MESOS_HOSTNAME=" + publicIp,
		"-e MESOS_SWITCH_USER=false",
		"-e MESOS_EXECUTOR_REGISTRATION_TIMEOUT=5mins",
		"-e MESOS_ATTRIBUTES=\"" + mesosAttributes + "\"", // note the use of escaped quotes
		"-e MESOS_RESOURCES=\"" + mesosResources + "\"", // note the use escaped quotes
		"-v /lib/libpthread.so.0:/lib/libpthread.so.0:ro",
		"-v /lib/x86_64-linux-gnu:/lib/x86_64-linux-gnu:ro",
		"-v /lib/usr/x86_64-linux-gnu:/lib/usr/x86_64-linux-gnu:ro",
		"-v /usr/bin/docker:/usr/bin/docker:ro",
		"-v /var/run/docker.sock:/var/run/docker.sock:ro",
		"-v /sys:/sys:ro",
		"-v /var/lib/mesos:/var/lib/mesos",
		"-p 5051:5051",
		"mesosphere/mesos-slave:0.25.0-0.2.70.ubuntu1404",
	}

	// Setup command that will create liquefy/logger container
	cmdStartLogger := []string{
		"docker run -d",
		"--name=logger",
		"--net=host",
		"-e ELASTIC_SEARCH_IP=" + masterIp,
		"-v /usr/local/bin/docker:/usr/bin/docker:ro",
		"-v /var/run/docker.sock:/var/run/docker.sock:ro",
		"-v /var/lib/docker/containers:/var/lib/docker/containers",
		"-v /var/lib/mesos:/var/lib/mesos",
		"liquefy/logger:latest",
	}

	// Run the ssh commands
	log.Debugf("Starting mesos slave on resource %d", resource.ID)
	cmd := strings.Join(cmdStartMesosSlave, " ")
	output, err = runCommandOverSsh(cmd)
	if err != nil {
		log.Error(err)
		return lq.NewErrorf(nil, "Failed to start mesos slave on resource %d", resource.ID)
	}
	log.Debugf("Output from starting mesos slave:\n%s", output)

	log.Debugf("Starting liquefy/logger on resource %d", resource.ID)
	cmd = strings.Join(cmdStartLogger, " ")
	output, err = runCommandOverSsh(cmd)
	if err != nil {
		log.Error(err)
		return lq.NewErrorf(nil, "Failed to start liquefy logger on resource %d", resource.ID)
	}
	log.Debugf("Output from starting liquefy logger:\n%s", output)

	return nil
}

func (manager awsManager) DeprovisionResource(resource *lq.ResourceInstance) error {
	log.Debugf("Deprovisioning resource %v", resource)

	// Try to verify that status of instance is shutting down or terminated
	awsAccount, err := manager.getAwsAccount(resource.OwnerId)
	if err != nil {
		return lq.NewError("Failed to Deprovision resource, cannot get aws creds ", err)
	}

	// If there is no aws instance for this resource, then we are done
	if resource.AwsInstanceId == "" {
		return nil
	}

	// Otherwise, terminate the instance and cancel the spot request
	awsCloud := aws.NewAwsCloud(awsAccount.AwsAccessKey, awsAccount.AwsSecretKey)
	var instance *ec2.Instance
	region := aws.Region(lq.AZtoRegion(resource.AwsAvailabilityZone))

	log.Debugf("Terminating instance %s", resource.AwsInstanceId)
	err = awsCloud.TerminateInstance(region, resource.AwsInstanceId)
	if err != nil {
		// We should try to cancel the spot request anyway
		awsCloud.CancelSpotInstanceRequest(region, resource.AwsInstanceId)
		return lq.NewError(fmt.Sprintf("Error trying to terminate instance %s for resource %d", resource.AwsInstanceId), err)
	}

	log.Debugf("Cancelling spot request for instance %s", resource.AwsInstanceId)
	err = awsCloud.CancelSpotInstanceRequest(region, resource.AwsInstanceId)
	if err != nil {
		return lq.NewError(fmt.Sprintf("Error trying to cancel spot request for instance %s for resource %d",
			resource.AwsInstanceId, resource.ID), err)
	}

	instance, err = awsCloud.GetInstance(region, resource.AwsInstanceId)
	if err != nil {
		return err
	}

	if *instance.State.Name != ec2.InstanceStateNameShuttingDown &&
		*instance.State.Name != ec2.InstanceStateNameTerminated {
		return lq.NewError("Not confirmed that instance is deprovisioned with AWS", nil)
	}

	return nil
}

func (manager awsManager) CheckHealth(resourceId uint) error {
	resource, err := db.Resources().Get(resourceId)
	if err != nil {
		// log this error, but do not consider unhealthy because this is an internal error
		log.Error(err)
		return nil
	}

	awsAccount, err := manager.getAwsAccount(resource.OwnerId)
	if err != nil {
		// log this error, but do not consider unhealthy because this is an internal error
		log.Error(err)
		return nil
	}

	region := aws.Region(lq.AZtoRegion(resource.AwsAvailabilityZone))
	awsCloud := aws.NewAwsCloud(awsAccount.AwsAccessKey, awsAccount.AwsSecretKey)

	// Check if instance is running
	instance, err := awsCloud.GetInstance(region, resource.AwsInstanceId)
	if err != nil {
		err = lq.NewErrorf(err, "Failed getting instance %s for resource %d", resource.AwsInstanceId, resource.ID)
		log.Error(err)
		// If we hit a RequestLimitExceeded, dont return an error for the health check
		if strings.Contains(err.Error(), "RequestLimitExceeded") {
			return nil
		}
		return err
	}

	if *instance.State.Name != "running" {
		err := lq.NewErrorf(nil, "Aws Instance %s for resource %d is not running and is in state %s.\n%s",
			resource.AwsInstanceId, resource.ID, *instance.State.Name, *instance.StateReason.Message)
		log.Error(err)
		return err
	}

	// TODO This should verify that mesos is running and that it has been registered with the mesos master

	// Check if spot request is marked for termination
	spotReq, err := awsCloud.GetSpotRequestByInstanceId(region, resource.AwsInstanceId)
	if err != nil {
		err = lq.NewErrorf(err, "Failed getting spot request for aws instance %s for resource %d",
			resource.AwsInstanceId, resource.ID)
		log.Error(err)
		// If we hit a RequestLimitExceeded, dont return an error for the health check
		if strings.Contains(err.Error(), "RequestLimitExceeded") {
			return nil
		}
		return err
	}

	if *spotReq.Status.Code == "marked-for-termination" {
		err := lq.NewErrorf(nil, "Aws Instance %s for resource %d is marked for termination",
			resource.AwsInstanceId, resource.ID)
		log.Error(err)
		return err
	}

	return nil
}

func (manager awsManager) ReconcileResources(userId uint, knownResources []*lq.ResourceInstance) ([]*lq.ResourceInstance, error) {
	//All known resources must have same user id
	badResources := []*lq.ResourceInstance{}
	awsAccount, err := manager.getAwsAccount(userId)
	if err != nil {
		return badResources, err
	}

	awsCloud := aws.NewAwsCloud(awsAccount.AwsAccessKey, awsAccount.AwsSecretKey)

	// TODO : This much also catch instances that we started and failed to TAG !
	// Reconcile across each supported region
	for region := range aws.AWSRegionsToAZs {
		awsInstances, err := awsCloud.GetAllActiveTaggedInstances(region, AwsTag)
		if err != nil {
			continue
		}

		activeTaggedInstanceIds := make([]string, len(awsInstances))
		idToAwsInstance := make(map[string]*ec2.Instance)
		for i, awsInstance := range awsInstances {
			idToAwsInstance[*awsInstance.InstanceId] = awsInstance
			activeTaggedInstanceIds[i] = *awsInstance.InstanceId
		}

		//log.Debugf("Active instances for user %d in %s: %v", userId, region, activeTaggedInstanceIds)

		// remove known resources from map, everything remaining must be deprovisioned
		for _, knownResource := range knownResources {
			// Skip checking resources that are not in this region
			if lq.AZtoRegion(knownResource.AwsAvailabilityZone) != region.String() {
				continue
			}
			awsInstance := idToAwsInstance[knownResource.AwsInstanceId]
			if awsInstance == nil { // resource obj does not have an instance
				log.Debugf("Resource %d does not have AWS instance with id %s", knownResource.ID,
					knownResource.AwsInstanceId)
				badResources = append(badResources, knownResource)
			} else { // resource obj has an instance
				delete(idToAwsInstance, knownResource.AwsInstanceId)
			}
		}

		// delete ids that remain (i.e. unknown instances)
		for id, instance := range idToAwsInstance {
			// Only delete if it is older than the delay
			if time.Since(*instance.LaunchTime) > DelayTillReconcillitation {
				log.Debugf("Terminate unknown AWS instance %s", id)
				if err := awsCloud.TerminateInstance(region, id); err != nil {
					log.Error(err)
				}
			} else {
				log.Debugf("Instance %s is unknown, but is younger than %s. Keeping", id, DelayTillReconcillitation)
			}
		}
	}

	return badResources, nil
}

/* Helpers */

//TODO : If needed it should be moved to aws.go / read this from a tbh a .ini file in aws.go
func (manager awsManager) getImageId(region string, instanceType string) string {
	supportsHvm := true
	supportsEbsBacked := true

	// From Amazon Docs
	// http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/virtualization_types.html
	// "All current generation instance types support HVM AMIs.
	// The CC2, CR1, HI1, and HS1 previous generation instance types support HVM AMIs."
	if strings.HasPrefix(instanceType, "t1") ||
		strings.HasPrefix(instanceType, "m2") ||
		strings.HasPrefix(instanceType, "m1") ||
		strings.HasPrefix(instanceType, "c1") {
		supportsHvm = false
	}

	if strings.HasPrefix(instanceType, "t2") {
		supportsEbsBacked = false
	}

	// Filling in the remaining regions involves checking out:
	// https://cloud-images.ubuntu.com/releases/14.04/release-20150305/
	if !supportsHvm {
		switch region {
		case "us-east-1":
			return "ami-988ad1f0"
		case "us-west-1":
			return "ami-397d997d"
		case "us-west-2":
			return "ami-cb1536fb"

		default:
			log.Errorf("Region %s not supported with instance type %s", region, instanceType)
		}
	}

	if supportsEbsBacked {
		// These amis were created by us
		switch region {
		case "us-east-1":
			return "ami-07f4c96d"
		case "us-west-1":
			return "ami-ea76068a"
		case "us-west-2":
			return "ami-2fbd504f"

		default:
			log.Errorf("Region %s not supported with instance type %s", region, instanceType)
		}
	} else {
		switch region {
		case "ap-northeast-1":
			return "ami-93876e93"

		case "ap-southeast-1":
			return "ami-66546234"

		case "eu-central-1":
			return "ami-e2a694ff"

		case "eu-west-1":
			return "ami-d7fd6ea0"

		case "sa-east-1":
			return "ami-a357eebe"

		case "us-east-1":
			return "ami-6089d208"

		case "us-west-1":
			return "ami-cf7d998b"

		case "cn-north-1":
			return "ami-d436a4ed"

		case "us-gov-west-1":
			return "ami-01523322"

		case "ap-southeast-2":
			return "ami-cd4e3ff7"

		case "us-west-2":
			return "ami-3b14370b"

		default:
			log.Errorf("Region %s not supported with instance type %s", region, instanceType)
		}
	}
	return ""
}
