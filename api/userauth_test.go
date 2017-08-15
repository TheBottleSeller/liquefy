package api

import (
	"bargain/liquefy/db"
	"fmt"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestUserCreation(t *testing.T) {

	db.Connect("localhost")
	db.Init()

	Convey("Setup ApiServer For Test", t, func() {

		api := NewApiServer(fmt.Sprintf("http://%s:3030", "127.0.0.1"))

		/* Create Test User */
		username := "TheDarkLord4"
		password := "LukeIAmYourFather"
		user := &ApiUser{
			Username:  username,
			Password:  password,
			Firstname: "Darth",
			Lastname:  "Vader",
			Email:     "darthvader@thedeathstar.com",
		}

		token := api.CreateUser(user, "")

		Convey("Validating Fetch User", func() {

			fetchedUser := api.GetUser(token)

			So(fetchedUser.Password, ShouldBeEmpty)
			So(fetchedUser.ApiKey, ShouldBeEmpty)

			So(fetchedUser.Firstname, ShouldEqual, user.Firstname)
			So(fetchedUser.Email, ShouldEqual, user.Email)
		})
	})
}
