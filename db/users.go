package db

import (
	lq "bargain/liquefy/models"
	log "github.com/Sirupsen/logrus"
	mesos "github.com/mesos/mesos-go/mesosproto"

	"fmt"
)

type UsersTable interface {
	Create(user *lq.User) error
	Get(userID uint) (*lq.User, error)
	GetByEmail(email string) (*lq.User, error)
	GetAll() ([]*lq.User, error)
	GetAllWithPendingJobs() ([]*lq.User, error)
	Update(uint, string,string) (error)
}

type usersTable struct{}

func Users() UsersTable {
	return &usersTable{}
}

func (table *usersTable) Create(user *lq.User) error {
	query := db.Create(user)
	if query.Error != nil {
		err := lq.NewErrorf(query.Error, "Failed cerating user: %v", user)
		log.Error(err)
		return err
	}
	return nil
}

func (table *usersTable) Get(userID uint) (*lq.User, error) {
	var user lq.User
	query := db.Find(&user, userID)
	if query.Error != nil {
		err := lq.NewErrorf(query.Error, "Failed getting user %d", userID)
		log.Error(err)
		return &user, err
	}
	return &user, nil
}

func (table *usersTable) GetByEmail(email string) (*lq.User, error) {
	var user lq.User
	query := db.Where(&lq.User{ Email: email }).First(&user)
	if query.Error != nil {
		err := lq.NewErrorf(query.Error, "Failed getting user by email %s", email)
		log.Error(err)
		return &user, err
	}
	return &user, nil
}

func (table *usersTable) GetAll() ([]*lq.User, error) {
	var users []*lq.User
	query := db.Find(&users)
	if query.Error != nil {
		err := lq.NewErrorf(query.Error, "Failed getting all users")
		log.Error(err)
		return users, err
	}
	return users, nil
}

func (table *usersTable) Update(userID uint, field string, value string) (error) {
	query := db.Find(&lq.User{}, userID).Update(field, value)
	if query.Error != nil {
		err := lq.NewErrorf(query.Error, "Failed updating user %d filed %s with value %s", userID, field, value)
		log.Error(err)
		return err
	}
	return nil
}

func (table *usersTable) GetAllWithPendingJobs() ([]*lq.User, error) {
	var users []*lq.User
	rows, err := db.Raw(fmt.Sprintf("SELECT id, api_key, username, firstname, lastname, email, public_id, " +
		"aws_account_id  FROM public.user WHERE aws_account_id > 0 AND " +
			"(SELECT COUNT(*) FROM container_job WHERE owner_id = public.user.id AND status = '%s') > 0",
			mesos.TaskState_TASK_STAGING.String())).Rows()
	if err != nil {
		err = lq.NewErrorf(err, "Failed getting all users with pending jobs in bulk")
		log.Error(err)
		return users, err
	}
	for rows.Next() {
		user := lq.User{}
		err = rows.Scan(&user.ID, &user.ApiKey, &user.Username, &user.Firstname, &user.Lastname, &user.Email,
			&user.PublicID, &user.AwsAccountID)
		if err != nil {
			err = lq.NewErrorf(err, "Failed getting all users with pending jobs by row")
			return users, err
		}
		users = append(users, &user)
	}
	return users, nil
}