#!/bin/sh
if [ -z "$1" ]; then
    MASTERHOST="vbox-master"
else
    MASTERHOST="$1"
fi

if [ ! -z "$2" ]; then
    echo "Force shutdown $MASTERHOST"
    VBoxManage controlvm $MASTERHOST poweroff
fi

echo "Checking if master is running"
docker-machine ssh $MASTERHOST "exit 0"
if [[ $? != 0 ]]; then
    echo "Restarting master"
    VBoxManage controlvm $MASTERHOST poweroff
    docker-machine start $MASTERHOST
fi

# Get docker machine ip addresses
MASTER_IP=$(docker-machine ip $MASTERHOST)
VM_HOME=$(docker-machine ssh $MASTERHOST pwd)
EXECUTOR_VM=$VM_HOME/executor
EXECUTOR_CONTAINER=/home/liquefy/executor

echo "Stopping all slaves"
docker-machine ls | \
    grep Running | \
    grep vbox-slave- | \
    awk '{ print $1 }' | \
    while read slave; do \
        VBoxManage controlvm $slave poweroff; \
    done

echo "Connecting to host $MASTERHOST"
eval $(docker-machine env ${MASTERHOST})

echo "Starting postgres"
docker stop postgres
docker rm postgres
docker run -d \
    --name postgres \
    -e POSTGRES_USER=liquiddev \
    -p 5432:5432 \
    postgres

echo "Starting zookeeper"
docker stop zookeeper
docker rm zookeeper
docker run -d --name=zookeeper -p 2181:2181 -p 2888:2888 -p 3888:3888 jplock/zookeeper:3.4.6

echo "Starting mesos master"
docker stop mesos-master
docker rm mesos-master
docker run -d \
    --name=mesos-master \
    --net=host \
    --privileged \
    -e MESOS_IP=$MASTER_IP \
    -e MESOS_ZK=zk://$MASTER_IP:2181/mesos \
    -e MESOS_PORT=5050 \
    -e MESOS_LOG_DIR=/var/log/mesos \
    -e MESOS_WORK_DIR=/var/lib/mesos \
    -e MESOS_QUORUM=1 \
    -p 5050:5050 \
    -v $EXECUTOR_VM:$EXECUTOR_CONTAINER \
    mesosphere/mesos-master:0.25.0-0.2.70.ubuntu1404

echo "Starting dummy mesos slave"
docker stop mesos-slave
docker rm mesos-slave
docker run -d \
    --name=mesos-slave \
    --net=host \
    --privileged \
    -e MESOS_LOG_DIR=/var/log \
    -e MESOS_WORK_DIR=/var/lib/mesos/slave \
    -e MESOS_MASTER=zk://${MASTER_IP}:2181/mesos \
    -e MESOS_ISOLATOR=cgroups/cpu,cgroups/mem \
    -e MESOS_CONTAINERIZERS=docker \
    -e MESOS_DOCKER_MESOS_IMAGE=mesosphere/mesos-slave:0.25.0-0.2.70.ubuntu1404 \
    -e MESOS_PORT=5051 \
    -e MESOS_IP=${MASTER_IP} \
    -e MESOS_SWITCH_USER=false \
    -e MESOS_RESOURCES="cpus:0.01;mem:1" \
    -v /lib/libpthread.so.0:/lib/libpthread.so.0:ro \
    -v /usr/local/bin/docker:/usr/bin/docker:ro \
    -v /var/run/docker.sock:/var/run/docker.sock:ro \
    -v $EXECUTOR_VM:$EXECUTOR_CONTAINER \
    -v /sys:/sys:ro \
    -p 5051:5051 \
    mesosphere/mesos-slave:0.25.0-0.2.70.ubuntu1404

echo "Starting elastic search"
docker stop elastic-search
docker rm elastic-search
docker run -d \
    --name=elastic-search \
    --net=host \
    -p 9200:9200 \
    elasticsearch

echo "Initializing the db"
go run ../initDB.go --masterip=$MASTER_IP

exit 0