#!/bin/sh
set -e
if [ -z "$1" ]; then
    echo "Need to pass docker-machine master name"
    exit 1
fi

MASTERHOST=$1

if [ ! -z "$2" ]; then
    ./build_common_image.sh $MASTERHOST
    ./build_api_image.sh $MASTERHOST
    ./build_provisioner_image.sh $MASTERHOST
    ./build_executor_image.sh $MASTERHOST
    ./build_scheduler_image.sh $MASTERHOST
fi

./setup_aws_master.sh $MASTERHOST
./deploy_aws_api.sh $MASTERHOST
./deploy_aws_provisioner.sh $MASTERHOST
./deploy_aws_executor.sh $MASTERHOST
./deploy_aws_scheduler.sh $MASTERHOST
