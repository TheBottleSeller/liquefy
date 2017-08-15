package provisioner

import (
	"fmt"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"

	"bargain/liquefy/db"
	lq "bargain/liquefy/models"
	
)

type Cloud string

const (
	AWS  Cloud = "aws"
	VBOX Cloud = "vbox"
	ReconcilliationInterval = time.Duration(1) * time.Minute
	HealthCheckInterval     = time.Duration(1) * time.Minute
)

type Provisioner interface {
	Run() error
}

type provisioner struct {
	mesosMasterIp       string
	resourceManager     ResourceManager
	deprovisioningChan  chan *DeprovisionEvent
	healthCheckers      map[uint]chan struct{}
}

type DeprovisionEvent struct {
	resourceId  uint
	msg         string
}

func NewProvisioner(mesosMasterIp string) Provisioner {
	// Create provisioner
	prov := &provisioner {
		mesosMasterIp:      mesosMasterIp,
		resourceManager:    NewAwsManager(),
		deprovisioningChan: make(chan *DeprovisionEvent, 10 * 1024),
		healthCheckers:     make(map[uint]chan struct{}),
	}

	// TODO Crash recovery
	// Go through jobs and do the following for each state on the db:
	// 'deprovisioning' =>  send them back through the deprovisioning chan

	// "provisioned" => check if the aws instance exists, if not deprovision it
	//
	//                  if it does exist, verify if it is registered with mesos and if so, do it
	//                  if it is registered, set state to "running"

	// "running" =>     verify instance is up and is registered with mesos, otherwise deprovision it

	// "provisioning / bidding / .... states"

	go prov.startResourceChecker()

	go prov.startResourceReconcilliation()

	return prov
}

func (prov *provisioner) Run() error {
	// This picks up resources with status 'new' in the databases and attempts to provision them after setting
	// their status to 'provisioning'. At the end of provisioning, the resource status is 'running'
	// If it fails, it marks the 'new' resource as status deprovisionable
	var provisionerThreadImpl = func() {
		ticker := time.NewTicker(time.Duration(5) * time.Second)
		for _ = range ticker.C {
			newResources, err := db.Resources().GetNewResources()
			if err != nil {
				log.Error(lq.NewErrorf(err, "Failed getting new resources in provisioner"))
				continue
			}

			for _, newResource := range newResources {
				log.Debugf("Provisioning new resource %d", newResource.ID)
				if err := db.Resources().SetStatus(newResource.ID, lq.ResourceProvisioning, ""); err != nil {
					log.Error(lq.NewErrorf(err, "Failed provisioning resource %d", newResource.ID))
					continue
				}
				go func(resource *lq.ResourceInstance) {
					if err := prov.provisionImpl(resource); err != nil {
						log.Error(lq.NewErrorf(err, "Failed provisioning resource %d", resource.ID))
						prov.deprovisioningChan <- &DeprovisionEvent{
							resourceId: resource.ID,
							msg: err.Error(),
						}
					} else {
						log.Debugf("Successfully provisioned resource %d", resource.ID)
					}
				}(newResource)
			}
		}
	}

	// This thread gets resources with status 'deprovisonable' from a deprovisioning channel to prevent races when
	// deprovisioning
	//
	// Racing threads are:
	// - thread that fetches 'deprovisionable' resources and sends them to be deprovisioned
	// - thread that checks health of resource and if is unhealthy, sends for deprovisioning
	//
	// If the status of the resource is 'deprovisioning' or 'deprovisioned', then the resource is skipped
	// otherwise, the status is set to 'deprovisioning', and asynchronously deprovisions them
	var deprovisionerThreadImpl = func() {
		for event := range prov.deprovisioningChan {
			log.Debugf("Deprovisioning resource %d because\n%s", event.resourceId, event.msg)
			resource, err := db.Resources().Get(event.resourceId)
			if err != nil {
				log.Errorf("Failed fetching resource %d during deprovisioning", event.resourceId)
				continue
			}

			// Skip if is being deprovisioned or has already been deprovisioned
			if resource.Status == lq.ResourceStatusDeprovisioning || resource.Status == lq.ResourceStatusDeprovisioned {
				log.Debugf("Skipping already deprovisioning/ed resource %d", resource.ID)
				continue
			}

			// Set status to prevent races with other attempts to deprovision this resource
			if err := db.Resources().SetStatus(resource.ID, lq.ResourceStatusDeprovisioning, event.msg); err != nil {
				log.Error(lq.NewErrorf(err, "Failed deprovisioning resource %d", resource.ID))
				continue
			}

			// Deprovision asynchronously
			// This is the only way that Provisioner::deprovision can be called!!!!
			go func(resourceId uint) {
				prov.deprovision(resourceId)
			}(event.resourceId)
		}
	}

	// START THE PROVISIONER AND DEPROVISIONER THREADS

	// If any thread exits then something has failed
	var wg sync.WaitGroup
	var err error
	wg.Add(1)

	go func() {
		provisionerThreadImpl()
		err = fmt.Errorf("Provisioner thread failed!!!!!")
		wg.Done()
	}()

	go func() {
		deprovisionerThreadImpl()
		err = fmt.Errorf("Deprovisioner thread failed!!!!!")
		wg.Done()
	}()

	wg.Wait()
	return err
}

