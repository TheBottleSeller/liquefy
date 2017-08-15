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

echo "Starting provisioner service"
docker stop provisioner
docker rm -f provisioner
docker run -d \
    --name provisioner \
    -e ENV=${DEPLOY_ENV} \
    -e DB_USER="liquiddev" \
    -e DB_NAME="liquiddev" \
    -e REDIS_URL="redis://$MASTER_PRIVATE_IP:6379/0" \
    liquefy-provisioner \
    /root/provisionerService \
        --mesosMasterIp=$MASTER_PUBLIC_IP \
        --dbIp=$MASTER_PRIVATE_IP \
        --esIp=$MASTER_PRIVATE_IP

exit 0

