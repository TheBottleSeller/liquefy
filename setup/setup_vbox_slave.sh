#!/bin/sh
if [ -z "$1" ]; then
    echo "Need to pass docker-machine slave name"
    exit 1
fi

if [ -z "$2" ]; then
    echo "Need to pass slave id"
    exit 1
fi

if [ -z "$3" ]; then
    echo "Need to pass master ip"
    exit 1
fi

if [ -z "$4" ]; then
    echo "Need to pass executor path"
    exit 1
fi

SLAVEHOST=$1
SLAVEID=$2
MASTER_IP=$3
EXECUTOR_PATH=$4

ATTRIBUTES="liquefyid:${SLAVEID}"

SLAVE_HOME=$(docker-machine ssh $SLAVEHOST pwd)
EXECUTOR_SLAVE=$SLAVE_HOME/executor
EXECUTOR_CONTAINER=/home/liquefy/executor

# scp executor from master to slave
#echo "Copy executor binary from master to slave"
#docker-machine scp $EXECUTOR_PATH $SLAVEHOST:$EXECUTOR_SLAVE

SLAVE_IP=$(docker-machine ip $SLAVEHOST)

echo "Connecting to slave"
eval $(docker-machine env $SLAVEHOST)

echo "Run mesos slave"
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
    -e MESOS_CONTAINERIZERS=mesos,docker \
    -e MESOS_DOCKER_MESOS_IMAGE=mesosphere/mesos-slave:0.25.0-0.2.70.ubuntu1404 \
    -e MESOS_PORT=5051 \
    -e MESOS_IP=${SLAVE_IP} \
    -e MESOS_SWITCH_USER=false \
    -e MESOS_ATTRIBUTES=$ATTRIBUTES \
    -e MESOS_EXECUTOR_REGISTRATION_TIMEOUT=5mins \
    -v /lib/libpthread.so.0:/lib/libpthread.so.0:ro \
    -v /usr/local/bin/docker:/usr/bin/docker:ro \
    -v /var/run/docker.sock:/var/run/docker.sock:ro \
    -v $EXECUTOR_SLAVE:$EXECUTOR_CONTAINER \
    -v /sys:/sys:ro \
    -p 5051:5051 \
    mesosphere/mesos-slave:0.25.0-0.2.70.ubuntu1404


# Build the filebeat logger container
# TODO : Remove and make it be a private registy
docker build -t filebeatlogger ../slave/logging

docker stop logger
docker rm logger
docker run -d \
    --name=logger \
    --net=host \
    --privileged \
    -e ELASTIC_SEARCH_IP=${MASTER_IP} \
    -v /usr/local/bin/docker:/usr/bin/docker:ro \
    -v /var/run/docker.sock:/var/run/docker.sock:ro \
    -v /var/lib/docker/containers/:/var/lib/docker/containers/ \
    filebeatlogger

exit 0