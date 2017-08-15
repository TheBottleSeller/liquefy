#!/bin/sh
if [ -z "$1" ]; then
    echo "Need to pass docker-machine master name"
    exit 1
fi

MASTERHOST=$1
MASTER_PUBLIC_IP=$2

if [ -z "$MASTER_PUBLIC_IP" ]; then
    MASTER_PUBLIC_IP=$(docker-machine ip $MASTERHOST)
fi

# Connect to master docker host
echo "Connecting to host $MASTERHOST"
eval $(docker-machine env $MASTERHOST)

# Run postgres
echo "Starting postgres"
docker stop postgres
docker rm -f postgres
docker run -d \
    --name postgres \
    -e POSTGRES_USER=liquiddev \
    -p 5432:5432 \
    -v /home/ubuntu/postgres:/var/lib/postgresql/data \
    postgres

echo "Initializing the db"
go run ../initDB.go --masterip=${MASTER_PUBLIC_IP}