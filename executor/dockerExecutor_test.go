package executor_test

//import (
//	"bargain/liquefy/liquidmesosexecutor"
//	"github.com/aws/aws-sdk-go/aws"
//	"github.com/aws/aws-sdk-go/aws/credentials"
//	"github.com/aws/aws-sdk-go/service/ec2"
//	. "github.com/smartystreets/goconvey/convey"
//	"os/exec"
//	"testing"
//
//	"bargain/liquefy/awsutil"
//	lq "bargain/liquefy/models"
//	"github.com/aws/aws-sdk-go/aws/session"
//	"strings"
//)
//
//const dm = "docker-machine"
//const testmachine = "lqexecutor-testmachine"
//


//
//func prepareEnv() liquidmesosexecutor.DockerActions{
//
//
//	//Check for docker machine
//	args := []string{"status", testmachine}
//	output, err := exec.Command(dm, args...).CombinedOutput()
//
//	//Setup Machine if not running
//	if !strings.Contains(string(output), "Running") {
//		Println(string(output))
//		setupMachine()
//	} else {
//		Println(string(output))
//		So(err, ShouldBeNil)
//	}
//
//	//Setup ENV
//	args = []string{"env", testmachine}
//	output, err = exec.Command(dm, args...).CombinedOutput()
//	So(err, ShouldBeNil)
//
//	//Load into env
//	for _, e := range strings.Split(string(output), "\n") {
//		_, err = exec.Command("bash", "-c", "eval "+e).CombinedOutput()
//		So(err, ShouldBeNil)
//	}
//
//	return liquidmesosexecutor.NewDockerActionsFromEnv()
//}
//
//func setupMachine() {
//
//	//Clean before setting up new machine
//	tearDownMachine()
//
//	//Setup Machine using AWS .ini File
//	awsCreds, err := awsutil.LoadUserConfig("/Users/anuraagjain/.aws/aws_config.ini")
//	So(err, ShouldBeNil)
//
//	//Launch Dev Machine for testing
//	launchArgs := []string{
//		"../setup/provision_aws.sh",
//		testmachine,
//		awsCreds.AccessKeyID,
//		awsCreds.SecretAccessKey,
//		awsCreds.VpcID,
//		awsCreds.SecurityGroup,
//		awsCreds.Region,
//		awsCreds.Subnet,
//		"g2.2xlarge",
//		"0.1",
//	}
//
//	Println("Bidding On AWS Machine .... ")
//	output, err := exec.Command("bash", launchArgs...).CombinedOutput()
//	Println(string(output))
//	So(err, ShouldBeNil)
//}
//
//func tearDownMachine() {
//
//	//Stop and Delete Machine if Exists
//	sargs := []string{"stop", testmachine}
//	output, err := exec.Command(dm, sargs...).CombinedOutput()
//	if strings.ContainsAny(string(output), "does not exist") {
//		Println("No Existing Machine Found")
//	} else {
//		So(err, ShouldBeNil)
//	}
//
//	rargs := []string{"rm", "-f", testmachine}
//	output, err = exec.Command(dm, rargs...).CombinedOutput()
//	if strings.ContainsAny(string(output), " does not exist") {
//		Println("No Existing Machine Found")
//	} else {
//		So(err, ShouldBeNil)
//	}
//
//	//Tear down all the keys that we made
//	awsCreds, err := awsutil.LoadUserConfig("/Users/anuraagjain/.aws/aws_config.ini")
//	So(err, ShouldBeNil)
//	creds := credentials.NewStaticCredentials(awsCreds.AccessKeyID, awsCreds.SecretAccessKey, "")
//	config := &aws.Config{
//		Region:      &awsCreds.Region,
//		Credentials: creds,
//	}
//
//	svc := ec2.New(session.New(config))
//	createPtr := testmachine
//	params := &ec2.DeleteKeyPairInput{
//		KeyName: &createPtr,
//	}
//	_, err = svc.DeleteKeyPair(params)
//	So(err, ShouldBeNil)
//
//}
//
//func TestEnvSetup(t *testing.T) {
//	Convey("Setting Up Env", t, func() {
//
//		dockerActions := prepareEnv()
//
//		Convey("Node should be empty", func() {
//			cts, err := dockerActions.ListAllContainers()
//			So(len(cts), ShouldEqual, 0)
//			So(err, ShouldBeNil)
//		})
//	})
//}
//
//func TestCreateContainer(t *testing.T) {
//
//	Convey("Fetching env connection", t, func() {
//
//		dockerActions := prepareEnv()
//		SetDefaultFailureMode(FailureHalts)
//
//		Convey("Create simple ubuntu", func() {
//
//			ctj := lq.ContainerJob{
//				ID:          uint(1),
//				Name:        "simpleCreate",
//				SourceImage: "ubuntu:latest",
//				Command:     "for i in $(seq 1 9999); do echo $i ; done",
//				Ram:         100,
//				Cpu:         1,
//				SourceType:  "image",
//			}
//
//			ctid, err := dockerActions.CreateContainer(&ctj)
//			So(ctid, ShouldNotBeEmpty)
//			So(err, ShouldBeNil)
//
//			cts, err := dockerActions.ListAllContainers()
//			So(len(cts), ShouldEqual, 1)
//			So(err, ShouldBeNil)
//
//		})
//
//	})
//
//}
//
//func TestStartContainer(t *testing.T) {
//	Convey("Fetching env connection", t, func() {
//
//		dockerActions := prepareEnv()
//		SetDefaultFailureMode(FailureHalts)
//
//		Convey("Create simple ubuntu", func() {
//
//			ctj := lq.ContainerJob{
//				ID:          uint(1),
//				Name:        "simpleRun",
//				SourceImage: "ubuntu:latest",
//				Command:     "for i in $(seq 1 9999); do echo $i ; done",
//				Ram:         100,
//				Cpu:         1,
//				SourceType:  "image",
//			}
//
//			ctid, err := dockerActions.CreateContainer(&ctj)
//			So(ctid, ShouldNotBeEmpty)
//			So(err, ShouldBeNil)
//
//			ctj.ContainerId = ctid
//			ctid, err = dockerActions.Start(&ctj)
//			So(ctid, ShouldNotBeEmpty)
//			So(err, ShouldBeNil)
//
//			cts, err := dockerActions.ListAllContainers()
//			So(len(cts), ShouldEqual, 2)
//			So(err, ShouldBeNil)
//
//		})
//
//	})
//}
//
////https://github.com/TrackDR/cudadocker.git
//func TestGithubBuildRun(t *testing.T) {
//	Convey("Fetching env connection", t, func() {
//
//		dockerActions := prepareEnv()
//
//		Convey("Build simple cuda", func() {
//
//			ctj := lq.ContainerJob{
//				ID:          uint(3),
//				Name:        "githubBuild",
//				SourceImage: "device_query",
//				Command:     "while :; do echo 'Hit CTRL+C'; sleep 1; done",
//				Ram:         10000,
//				Cpu:         4,
//				Gpu:         1,
//				SourceType:  "image",
//			}
//
//			ctid, err := dockerActions.CreateContainer(&ctj)
//			So(ctid, ShouldNotBeEmpty)
//			So(err, ShouldBeNil)
//
//			ctj.ContainerId = ctid
//			ctid, err = dockerActions.Start(&ctj)
//			So(ctid, ShouldNotBeEmpty)
//			So(err, ShouldBeNil)
//
//			cts, err := dockerActions.ListAllContainers()
//			So(len(cts), ShouldEqual, 3)
//			So(err, ShouldBeNil)
//
//		})
//
//	})
//}
//
//func TestGithubJob(t *testing.T) {
//
//}
//
//func TestGPUJob(t *testing.T) {
//
//}
//
//func TestPrintSomething(t *testing.T) {
//
//	//	ctjob := lq.ContainerJob{
//	//		Name: "Niglet",
//	//		SourceImage: "github.com/swagger-api/swagger-ui.git",
//	//		SourceType: "code",
//	//		Command: "",
//	//		Environment: []lq.EnvVar{{Variable: "y", Value: "2"}},
//	//		Ram:         9999999999,
//	//		Cpu:         1,
//	//		FirstTry:      true,
//	//		Status:        "",
//	//		OwnerID:    uint(1),
//	//	}
//	//
//	//	ca := liquidmesosexecutor.NewDockerActionsFromEnv()
//	//	fmt.Println(ca)
//	//
//	//	out,err := ca.CreateContainer(&ctjob)
//	//
//	//	ctjob.ContainerId= out
//	//	fmt.Println(err)
//	//	if err == nil {
//	//		out,err := ca.Start(&ctjob)
//	//		fmt.Println(out)
//	//		fmt.Println(err)
//	//
//	//	}
//	//	fmt.Println(out)
//
//}
