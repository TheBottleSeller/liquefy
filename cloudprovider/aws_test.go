package cloudprovider

import(
    ."github.com/smartystreets/goconvey/convey"
    "testing"
    "os"
	"fmt"

    "bargain/liquefy/awsutil"
	"time"
)

func TestCreateAwsCloud(t *testing.T) {
    Convey("Create new aws cloud with creds", t, func() {
        NewAwsCloud(fetchCreds())
    })
}

var testRegion = Region("us-west-1")
var testAz = AZ("us-west-1a")

func fetchCreds() (*string, *string) {
	home := os.Getenv("HOME")
	awsCreds, err := awsutil.LoadUserConfig(home + "/.aws/aws_config.ini")
	So(err, ShouldBeNil)
	return &awsCreds.AccessKeyID, &awsCreds.SecretAccessKey
}

func TestSetupVPC(t *testing.T) {

	Convey("Setting up Creds", t, func() {

		aws := NewAwsCloud(fetchCreds())

		Convey("Setup VPC in US-West-1", func() {
			vpcid,err := aws.CreateVPC(testRegion)
			So(err,ShouldBeNil)
			So(vpcid,ShouldNotBeNil)

			Convey("Destory VPC", func(){
				err := aws.DestroyVPC(testRegion,vpcid)
				So(err,ShouldBeNil)
			})
		})
	})
}

func TestSetupSubnets(t *testing.T){
	Convey("Setting up Creds", t, func() {

		aws := NewAwsCloud(fetchCreds())

		Convey("Setup VPC in US-West-1", func() {
			vpcid, err := aws.CreateVPC(testRegion)
			So(err,ShouldBeNil)
			So(vpcid,ShouldNotBeNil)

			Convey("Setup Subnets", func(){
				subnets := []string {}
				for _, az := range AWSRegionsToAZs[testRegion] {
					subnetId, err := aws.CreateSubnet(testRegion, string(az), vpcid)
					So(err,ShouldBeNil)
					subnets = append(subnets, subnetId)
				}
				So(len(subnets),ShouldEqual,3)
			})

			Convey("Destory VPC", func(){
				err := aws.DestroyVPC(testRegion,vpcid)
				So(err,ShouldBeNil)
			})
		})
	})
}


func TestSecurityGroup(t *testing.T){
	Convey("Setting up Creds", t, func() {

		aws := NewAwsCloud(fetchCreds())

		Convey("Setup VPC in US-West-1", func() {
			vpcid,err := aws.CreateVPC(testRegion)
			So(err,ShouldBeNil)
			So(vpcid,ShouldNotBeNil)

			Convey("Setup Security Group", func(){
				_, err := aws.CreateSecurityGroup(testRegion,vpcid,"unittestSG")
				So(err,ShouldBeNil)
			})

			//Test Create-Over SG
			Convey("Destory VPC", func(){
				err := aws.DestroyVPC(testRegion,vpcid)
				So(err,ShouldBeNil)
			})
		})
	})
}

func TestCleanupAwsAccount(t *testing.T) {
	Convey("Cleaning up aws account", t, func() {
		aws := NewAwsCloud(fetchCreds())
		aws.CleanUpAwsAccount()
	})
}

func TestGettingSpotPrices(t *testing.T) {
	Convey("Getting spot prices", t, func() {
		aws := NewAwsCloud(fetchCreds())

		end := time.Now()
		start := end.Add(time.Duration(-24) * time.Hour)
		spotPrices, err := aws.GetSpotPriceHistory(testAz, "g2.2xlarge", start, end)
		if err != nil {
			fmt.Println(err)
			return
		}

		fmt.Println("Sta: ", start.UTC().String())
		fmt.Println("End: ", end.UTC().String())
		for _, spotPrice := range spotPrices {
			fmt.Println(spotPrice.Timestamp.UTC().String())
		}
	})
}

func TestVerifyPolicy(t *testing.T){
	Convey("Test Policy Verification", t, func() {
		aws := NewAwsCloud(fetchCreds())
		err := aws.VerifyPolicy()
		So(err,ShouldBeNil)
	})
}

