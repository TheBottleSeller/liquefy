package db

import (
	lq "bargain/liquefy/models"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAccountCreate(t *testing.T) {
	var (
		userId     = uint(7896)
		testKey    = "testaccess"
		testSecret = "testsecret"
	)

	awsAccount := &lq.AwsAccount{
		AwsAccessKey: &testKey,
		AwsSecretKey: &testSecret,
	}

	Connect("localhost")
	accountsTable := AwsAccounts()

	accountsTable.Create(userId, awsAccount)
	defer db.Delete(&awsAccount)

	// Directly accessing the record will yield the encrypted value
	var result lq.AwsAccount
	query := db.Find(&result, *awsAccount.ID)
	assert.Nil(t, query.Error)
	assert.NotEqual(
		t,
		"testsecret",
		*result.AwsSecretKey,
	)
	assert.NotEqual(
		t,
		"testaccess",
		*result.AwsAccessKey,
	)

	// Accessing via awsAccountTable.Get will decrypt the values at runtime
	var res *lq.AwsAccount
	res, err := accountsTable.Get(uint(*awsAccount.ID))
	assert.Nil(t, err)

	assert.Equal(
		t,
		"testsecret",
		*res.AwsSecretKey,
	)
	assert.Equal(
		t,
		"testaccess",
		*res.AwsAccessKey,
	)

}

func TestDecryptError(t *testing.T) {
	var (
		key    = "boguskey"
		secret = "bogussecret"
	)
	awsAccount := &lq.AwsAccount{
		AwsAccessKey: &key,
		AwsSecretKey: &secret,
	}

	Connect("localhost")
	accountsTable := AwsAccounts()

	// Write directly to DB, circumventing encryption step
	db.Create(awsAccount)
	defer db.Delete(&awsAccount)

	_, err := accountsTable.Get(uint(*awsAccount.ID))

	// Should get an error since keys are not decrypt-able
	assert.NotNil(t, err, "Error should be raised since keys are not decrypt-able")

}