/*
 * Provision the resource
 * 1. Spin up resource use cloud resource manager
 * 2. Start resource checkers
 * 3. Persist resource in db
 * 4. Setup mesos on resource
 */
func (prov *provisioner) provisionImpl(resource *lq.ResourceInstance) error {

	if err := prov.resourceManager.ProvisionResource(resource); err != nil {
		return err
	}

	if err := db.Resources().SetStatus(resource.ID, lq.ResourceStatusProvisioned, ""); err != nil {
		return err
	}

	log.Infof("Initialize mesos on resource %d", resource.ID)
	if err := prov.resourceManager.SetupMesos(resource, prov.mesosMasterIp); err != nil {
		return lq.NewErrorf(err, "Failed setuping up mesos on resource %d", resource.ID)
	}

	if err := db.Resources().SetStatus(resource.ID, lq.ResourceStatusRunning, ""); err != nil {
		return err
	}

	return nil
}

func (prov *provisioner) deprovision(resourceId uint) {
	// TODO: Add locking around this to make sure that a resource is only being deprovisioned by a single thread
	// It is fine if the same resource gets deprovisioned many times, just not at once
	log.Infof("Deprovisioning resource %d", resourceId)

	resource, err := db.Resources().Get(resourceId)
	if err != nil {
		log.Errorf("Failed to deprovision resource %d because of failure to query from db", resourceId)
	}

	prov.resourceManager.DeprovisionResource(resource)

	if err = db.Resources().SetStatus(resource.ID, lq.ResourceStatusDeprovisioned, ""); err != nil {
		log.Error(err)
	}
}

func (prov *provisioner) startResourceChecker() {
	clock := time.NewTicker(HealthCheckInterval)
	for range clock.C {
		//Perform Check for all resources
		if activeResources,err := db.Resources().GetAllProvisionedOrRunningResources(); err != nil {
			log.Error("Unable To Run Health Checker : " + err.Error())
		} else {
			for _,resource := range activeResources{
				if err := prov.resourceManager.CheckHealth(resource.ID); err != nil {
					err = lq.NewErrorf(err, "Health check failed")
					prov.deprovisioningChan <- &DeprovisionEvent{
						resourceId: resource.ID,
						msg: err.Error(),
					}
				}

				if resource.Status == lq.ResourceStatusRunning {
					// Check if resource has running jobs and if not, kill it
					if jobs, err := db.Jobs().GetActiveJobsOnResource(resource.ID); err != nil {
						log.Error(lq.NewErrorf(err, "Failed getting active jobs on resource %d", resource.ID))
					} else if len(jobs) == 0 {
						prov.deprovisioningChan <- &DeprovisionEvent{
							resourceId: resource.ID,
							msg: "No jobs running on resource",
						}
					}
				}
			}
		}
	}
}

func (prov *provisioner) startResourceReconcilliation() {
	clock := time.NewTicker(ReconcilliationInterval)
	for range clock.C {
		// Get all known resources in the database
		if users, err := db.Users().GetAll(); err != nil {
			log.Error(lq.NewErrorf(err, "Failed getting all users for resource reconcilliation"))
		} else {
			for _, user := range users {
				knownResources, err := db.Resources().GetUsersProvisionedResources(user.ID)
				if err != nil {
					continue
				}

				badResources, err := prov.resourceManager.ReconcileResources(user.ID, knownResources)
				if err != nil {
					log.Error(err)
				}

				for _, resource := range badResources {
					prov.deprovisioningChan <- &DeprovisionEvent{
						resourceId: resource.ID,
						msg: fmt.Sprintf("Resource %d failed resource reconcilliation. The AWS instance cannot be found",
							resource.ID),
					}
				}
			}
		}

		// Get all resources marked for termination by the user
		if userTerminatedResources, err := db.Resources().GetRunningUserTerminatedResources(); err != nil {
			log.Error(lq.NewErrorf(err, "Failed getting user terminated resources"))
		} else {
			for _, resource := range userTerminatedResources {
				prov.deprovisioningChan <- &DeprovisionEvent{
					resourceId: resource.ID,
					msg: fmt.Sprintf("Resource %d terminated by user", resource.ID),
				}
			}
		}
	}
}