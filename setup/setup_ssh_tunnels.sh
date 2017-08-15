#!/bin/sh
if [ -z "$1" ]; then
    echo "Need to pass docker-machine master name"
    exit 1
fi

MASTER="$1"

MASTER_PUBLIC_IP=$(docker-machine ip $MASTER)
MASTER_PRIVATE_IP=$(docker-machine ssh $MASTER ifconfig eth0 | grep 'inet addr:' | cut -d: -f2 | awk '{ print $1}')

EXIST_SSH_TUNNEL_PID=$(ps aux | \
	grep "ssh \-fN \-i" | \
	grep "\-R 9090:127.0.0.1:9090" | \
	grep $MASTER_PUBLIC_IP | \
	awk '{ print $2 }')
if [ ! -z $EXIST_SSH_TUNNEL_PID ]; then
	echo "Terminating SSH tunnel that already"
	kill -9 $EXIST_SSH_TUNNEL_PID
fi

echo "Starting SSH tunnel"
ssh -fN -i $HOME/.docker/machine/machines/$MASTER/id_rsa -R 9090:127.0.0.1:9090 ubuntu@$MASTER_PUBLIC_IP

