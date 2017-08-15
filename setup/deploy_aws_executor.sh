#!/bin/sh
if [ -z "$1" ]; then
    echo "Need to pass docker-machine master name"
    exit 1
fi

MASTERHOST=$1

MASTER_PUBLIC_IP=$(docker-machine ip $MASTERHOST)
MASTER_PRIVATE_IP=$(docker-machine ssh $MASTERHOST ifconfig eth0 | grep 'inet addr:' | cut -d: -f2 | awk '{ print $1}')

echo "Connecting to host $MASTERHOST"
eval $(docker-machine env $MASTERHOST)

echo "Starting executor server"
docker stop executor
docker rm -f executor
docker run -d \
    --name executor \
    --net=host \
    --privileged \
    -p 4949:4949 \
    liquefy-executor \
    /root/executorServer \
        --publicIp=$MASTER_PUBLIC_IP \
        --privateIp=$MASTER_PRIVATE_IP \
        --executor=/root/executor

exit 0
