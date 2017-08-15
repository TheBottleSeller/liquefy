#!/bin/sh
if [ -z "$1" ]; then
    echo "Need to pass docker-machine master name"
    exit 1
fi

DEPLOY_ENV=$2

if [ -z $DEPLOY_ENV ]; then
    DEPLOY_ENV="STAGING"
fi


MASTERHOST=$1
MASTER_PUBLIC_IP=$(docker-machine ip $MASTERHOST)
MASTER_PRIVATE_IP=$(docker-machine ssh $MASTERHOST ifconfig eth0 | grep 'inet addr:' | cut -d: -f2 | awk '{ print $1}')


echo "Connecting to host $MASTERHOST"
eval $(docker-machine env $MASTERHOST)

echo "Starting API Server"
docker stop api
docker rm -f api
docker run -d \
    --name api \
    --net=host \
    -p 3030:3030 \
    -e ENV=${DEPLOY_ENV} \
    -e DB_USER="liquiddev" \
    -e DB_NAME="liquiddev" \
    liquefy-api \
    /root/apiServer \
        --dbIp=$MASTER_PRIVATE_IP \
        --esIp=$MASTER_PRIVATE_IP \
