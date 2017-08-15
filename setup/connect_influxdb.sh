#!/bin/sh

if [ -z "$1" ]; then
    echo "Need to pass docker-machine name"
    exit 1
fi

MACHINE=$1

eval $(docker-machine env ${MACHINE})
docker exec -it influxdb influx
