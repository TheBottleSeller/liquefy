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

VM_HOME=$(docker-machine ssh $MASTERHOST pwd)

echo "Connecting to host $MASTERHOST"
eval $(docker-machine env $MASTERHOST)

echo "Starting scheduler"
docker stop scheduler
docker rm -f scheduler
docker run -d \
    --name scheduler \
    --net=host \
    -e ENV=${DEPLOY_ENV} \
    -e DB_USER="liquiddev" \
    -e DB_NAME="liquiddev" \
    -e REDIS_URL="redis://$MASTER_PRIVATE_IP:6379/0" \
    -v /usr/local/bin:/usr/local/bin \
    -v /lib/x86_64-linux-gnu:/lib/x86_64-linux-gnu:ro \
    -v /lib/usr/x86_64-linux-gnu:/lib/usr/x86_64-linux-gnu:ro \
    -v /usr/lib/x86_64-linux-gnu/:/usr/lib/x86_64-linux-gnu/:ro \
    -v /usr/bin/docker:/usr/bin/docker:ro \
    -v /var/run/docker.sock:/var/run/docker.sock:ro \
    -v /sys:/sys:ro \
    -p 3000:3000 \
    liquefy-scheduler \
    /root/schedulerService \
        --mesosMasterIp=$MASTER_PUBLIC_IP \
        --schedIp=$MASTER_PRIVATE_IP \
        --executorIp=$MASTER_PUBLIC_IP \
        --dbIp=$MASTER_PRIVATE_IP \
        --esIp=$MASTER_PRIVATE_IP

exit 0
