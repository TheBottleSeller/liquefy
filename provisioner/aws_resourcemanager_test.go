package provisioner

import(
	"fmt"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	lq "bargain/liquefy/models"
	"bargain/liquefy/db"
)
func TestAwsSetup(t *testing.T) {
	Convey("Setting up user", t, func() {
		var user = &lq.User{
			Username:  "darthvader2",
			Password:  "darthvader",
			Firstname: "Darth",
			Lastname:  "Vader",
			Email:     "darthvader@thedeathstar.com",
		}

		db.Connect("localhost")
		db.Init()

		err := db.Users().Create(user)
		So(err, ShouldBeNil)

		Convey("Setup Aws Account", func() {
			rm, err := NewAwsManager()
			So(err, ShouldBeNil)

			empty := ""
			lqerr := rm.LinkAwsAccount(user, &empty, &empty)
			So(lqerr.Error(), ShouldBeEmpty)
		})
	})
}

func TestSetupOfMesos(t *testing.T) {
	Convey("Setting up of mesos", t, func() {
		masterIp := "52.90.208.13"
		dbIp := masterIp
		err := db.Connect(dbIp)
		So(err, ShouldBeEmpty)

		manager, err := NewAwsManager()
		So(err, ShouldBeEmpty)

		resource, _ := db.Resources().Get(5)
		err = manager.SetupMesos(resource, masterIp)
		if err != nil {
			fmt.Println(err)
		}
	})
}