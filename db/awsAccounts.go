package db

import (
	lq "bargain/liquefy/models"
	log "github.com/Sirupsen/logrus"
)

type AwsAccountTable interface {
	Create(uint, *lq.AwsAccount) error
	Update(*lq.AwsAccount) error
	Get(uint) (*lq.AwsAccount, error)
}

type awsAccountTable struct{}

func AwsAccounts() AwsAccountTable {
	return &awsAccountTable{}
}

func (table *awsAccountTable) Get(id uint) (*lq.AwsAccount, error) {
	var res lq.AwsAccount
	query := db.Find(&res, id)
	if query.Error != nil {
		log.Error(query.Error)
		return &res, query.Error
	}

	decryptErr := decryptAccountSecrets(&res)
	if decryptErr != nil {
		log.Error(decryptErr)
		return &res, decryptErr
	}

	return &res, nil
}

func encryptAccountSecrets(awsAccount *lq.AwsAccount) {
	encoder := NewEncoder()

	encryptedAccessKey := string(
		encoder.Encode([]byte(*awsAccount.AwsAccessKey)))
	encryptedSecret := string(
		encoder.Encode([]byte(*awsAccount.AwsSecretKey)))
	
	awsAccount.AwsAccessKey = &encryptedAccessKey
	awsAccount.AwsSecretKey = &encryptedSecret
}

func decryptAccountSecrets(awsAccount *lq.AwsAccount) error {
	encoder := NewEncoder()

	decoded, err := encoder.Decode([]byte(*awsAccount.AwsSecretKey))
	if err != nil {
		return lq.NewErrorf(err, "Failed to decode secret for account %d", *awsAccount.ID)
	}
	decryptedSecret := string(decoded)

	decoded, err = encoder.Decode([]byte(*awsAccount.AwsAccessKey))
	if err != nil {
		return lq.NewErrorf(err, "Failed to decode access key for account %d", *awsAccount.ID)
	}
	decryptedAccessKey := string(decoded)

	awsAccount.AwsAccessKey = &decryptedAccessKey
	awsAccount.AwsSecretKey = &decryptedSecret
	return nil
}

func (table *awsAccountTable) Create(userID uint, awsAccount *lq.AwsAccount) error {
	encryptAccountSecrets(awsAccount)
	query := db.Create(awsAccount)

	if query.Error != nil {
		log.Error("Failed creating aws account")
		log.Error(query.Error)
		return query.Error
	}

	var user lq.User
	query = db.Find(&user, userID)
	if query.Error != nil {
		log.Error("Failed find user when creating aws account")
		log.Error(query.Error)
		return query.Error
	}

	query = db.Model(&user).UpdateColumn("aws_account_id", awsAccount.ID)
	if query.Error != nil {
		log.Errorf("Failed updating aws account id for user: %d", userID)
		log.Error(query.Error)
		return query.Error
	}

	return nil
}

func (table *awsAccountTable) Update(account *lq.AwsAccount) error {
	encryptAccountSecrets(account)
	query := db.Save(account)
	if query.Error != nil {
		log.Error("Failed updating aws account")
		log.Error(query.Error)
	}
	return query.Error
}
