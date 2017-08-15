#!/bin/sh
if [ -z "$1" ]; then
    echo "Need to pass docker-machine master name"
    exit 1
fi

MASTERHOST=$1
MASTER_IP=$2
MASTER_PRIVATE_IP=$3

if [ -z "$MASTER_IP" ]; then
    MASTER_IP=$(docker-machine ip $MASTERHOST)
fi

if [ -z "$MASTER_PRIVATE_IP" ]; then
    MASTER_PRIVATE_IP=$(docker-machine ssh $MASTERHOST ifconfig eth0 | grep 'inet addr:' | cut -d: -f2 | awk '{ print $1}')
fi


echo "Connecting to $MASTERHOST"
eval $(docker-machine env $MASTERHOST)

echo "Running website server"
docker stop liquefy-website
docker rm -f liquefy-website
docker run -d \
	--name liquefy-website \
	--net=host \
    -e POSTGRES_HOST=$MASTER_PRIVATE_IP \
    -e SCHED_HOST=$MASTER_PRIVATE_IP \
	--restart always \
	-p 80:80 \
	liquefy-website
