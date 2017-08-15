package provisioner
//
//import (
//	"fmt"
//	"time"
//
//	log "github.com/Sirupsen/logrus"
//
//	"bargain/liquefy/db"
//	lq "bargain/liquefy/models"
//)
//
//const (
//	VBOX_RAM                   = 498
//	VBOX_CPU                   = 1.0
//	VBOX_BID_PRICE             = 1.00
//	VBOX_TIME_TILL_PAY_RENEWAL = time.Duration(30) * time.Minute
//	VBOX_PAY_RENEWAL_BUFFER    = time.Duration(2) * time.Second
//)
//
//type vboxManager struct {
//	setupDir string
//}
//
//func NewVboxManager(setupDir string) ResourceManager {
//	return vboxManager{setupDir}
//}
//
//func (manager vboxManager) LinkAwsAccount(user *lq.User, key *string, secret *string) error {
//	return nil
//}
//
//func (manager vboxManager) SetupAccountResource(user *lq.User) error {
//	return nil
//}
//
//
//func (manager vboxManager) GetDockerHost(resource *lq.ResourceInstance) string {
//	return fmt.Sprintf("vbox-slave-%d", resource.ID)
//}
//
//func (manager vboxManager) AssignRequestToResource(req Request) (*lq.ResourceInstance, error) {
//	//TODO :: We cannot have this be here , if i change something small about the job struct it will cause all Resource managers to change
//	// right now we naively assign each job its own vbox vm
//	return &lq.ResourceInstance{
//		RamTotal:     VBOX_RAM,
//		CpuTotal:     VBOX_CPU,
//		RamUsed:      0,
//		CpuUsed:      0.0,
//		OwnerId:      req.UserId,
//	}, nil
//}
//
//func (manager vboxManager) ProvisionResource(resource *lq.ResourceInstance) error {
//	safeExec("bash", manager.setupDir + "provision_vbox.sh", manager.GetDockerHost(resource))
//	ip := safeExec("docker-machine", "ip", manager.GetDockerHost(resource))
//	resource.IP = ip
//	query := db.GetDB().Model(&lq.ResourceInstance{}).Where("id = ?", resource.ID).UpdateColumn("ip", resource.IP)
//	return query.Error
//}
//
//
//func (manager vboxManager) SetupMesos(resource *lq.ResourceInstance, masterIp string) error {
//	safeExec("bash", manager.setupDir + "setup_vbox_slave.sh",
//		manager.GetDockerHost(resource), fmt.Sprintf("%d", resource.ID), masterIp)
//	return nil
//}
//
//func (manager vboxManager) DeprovisionResource(resource *lq.ResourceInstance) error {
//	safeExec("docker-machine", "stop", manager.GetDockerHost(resource))
//	return nil
//}
//
//func (manager vboxManager) GetTimeToRenewal(resource *lq.ResourceInstance) time.Duration {
//	return VBOX_TIME_TILL_PAY_RENEWAL - VBOX_PAY_RENEWAL_BUFFER
//}
//
//func (manager vboxManager) IsResourceTerminating(resource *lq.ResourceInstance) bool {
//	return false
//}
//
//func (manager vboxManager) ReconcileResources(userId uint, knownResources []*lq.ResourceInstance) ([]*lq.ResourceInstance, error) {
//	// It is assumed that no resource reconcilliation needs to occur for vbox
//	return []*lq.ResourceInstance{}, nil
//}
//
//func safeExec(name string, arg ...string) string {
//	out, err := executeAndLogStream(name, arg...)
//	if err != nil {
//		log.Error(out)
//		panic(err)
//	}
//	return out
//}
