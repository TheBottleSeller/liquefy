#!/usr/bin/env bash

#!/bin/sh
# args: docker-machine-name aws-access-key aws-secret-key
if [ -z "$1" ]; then
    echo "Need to supply a docker-machine name"
    exit 1
fi

if [ -z "$2" ]; then
    echo "Need to supply aws access key"
    exit 1
fi

if [ -z "$3" ]; then
    echo "Need to supply aws secret key"
    exit 1
fi

if [ -z "$4" ]; then
    echo "Need to supply vpc id"
    exit 1
fi

DOCKER_MACHINE_NAME=$1
AWS_ACCESS_KEY=$2
AWS_SECRET_KEY=$3
AWS_VPC_ID=$4
AWS_REGION=$5
#AWS_SUBNET=$6
AWS_SECGROUP=$6

# AWS_ZONE=$5
# AWS_INSTANCE_TYPE=$6
# AWS_BID_PRICE=$7
# AWS_AMI_ID=$8
# AWS_VPC_ID=$9

if [ -z "$INSTANCE_TYPE" ]; then
    INSTANCE_TYPE=m3.medium
fi

# ubuntu-trusty-14.04-amd64-server, this works with t1.micro
#UBUNTU

#US-West
# AMI=ami-d16a8b95

#US-East
AMI=ami-d85e75b0

#Centos
#AMI=ami-7ea24a17


# this works too, but needs to be upgraded
#AMI=ami-8b3cdecf


echo "Provisioning AWS Spot Instance"

#ADD THIS:
#--amazonec2-subnet-id $AWS_SUBNET \

docker-machine -D create -d amazonec2 \
    --amazonec2-access-key $AWS_ACCESS_KEY \
    --amazonec2-secret-key $AWS_SECRET_KEY \
    --amazonec2-region $AWS_REGION \
    --amazonec2-vpc-id $AWS_VPC_ID \
    --amazonec2-security-group  "sg-effc6089" \
    --amazonec2-ami $AMI \
    --amazonec2-instance-type $INSTANCE_TYPE \
    $DOCKER_MACHINE_NAME

exit 0

