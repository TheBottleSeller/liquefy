package executor

import (
	"github.com/mesos/mesos-go/mesosproto"
	"os"
)

// MockedExecutorDriver is used for testing the executor driver.
type MockedExecutorDriver struct {
}

// NewMockedExecutorDriver returns a mocked executor.
func NewMockedExecutorDriver() *MockedExecutorDriver {
	return &MockedExecutorDriver{}
}

func (e *MockedExecutorDriver) Start() (mesosproto.Status, error) {
	return mesosproto.Status_DRIVER_RUNNING, nil
}

// Reregistered implements the Reregistered handler.
func (e *MockedExecutorDriver) Stop() (mesosproto.Status, error) {
	os.Exit(0)
	return mesosproto.Status_DRIVER_STOPPED, nil
}

// Reregistered implements the Reregistered handler.
func (e *MockedExecutorDriver) Abort() (mesosproto.Status, error) {
	return mesosproto.Status_DRIVER_ABORTED, nil
}

// Disconnected implements the Disconnected handler.
func (e *MockedExecutorDriver) Run() (mesosproto.Status, error) {
	return mesosproto.Status_DRIVER_RUNNING, nil
}

func (e *MockedExecutorDriver) Join() (mesosproto.Status, error) {
	return mesosproto.Status_DRIVER_RUNNING, nil
}

func (e *MockedExecutorDriver) SendStatusUpdate(*mesosproto.TaskStatus) (mesosproto.Status, error) {
	return mesosproto.Status_DRIVER_RUNNING, nil
}

func (e *MockedExecutorDriver) SendFrameworkMessage(string) (mesosproto.Status, error) {
	return mesosproto.Status_DRIVER_RUNNING, nil
}
