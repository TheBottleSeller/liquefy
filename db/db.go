package db

import (
	"fmt"
	"github.com/jinzhu/gorm"
	_ "github.com/lib/pq"
	"os"
	"errors"

	lq "bargain/liquefy/models"
)

var db gorm.DB

func GetDB() *gorm.DB {
	return &db
}

func Connect(hostname string) error {
	fmt.Println("Connecting to postgres at " + hostname)

	var dbUser string
	var dbPass string
	var dbName string
	var sslMode string
	//Configure Based on ENV

	if os.Getenv("ENV") == "PRODUCTION" {
		dbUser = os.Getenv("DB_USER")
		dbPass = os.Getenv("DB_PASS")
		dbName = os.Getenv("DB_NAME")
		//TODO : Fix this with adding certs and RDS
		sslMode = "disable"

		//Validate that fields are not empty ( TODO: Add Password Check with RDS deploy )
		if (dbUser == "" || dbName == "" ){
			return errors.New("Empty DB Info in ENV")
		}

	} else {
		dbUser = "liquiddev"
		dbPass = ""
		dbName = "liquiddev"
		sslMode = "disable"
	}

	var err error
	connectString := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",hostname,5432,dbUser,dbPass,dbName,sslMode)
	db, err = gorm.Open("postgres",connectString)
	if err != nil {
		return err
	}

	if err := db.DB().Ping(); err != nil {
		return err
	}

	db.DB().SetMaxIdleConns(10)
	db.DB().SetMaxOpenConns(100)
	db.SingularTable(true)

	return nil
}

func TxCommitOrRollback(tx *gorm.DB, err *error, formatMessage string, params ...interface{}) {
	if *err != nil {
		tx.Rollback()
		*err = lq.NewErrorf(*err, formatMessage, params...)
	} else {
		if commitErr := tx.Commit().Error; commitErr != nil {
			*err = lq.NewErrorf(commitErr, formatMessage, params...)
		}
	}
}
