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
    echo "Need to pass cpu total"
    exit 1
fi

if [ -z "$5" ]; then
    echo "Need to pass mem total (mb)"
    exit 1
fi

SLAVEHOST=$1
SLAVEID=$2
MASTER_IP=$3
CPU_TOTAL=$4
MEM_TOTAL=$5

ATTRIBUTES="liquefyid:${SLAVEID}"
RESOURCES="cpus:$CPU_TOTAL;mem:$MEM_TOTAL"

#
# TODO: FIX THIS SHIT
#
# if [ "$GPU_COUNT" -eq "0" ]; then
#     RESOURCES="gpus:{}"
# elif [ "$GPU_COUNT" -eq "1" ]; then
#     RESOURCES="gpus:{ gpu1 }"
# else
#     RESOURCES="gpus:{ gpu1"
#     for i in `seq 2 $GPU_COUNT`;
#     do
#         RESOURCES="$RESOURCES, gpu$i"
#     done
#     RESOURCES="$RESOURCES }"
# fi
#

echo "Connecting to $SLAVEHOST"
eval $(docker-machine env $SLAVEHOST --shell /bin/sh)

# Get ip addresses
SLAVE_PRIVATE_IP=$(docker-machine ssh $SLAVEHOST ifconfig eth0 | grep 'inet addr:' | cut -d: -f2 | awk '{ print $1}')
SLAVE_PUBLIC_IP=$(docker-machine ip $SLAVEHOST)

echo "Starting logger"
docker run -d \
    --name=logger \
    --net=host \
    -e ELASTIC_SEARCH_IP=${MASTER_IP} \
    -v /usr/local/bin/docker:/usr/bin/docker:ro \
    -v /var/run/docker.sock:/var/run/docker.sock:ro \
    -v /var/lib/docker/containers/:/var/lib/docker/containers/ \
    -v /var/lib/mesos/:/var/lib/mesos/ \
    liquefy/logger:latest 2>&1

# Run mesos slave
echo "Starting mesos slave"
docker run -d \
    --name=mesos-slave \
    --net=host \
    --privileged \
    -e MESOS_LOG_DIR=/var/log \
    -e MESOS_WORK_DIR=/var/lib/mesos/slave \
    -e MESOS_MASTER=zk://${MASTER_IP}:2181/mesos \
    -e MESOS_ISOLATOR=cgroups/cpu,cgroups/mem \
    -e MESOS_CONTAINERIZERS=mesos \
    -e MESOS_DOCKER_MESOS_IMAGE=mesosphere/mesos-slave:0.25.0-0.2.70.ubuntu1404 \
    -e MESOS_PORT=5051 \
    -e LIBPROCESS_ADVERTISE_IP=${SLAVE_PUBLIC_IP} \
    -e MESOS_IP=${SLAVE_PRIVATE_IP} \
    -e MESOS_HOSTNAME=${SLAVE_PUBLIC_IP} \
    -e MESOS_SWITCH_USER=false \
    -e MESOS_EXECUTOR_REGISTRATION_TIMEOUT=5mins \
    -e MESOS_ATTRIBUTES=$ATTRIBUTES \
    -e MESOS_RESOURCES=$RESOURCES \
    -v /lib/libpthread.so.0:/lib/libpthread.so.0:ro \
    -v /lib/x86_64-linux-gnu:/lib/x86_64-linux-gnu:ro \
    -v /lib/usr/x86_64-linux-gnu:/lib/usr/x86_64-linux-gnu:ro \
    -v /usr/lib/x86_64-linux-gnu:/usr/lib/x86_64-linux-gnu:ro \
    -v /usr/bin/docker:/usr/bin/docker:ro \
    -v /var/run/docker.sock:/var/run/docker.sock:ro \
    -v /sys:/sys:ro \
    -v /var/lib/mesos/:/var/lib/mesos/ \
    -p 5051:5051 \
    mesosphere/mesos-slave:0.25.0-0.2.70.ubuntu1404 2>&1

exit 0
