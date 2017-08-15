#!/bin/sh
if [ -z "$1" ]; then
    echo "Need to pass docker-machine master name"
    exit 1
fi

MASTER="$1"

MASTER_PUBLIC_IP=$(docker-machine ip $MASTER)
MASTER_PRIVATE_IP=$(docker-machine ssh $MASTER ifconfig eth0 | grep 'inet addr:' | cut -d: -f2 | awk '{ print $1}')

echo "Destroying existing aws slaves"
docker-machine ls | \
    awk '{ print $1 }' | \
    grep "aws-slave-" | \
    while read slave; do
        docker-machine rm $slave 
    done

echo "Destroying existing aws keys"
ls /root/users | \
    while read user; do
        rm -rf /root/users/$user
    done

./setup_ssh_tunnels.sh $MASTER

echo "Starting scheduler locally"
go run \
	$GOPATH/src/bargain/liquefy/entrypoint/scheduler.go \
	-publicIp=$MASTER_PUBLIC_IP \
	-privateIp=127.0.0.1 \
	-setupscripts=$GOPATH/src/bargain/liquefy/setup/ \
	-provider=aws

