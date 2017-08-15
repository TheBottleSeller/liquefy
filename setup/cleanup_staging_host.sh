#!/bin/sh

if [ -z "$1" ]; then
    echo "Need to pass docker-machine name"
    exit 1
fi

HOST="$1"

STOP_CONTAINERS="docker ps -a -q | xargs -L1 docker stop"
RM_CONTAINERS="docker ps -a -q | xargs -L1 docker rm"
RMI_NONE_IMAGES="docker images | grep none | awk '{ print $3 }' | xargs -L1 docker rmi"

docker-machine ssh $HOST "$STOP_CONTAINERS"
docker-machine ssh $HOST "$RM_CONTAINERS"
docker-machine ssh $HOST "$RMI_NONE_IMAGES"