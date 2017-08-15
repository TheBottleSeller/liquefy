#!/bin/sh
# args: docker-machine-name aws-access-key aws-secret-key
if [ -z "$1" ]; then
    echo "Need to supply an aws machine name"
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

if [ -z "$5" ]; then
    echo "Need to supply security group name"
    exit 1
fi

if [ -z "$6" ]; then
    echo "Need to supply region"
    exit 1
fi

if [ -z "$7" ]; then
    echo "Need to supply zone"
    exit 1
fi

if [ -z "$8" ]; then
    echo "Need to supply subnet id"
    exit 1
fi

if [ -z "$9" ]; then
    echo "Need to supply instance"
    exit 1
fi

if [ -z "${10}" ]; then
    echo "Need to supply price"
    exit 1
fi

AWS_MACHINE_NAME=$1
AWS_ACCESS_KEY=$2
AWS_SECRET_KEY=$3
AWS_VPC_ID=$4
AWS_SG_NAME=$5
AWS_REGION=$6
AWS_ZONE=$7
AWS_SUBNET=$8
INSTANCE_TYPE=$9
SPOT_MAXP=${10}

SUPPORTS_HVM=true
SUPPORTS_EBS_BACKED=true

if [[ $INSTANCE_TYPE == t1* ]]; then
    SUPPORTS_HVM=false
fi

if [[ $INSTANCE_TYPE == t2* ]]; then
    SUPPORTS_EBS_BACKED=false
fi

echo "Instance type supports HVM: ${SUPPORTS_HVM}"

#
# Filling in the remaining regions involves checking out:
# https://cloud-images.ubuntu.com/releases/14.04/release-20150305/
#
if [[ $SUPPORTS_HVM == false ]]; then
    # t1 family
    case "${AWS_REGION}" in
      us-east-1)
        AWS_IMAGE=ami-988ad1f0
        ;;

      *)
        echo "Region ${AWS_REGION} not supported with instance ${INSTANCE_TYPE}"
        exit 1
    esac
else
    if [[ $SUPPORTS_EBS_BACKED == true ]]; then
        # g2, m, and c family
        case "${AWS_REGION}" in
          us-east-1)
            # public liquefy image
            AWS_IMAGE=ami-398bdc53
            ;;

          *)
            echo "Region ${AWS_REGION} not supported with instance ${INSTANCE_TYPE}"
            exit 1
        esac
    else
        # t2 family
        case "${AWS_REGION}" in
          ap-northeast-1)
            AWS_IMAGE=ami-93876e93
            ;;

          ap-southeast-1)
            AWS_IMAGE=ami-66546234
            ;;

          eu-central-1)
            AWS_IMAGE=ami-e2a694ff
            ;;

          eu-west-1)
            AWS_IMAGE=ami-d7fd6ea0
            ;;

          sa-east-1)
            AWS_IMAGE=ami-a357eebe
            ;;

          us-east-1)
            AWS_IMAGE=ami-6089d208
            ;;

          us-west-1)
            AWS_IMAGE=ami-cf7d998b
            ;;

          cn-north-1)
            AWS_IMAGE=ami-d436a4ed
            ;;

          us-gov-west-1)
            AWS_IMAGE=ami-01523322
            ;;

          ap-southeast-2)
            AWS_IMAGE=ami-cd4e3ff7
            ;;

          us-west-2)
            AWS_IMAGE=ami-3b14370b
            ;;

          *)
            echo "Please specify AWS_IMAGE directly (region ${AWS_REGION} not recognized)"
            exit 1
        esac
    fi
fi

AWS_SSH_USER=ubuntu

echo "Provisioning AWS Spot Instance"
echo "${AWS_REGION}"
echo "${INSTANCE_TYPE}"
echo "${AWS_IMAGE}"

docker-machine -D create -d amazonec2 \
    --amazonec2-request-spot-instance \
    --amazonec2-access-key $AWS_ACCESS_KEY \
    --amazonec2-secret-key $AWS_SECRET_KEY \
    --amazonec2-region $AWS_REGION \
    --amazonec2-zone $AWS_ZONE \
    --amazonec2-vpc-id $AWS_VPC_ID \
    --amazonec2-subnet-id $AWS_SUBNET\
    --amazonec2-security-group $AWS_SG_NAME \
    --amazonec2-instance-type $INSTANCE_TYPE \
    --amazonec2-ami $AWS_IMAGE \
    --amazonec2-ssh-user $AWS_SSH_USER \
    --amazonec2-spot-price $SPOT_MAXP \
    $AWS_MACHINE_NAME 2>&1

echo "Done provisioning AWS"
exit 0

