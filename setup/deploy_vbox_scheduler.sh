#!/bin/sh

if [ -z "$1" ]; then
    MASTERHOST="vbox-master"
else
    MASTERHOST="$1"
fi

BUILD_EXECUTOR=$2

MASTER_IP=$(docker-machine ip $MASTERHOST)
LOCAL_IP=$(ifconfig en0 | grep 'inet ' | cut -d: -f2 | awk '{ print $2}')

# Start mesos scheduler
echo "Starting scheduler"
go run ../scheduler/entrypoint/scheduler.go \
        --publicIp=$MASTER_IP \
        --schedIp=$LOCAL_IP \
        --provider=vbox

exit 0
