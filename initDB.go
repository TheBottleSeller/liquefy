package main

import (
	database "bargain/liquefy/db"
	"errors"
	"flag"
	"fmt"
	"time"
	log "github.com/Sirupsen/logrus"
	lq "bargain/liquefy/models"

)

func main() {
	masterIP := flag.String("masterip", "", "IP on which the master is running")
	flag.Parse()

	if masterIP == nil || *masterIP == "" {
		panic(errors.New("No masterip, please provide valid values"))
	}

	err := fmt.Errorf("Error")
	for err != nil {
		err = database.Connect(*masterIP)
		time.Sleep(time.Second)
	}

	fmt.Println("Initializing postgres db")


	Init()
}

func Init() {
	db := database.GetDB()
	//REMOVE THIS ONCE IN PROD
	db.LogMode(true)
	db.DropTable(&lq.ContainerJob{})
	db.DropTable(&lq.ContainerJobTracker{})
	db.DropTable(&lq.ContainerJobGroup{})
	db.DropTable(&lq.ResourceInstance{})
	db.DropTable(&lq.User{})
	db.DropTable(&lq.AwsAccount{})
	db.Exec("DROP TABLE resource_events")
	database.Mesos().DropTable()


	if err := db.CreateTable(&lq.User{}).Error; err != nil {
		log.Error(err)
	}
	if err := db.CreateTable(&lq.ContainerJobGroup{}).Error; err != nil {
		log.Error(err)
	}
	//db.Model(&UserJob{}).AddForeignKey("owner_id", "user(id)", "CASCADE", "CASCADE")

	if err := db.CreateTable(&lq.ContainerJob{}).Error; err != nil {
		log.Error(err)
	}
	//db.Model(&ContainerJob{}).AddForeignKey("user_job_id", "user_job(id)", "CASCADE", "CASCADE")

	if err := db.CreateTable(&lq.ResourceInstance{}).Error; err != nil {
		log.Error(err)
	}

	if err := db.CreateTable(&lq.ContainerJobTracker{}).Error; err != nil {
		log.Error(err)
	}
	//db.Model(&ResourceInstance{}).AddForeignKey("owner_id", "user(id)", "CASCADE", "CASCADE")

	if err := db.CreateTable(&lq.AwsAccount{}).Error; err != nil {
		log.Error(err)
	}

	if err := db.Table("resource_events").CreateTable(&lq.ResourceEvent{}).Error; err != nil {
		log.Error(err)
	}

	if err := database.Mesos().CreateTable(); err != nil {
		log.Error(err)
	}

//	if err := db.Exec("ALTER DATABASE liquiddev SET default_transaction_isolation=serializable").Error; err != nil {
//		log.Error(err)
//	}
}
