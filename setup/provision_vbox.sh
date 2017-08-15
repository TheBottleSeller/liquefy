#!/bin/sh
if [ -z "$1" ]; then
    echo "Need to supply a docker-machine name"
    exit 1
fi

DOCKER_MACHINE_NAME=$1

# Check if machine exists
EXISTS=$(docker-machine ls | awk '{ print $1 }' | grep $DOCKER_MACHINE_NAME)
echo $EXISTS
if [ ! -z $EXISTS ]; then
    echo "Docker host $DOCKER_MACHINE_NAME exists. Restarting"
    VBoxManage controlvm $DOCKER_MACHINE_NAME poweroff
    docker-machine start $DOCKER_MACHINE_NAME
else
    echo "Creating host $DOCKER_MACHINE_NAME"
    docker-machine create -d virtualbox $DOCKER_MACHINE_NAME
fi

docker-machine ssh $DOCKER_MACHINE_NAME "exit 0"
echo "Docker host $DOCKER_MACHINE_NAME is provisioned"
exit 0