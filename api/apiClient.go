package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"
	lq "bargain/liquefy/models"
)

type ApiUser struct {
	ApiKey     string        `json:"apiKey"`
	Username   string        `json:"username"`
	Password   string        `json:"password"`
	Firstname  string        `json:"firstname"`
	Lastname   string        `json:"lastname"`
	Email      string        `json:"email"`
	PublicID   string        `json:"publicId"`
}

type apiClient struct {
	url string
}

type ApiClient interface {
	CreateUser(user *ApiUser) (string, error)
	GetUser(apiKey string) *lq.User
	DeleteUser(apiKey string)

	RegisterAwsAccount(awsAccessKey, awsSecretKey, apiKey string)

	GetInstance(instanceId uint, apiKey string) *lq.ResourceInstance
	DeleteInstance(instanceId uint, apiKey string) error

	CreateJob(job *ContainerJobPublic, apiKey string) uint
	GetJob(jobId uint, apiKey string) *lq.ContainerJob
	DeleteJob(jobId uint, apiKey string) error
}

func NewApiClient(serverUrl string) ApiClient {
	return &apiClient{serverUrl}
}

func (server *apiClient) CreateUser(user *ApiUser) (string, error) {
	targetUrl := fmt.Sprintf("%s/private/user", server.url)
	jsonBytes, err := json.Marshal(user)
	if err != nil {
		return "", err
	}

	data, err := server.post(targetUrl, "", jsonBytes)
	if err != nil {
		return "", err
	}

	apiKey := string(data[:])
	log.Infof("Created used with api key: %s", apiKey)
	return apiKey, nil
}

func (server *apiClient) GetUser(apiKey string) *lq.User {
	targetUrl := fmt.Sprintf("%s/api/user", server.url)
	data, err := server.get(targetUrl, apiKey)
	if err != nil {
		panic(err)
	}

	var user lq.User
	err = json.Unmarshal(data, &user)
	fmt.Println(string(data[:]))

	if err != nil {
		panic(err)
	}
	return &user
}

func (server *apiClient) DeleteUser(apiKey string) {
	targetUrl := fmt.Sprintf("%s/api/user", server.url)
	_, err := server.delete(targetUrl, apiKey)
	if err != nil {
		panic(err)
	}
}

func (server *apiClient) RegisterAwsAccount(awsAccessKey, awsSecretKey, apiKey string) {
	targetUrl := fmt.Sprintf("%s/api/linkAwsAccount", server.url)
	account := lq.AwsAccount{
		AwsAccessKey:   &awsAccessKey,
		AwsSecretKey:   &awsSecretKey,
	}

	jsonBytes, err := json.Marshal(account)
	if err != nil {
		panic(err)
	}

	_, err = server.post(targetUrl, apiKey, jsonBytes)
	if err != nil {
		panic(err)
	}

	targetUrl = fmt.Sprintf("%s/api/setupAwsAccount", server.url)
	_, err = server.post(targetUrl, apiKey, []byte{})
	if err != nil {
		panic(err)
	}

	return
}

func (server *apiClient) GetInstance(instanceId uint, apiKey string) *lq.ResourceInstance {
	targetUrl := fmt.Sprintf("%s/api/instance/%d", server.url, instanceId)
	log.Info(targetUrl)
	data, err := server.get(targetUrl, apiKey)
	if err != nil {
		panic(err)
	}
	log.Info(string(data))
	var instance lq.ResourceInstance
	err = json.Unmarshal(data, &instance)
	if err != nil {
		panic(err)
	}
	return &instance
}

func (server *apiClient) DeleteInstance(instanceId uint, apiKey string) error {
	targetUrl := fmt.Sprintf("%s/api/instance/%d", server.url, instanceId)
	_, err := server.delete(targetUrl, apiKey)
	if err != nil {
		panic(err)
	}
	return nil
}

func (server *apiClient) CreateJob(job *ContainerJobPublic, apiKey string) uint {
	targetUrl := fmt.Sprintf("%s/api/job", server.url)
	jsonBytes, err := json.Marshal(job)
	if err != nil {
		panic(err)
	}

	data, err := server.post(targetUrl, apiKey, jsonBytes)
	if err != nil {
		log.Error(err)
		return 0
	}

	id, err := strconv.Atoi(strings.Trim(string(data), "\n "))
	if err != nil {
		log.Error(err)
		return 0
	}
	return uint(id)
}

func (server *apiClient) GetJob(jobId uint, apiKey string) *lq.ContainerJob {
	targetUrl := fmt.Sprintf("%s/api/job/%d", server.url, jobId)
	data, err := server.get(targetUrl, apiKey)
	if err != nil {
		panic(err)
	}

	var job lq.ContainerJob
	err = json.Unmarshal(data, &job)
	if err != nil {
		panic(err)
	}
	return &job
}

func (server *apiClient) DeleteJob(jobId uint, apiKey string) error {
	targetUrl := fmt.Sprintf("%s/api/job/%d", server.url, jobId)
	_, err := server.delete(targetUrl, apiKey)
	if err != nil {
		panic(err)
	}
	return nil
}

func (server *apiClient) get(targetUrl string, apiKey string) ([]byte, error) {
	return server.executeHttp("GET", targetUrl, apiKey, []byte{})
}

func (server *apiClient) post(targetUrl string, apiKey string, jsonBytes []byte) ([]byte, error) {
	return server.executeHttp("POST", targetUrl, apiKey, jsonBytes)
}

func (server *apiClient) delete(targetUrl string, apiKey string) ([]byte, error) {
	return server.executeHttp("DELETE", targetUrl, apiKey, []byte{})
}

func (server *apiClient) executeHttp(httpType string, targetUrl string, apiKey string, jsonBytes []byte) ([]byte, error) {
	req, err := http.NewRequest(httpType, targetUrl, bytes.NewBuffer(jsonBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return []byte{}, err
	}
	defer resp.Body.Close()

	contents, err := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return contents, fmt.Errorf("Msg: %s\nStatus Code: %d", string(contents), resp.StatusCode)
	}
	return contents, err
}
