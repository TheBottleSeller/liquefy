package awsutil

import (
	"fmt"
	"github.com/vaughan0/go-ini"
	"os"
	"path/filepath"
)

type AWSUserConfig struct {
	ARN              string
	Name             string
	AccessKeyID      string
	SecretAccessKey  string
	VpcID            string
	PemPath          string
	PemName          string
	Region           string
	AvailabilityZone string
	SecurityGroup    string
	Subnet           string
}

type InstanceOffering struct {
	OfferingName     string
	InstanceType     string
	InstanceAMI      string
	EBSStorageAmount string
}

type SystemConfig struct {
	MaxNodes int
}

var (
	// ErrSharedCredentialsHomeNotFound is emitted when the user directory cannot be found.
	ErrSharedCredentialsHomeNotFound = fmt.Errorf("User home directory not found.")
)

// Profile ini file example: $HOME/
type ConfigProvider struct {
	// Path to the shared credentials file. If empty will default to current user's home directory.
	Filename string

	// retrieved states if the credentials have been successfully retrieved.
	retrieved bool
}

func NewConfigProvider(filename string) *ConfigProvider {
	return &ConfigProvider{
		Filename:  filename,
		retrieved: false,
	}
}

func LoadUserConfig(filename string) (AWSUserConfig, error) {
	config, err := ini.LoadFile(filename)
	if err != nil {
		return AWSUserConfig{}, err
	}

	return extractUserConfig(config.Section("aws"), filename)
}

func extractUserConfig(profile map[string]string, filename string) (AWSUserConfig, error) {
	errorMsg := "config at " + filename + " did not contain %s"
	arn, ok := profile["arn"]
	if !ok {
		return AWSUserConfig{}, fmt.Errorf(errorMsg, "arn")
	}

	name, ok := profile["name"]
	if !ok {
		return AWSUserConfig{}, fmt.Errorf(errorMsg, "name")
	}

	id, ok := profile["aws_access_key_id"]
	if !ok {
		return AWSUserConfig{}, fmt.Errorf(errorMsg, "aws_access_key_id")
	}

	secret, ok := profile["aws_secret_access_key"]
	if !ok {
		return AWSUserConfig{}, fmt.Errorf(errorMsg, "aws_secret_access_key")
	}

	pempath, ok := profile["pem_path"]
	if !ok {
		return AWSUserConfig{}, fmt.Errorf(errorMsg, "pem_path")
	}

	pemname, ok := profile["pem_name"]
	if !ok {
		return AWSUserConfig{}, fmt.Errorf(errorMsg, "pem_name")
	}

	region, ok := profile["region"]
	if !ok {
		return AWSUserConfig{}, fmt.Errorf(errorMsg, "region")
	}

	availabilityZone, ok := profile["availability_zone"]
	if !ok {
		return AWSUserConfig{}, fmt.Errorf(errorMsg, "availability_zone")
	}

	vpcId, ok := profile["vpc_id"]
	if !ok {
		return AWSUserConfig{}, fmt.Errorf(errorMsg, "vpc_id")
	}

	sgid, ok := profile["security_group_id"]
	if !ok {
		return AWSUserConfig{}, fmt.Errorf(errorMsg, "security_group_id")
	}

	subnet, ok := profile["subnet_id"]
	if !ok {
		return AWSUserConfig{}, fmt.Errorf(errorMsg, "subnet_id")
	}

	//Export the ENV Vars for the AWS LIb to use
	// THIS NEEDS TO GO TO HANDLE MULTIPLE USERS
	os.Setenv("AWS_ACCESS_KEY_ID", id)
	os.Setenv("AWS_SECRET_ACCESS_KEY", secret)

	return AWSUserConfig{
		ARN:              arn,
		Name:             name,
		AccessKeyID:      id,
		SecretAccessKey:  secret,
		VpcID:            vpcId,
		PemPath:          pempath,
		PemName:          pemname,
		Region:           region,
		AvailabilityZone: availabilityZone,
		SecurityGroup:    sgid,
		Subnet:           subnet,
	}, nil
}

// Returns the filename to use to read AWS shared credentials.
// Will return an error if the user's home directory path cannot be found.
func (p *ConfigProvider) filename() (string, error) {
	if p.Filename == "" {
		homeDir := os.Getenv("HOME") // *nix
		if homeDir == "" {           // Windows
			homeDir = os.Getenv("USERPROFILE")
		}
		if homeDir == "" {
			return "", ErrSharedCredentialsHomeNotFound
		}
		p.Filename = filepath.Join(homeDir, ".clusterconf", "")
	}
	return p.Filename, nil
}
