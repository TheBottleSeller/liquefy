package models

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"fmt"
	"testing"
)

func TestCommon(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Common Suite")
}

var _ = Describe("Common", func() {
	var user *User
	user = &User{
		Firstname: "lil",
		Lastname:  "nig",
		Email:     "lil.nig@willfuckyouup.com",
	}

	var containerJob *ContainerJob
	containerJob = &ContainerJob{
		SourceImage: "foo/bar",
		Environment: []EnvVar{{Variable: "y", Value: "2"}},
		Ram:         5,
		Cpu:         1,
	}

	var userJob *UserJob
	userJob = &UserJob{
		Name:          "foo",
		Environment:   []EnvVar{{Variable: "x", Value: "1"}},
		ContainerJobs: []ContainerJob{},
		Ram:           5,
		Cpu:           1,
		FirstTry:      true,
	}

	var instance *ResourceInstance
	instance = &ResourceInstance{
		Type:          "some",
		RamTotal:      20,
		RamUsed:       0,
		CpuTotal:      32,
		CpuUsed:       0,
		LaunchTime:    1000,
		BidMaxPrice:   "1",
		OwnerId:       user.ID,
		Status:        "status",
		ContainerJobs: []ContainerJob{},
	}

	var tempUserJob UserJob
	var tempJob ContainerJob
	var tempInstance ResourceInstance

	Context("Simple Create Tests", func() {

		It("Persist Simple User Object", func() {
			query := GetDB().Create(user)
			fmt.Println(query.Error)
			Expect(query.Error).To(BeNil())
		})

		It("Persist User Job", func() {
			userJob.OwnerID = user.ID
			query := GetDB().Create(userJob)
			Expect(query.Error).To(BeNil())
		})

		It("Persis Container Job", func() {
			containerJob.UserJobID = userJob.ID
			containerJob.InstanceID = 0
			query := GetDB().Create(containerJob)
			Expect(query.Error).To(BeNil())

		})

		It("Persist Instance", func() {
			instance.OwnerId = user.ID
			query := GetDB().Create(instance)
			Expect(query.Error).To(BeNil())

			GetDB().First(&tempInstance)
			Expect(tempInstance.CpuUsed).To(Equal(0))
			Expect(tempInstance.RamUsed).To(Equal(0))

		})

		It("Instance has no container jobs", func() {
			Expect(len(tempInstance.ContainerJobs)).To(Equal(0))
		})

		It("UserJob has 1 container Job", func() {
			GetDB().First(&tempUserJob)
			GetDB().First(&tempJob)

			ctjobs, err := tempUserJob.GetContainerJobs()
			Expect(err).To(BeNil())
			Expect(len((ctjobs))).To(Equal(1))
			Expect(ctjobs[0].ID).To(Equal(tempJob.ID))
		})

		It("Container job has no instance", func() {
			Expect(int(tempJob.InstanceID)).To(Equal(0))
		})

		It("Verify list resources works correctly", func() {
			resources, err := user.ListInstances()

			Expect(err).To(BeNil())
			Expect(len(resources)).To(Equal(1))
			Expect(resources[0].ID).To(Equal(instance.ID))

		})
	})

	Context("Job Assing to instance ", func() {

		It("Verify assigning reduces cpu, ram, and makes associations", func() {
			err := AssignInstanceTx(containerJob, instance)
			Expect(err).To(BeNil())

			GetDB().First(&tempInstance, tempInstance.ID)
			GetDB().First(&tempJob, tempJob.ID)

			containerJobs, err := tempInstance.ListContainerJobs()
			Expect(err).To(BeNil())

			//BREAK TO IT
			Expect(tempInstance.CpuUsed).To(Equal(tempJob.Cpu))
			Expect(tempInstance.RamUsed).To(Equal(tempJob.Ram))

			//BREAK TO IT
			Expect(len(containerJobs)).To(Equal(1))
			Expect(containerJobs[0].ID).To(Equal(tempJob.ID))

			//BREAK TO IT
			Expect(tempInstance.ID).To(Equal(tempJob.InstanceID))
		})

	})

	Context("Job UnAssign from instance ", func() {

		It("Verify unassign adds  cpu, ram, and makes unassociation", func() {
			err := UnassignInstanceTx(containerJob, instance)
			Expect(err).To(BeNil())

			GetDB().First(&tempInstance)
			GetDB().First(&tempJob)

			containerJobs, err := tempInstance.ListContainerJobs()
			Expect(err).To(BeNil())

			//BREAK TO IT
			Expect(tempInstance.CpuUsed).To(Equal(0))
			Expect(tempInstance.RamUsed).To(Equal(0))

			//BREAK TO IT
			Expect(len(containerJobs)).To(Equal(0))
			Expect(tempJob.InstanceID).To(Equal(uint(0)))
		})
	})
})
