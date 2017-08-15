package provisioner
//
//import (
//	"encoding/json"
//	"fmt"
//	"io/ioutil"
//	"net/http"
//	"os"
//	"strings"
//	"testing"
//	"time"
//
//	. "github.com/onsi/ginkgo"
//	. "github.com/onsi/gomega"
//
//	"bargain/liquefy/db"
//	lq "bargain/liquefy/models"
//)
//
//const (
//	DOCKER_MACHINE_MASTER = "vbox-master"
//)
//
//func TestVboxManager(t *testing.T) {
//	RegisterFailHandler(Fail)
//	RunSpecs(t, "Vbox Manager Suite")
//}
//
//var _ = Describe("VboxManager", func() {
//
//	// Setup vbox mesos master
//	executorPath := os.Getenv("GOPATH") + "/src/bargain/liquefy/liquidmesosexecutor/liquidmesosexecutor"
//	safeExec("bash", "../setup/provision_vbox.sh", DOCKER_MACHINE_MASTER)
//	safeExec("bash", "../setup/publish_executor.sh", DOCKER_MACHINE_MASTER, executorPath)
//	safeExec("bash", "../setup/setup_vbox_master.sh", DOCKER_MACHINE_MASTER)
//
//	vboxManager := NewVboxManager()
//	provisioner := NewProvisioner(vboxManager, DOCKER_MACHINE_MASTER)
//	testJob := &lq.ContainerJob{
//		Name:     "yippikayemotherfucker",
//		Ram:      1024,
//		Cpu:      1,
//		FirstTry: true,
//	}
//
//	if err := db.GetDB().Create(testJob).Error; err != nil {
//		panic(err)
//	}
//
//	var resource *lq.ResourceInstance
//	masterIp := "http://" + strings.TrimSpace(safeExec("docker-machine", "ip", DOCKER_MACHINE_MASTER)) + ":5050/master/state.json"
//
//	Context("Simple Provision and Deprovision test", func() {
//		It("Verify master has no slaves", func() {
//			state := queryMesos(masterIp)
//			Expect(state.ActivatedSlaves).To(BeEquivalentTo(0))
//		})
//
//		It("Provision single slave", func() {
//			assignment := provisioner.Provision([]*lq.ContainerJob{testJob})
//			Expect(len(assignment)).To(BeEquivalentTo(1))
//			for r := range assignment {
//				resource = r
//			}
//		})
//
//		It("Verify master a single slave", func() {
//			time.Sleep(time.Duration(10) * time.Second)
//			state := queryMesos(masterIp)
//			Expect(state.ActivatedSlaves).To(BeEquivalentTo(1.0))
//		})
//
//		It("Verify job was assigned to resource", func() {
//			var tempJob lq.ContainerJob
//			db.GetDB().Find(&tempJob, testJob.ID)
//			Expect(tempJob.InstanceID).To(BeEquivalentTo(resource.ID))
//		})
//
//		It("Wait for temination of single slave", func() {
//			var tempResource *lq.ResourceInstance
//			termChan := provisioner.GetResourceTerminationChan()
//			expected := time.Now().Add(VBOX_TIME_TILL_PAY_RENEWAL - VBOX_PAY_RENEWAL_BUFFER)
//			tempResource = <-termChan
//			Expect(tempResource.ID).To(BeEquivalentTo(resource.ID))
//			Expect(time.Now().After(expected)).To(BeTrue())
//		})
//
//		It("Deprovision resource", func() {
//			provisioner.Deprovision(resource)
//		})
//
//		It("Verify job was unassigned from resource", func() {
//			var tempJob lq.ContainerJob
//			db.GetDB().Find(&tempJob, testJob.ID)
//			Expect(tempJob.InstanceID).To(BeEquivalentTo(0))
//		})
//	})
//})
//
//type MesosState struct {
//	ActivatedSlaves float64
//}
//
//func queryMesos(mesosIp string) MesosState {
//	res, err := http.Get(mesosIp)
//
//	if err != nil {
//		panic(err.Error())
//	}
//
//	body, err := ioutil.ReadAll(res.Body)
//
//	if err != nil {
//		panic(err.Error())
//	}
//
//	fmt.Println(string(body))
//	var data MesosState
//	json.Unmarshal(body, &data)
//	fmt.Println("MesosState: %v\n", data)
//	return data
//}
