package executor

import (
	"testing"
)

func TestSingleScope(t *testing.T) {

	//	output := ""
	//	executor := NewLiquidExecutor("tcp://192.168.99.102:2376")
	//	mockDriver := NewMockedExecutorDriver()
	//
	//
	//	Convey("hi", t, func() {
	//		executor.LaunchTask(mockDriver,test.TaskInfo1)
	//		executor.LaunchTask(mockDriver,test.TaskInfo2)
	//
	//		output += "done"
	//		So("done",ShouldEqual,output)
	//	})

	//executor.Disconnected(mockDriver)
}

//func TestCommon(t *testing.T) {
//	RegisterFailHandler(Fail)
//	RunSpecs(t, "Executor Suite")
//}
//
//var _ = Describe("Executor", func() {
//
//	var executor *liquidExecutor
//	executor = NewLiquidExecutor("tcp://192.168.33.10:2375")
//
//	var ed *MockedExecutorDriver
//	ed = NewMockedExecutorDriver()
//
//	Context("Launch Task Test", func() {
//		It("Simple Launch Task", func() {
//			ed.Mock.On("SendStatusUpdate").Return("","").Twice()
//			executor.LaunchTask(ed,test.TaskInfo)
//		})
//	})
//
//
//})
