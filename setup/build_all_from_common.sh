#!/bin/sh
if [ -z "$1" ]; then
    echo "Need to pass docker-machine name"
    exit 1
fi

./build_api_image.sh $1 &
./build_scheduler_image.sh $1 &
./build_executor_image.sh $1 &
./build_provisioner_image.sh $1 &


