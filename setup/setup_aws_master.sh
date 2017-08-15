#!/bin/sh
if [ -z "$1" ]; then
    echo "Need to pass docker-machine master name"
    exit 1
fi

MASTERHOST=$1

# Connect to master docker host
echo "Connecting to host $MASTERHOST"
eval $(docker-machine env $MASTERHOST)

VM_HOME=$(docker-machine ssh $MASTERHOST pwd)

MASTER_PUBLIC_IP=$2
MASTER_PRIVATE_IP=$3

if [ -z "$MASTER_PUBLIC_IP" ]; then
    MASTER_PUBLIC_IP=$(docker-machine ip $MASTERHOST)
fi
if [ -z "$MASTER_PRIVATE_IP" ]; then
    MASTER_PRIVATE_IP=$(docker-machine ssh $MASTERHOST ifconfig eth0 | grep 'inet addr:' | cut -d: -f2 | awk '{ print $1}')
fi

# Run zookeeper
echo "Starting zookeeper"
docker stop zookeeper
docker rm -f zookeeper
docker run -d \
    --name zookeeper \
    --restart always \
    --net=host \
    -p 2181:2181 \
    -p 2888:2888 \
    -p 3888:3888 \
    jplock/zookeeper:3.4.6

# Run mesos master
echo "Starting mesos master"
docker stop mesos-master
docker rm -f mesos-master
docker run -d \
    --name mesos-master \
	--net host \
    --restart always \
    -e MESOS_ADVERTISE_IP=$MASTER_PUBLIC_IP \
	-e MESOS_IP=$MASTER_PRIVATE_IP \
	-e MESOS_ZK=zk://$MASTER_PRIVATE_IP:2181/mesos \
	-e MESOS_PORT=5050 \
	-e MESOS_LOG_DIR=/var/log/mesos \
	-e MESOS_WORK_DIR=/var/lib/mesos \
	-e MESOS_QUORUM=1 \
	-p 5050:5050 \
    mesosphere/mesos-master:0.25.0-0.2.70.ubuntu1404

# Start dummy slave
echo "Starting dummy mesos slave"
docker stop mesos-slave
docker rm -f mesos-slave
docker run -d \
    --name=mesos-slave \
    --net=host \
    --restart=always \
    --privileged \
    -e MESOS_LOG_DIR=/var/log \
    -e MESOS_WORK_DIR=/var/lib/mesos/slave \
    -e MESOS_MASTER=zk://${MASTER_PRIVATE_IP}:2181/mesos \
    -e MESOS_ISOLATOR=cgroups/cpu,cgroups/mem \
    -e MESOS_CONTAINERIZERS=mesos \
    -e MESOS_DOCKER_MESOS_IMAGE=mesosphere/mesos-slave:0.25.0-0.2.70.ubuntu1404 \
    -e MESOS_PORT=5051 \
    -e MESOS_IP=${MASTER_PRIVATE_IP} \
    -e MESOS_SWITCH_USER=false \
    -e MESOS_RESOURCES="cpus:0.01;mem:10" \
    -p 5051:5051 \
    mesosphere/mesos-slave:0.25.0-0.2.70.ubuntu1404

echo "Starting elastic search"
docker stop elastic-search
docker rm -f elastic-search
docker run -d \
    --name elastic-search \
    --net=host \
    -p 9200:9200 \
    elasticsearch

echo "Starting Kibana Log Dashboard"
docker stop kibana
docker rm -f kibana
docker run -d \
    --name kibana \
    -e ELASTICSEARCH_URL=http://${MASTER_PRIVATE_IP}:9200 \
    -p 5601:5601 \
    kibana

docker stop logger
docker rm -f logger
docker run -d \
    --name logger \
    --net=host \
    -e ELASTIC_SEARCH_IP=${MASTER_PRIVATE_IP} \
    -v /usr/local/bin/docker:/usr/bin/docker:ro \
    -v /var/run/docker.sock:/var/run/docker.sock:ro \
    -v /var/lib/docker/containers/:/var/lib/docker/containers/ \
    liquefy/logger

echo "Starting Redis"
docker stop redis
docker rm -f redis
docker run --name redis -p 6379:6379 -d redis

exit 0
