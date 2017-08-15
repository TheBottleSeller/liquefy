#!/bin/sh
if [ -z "$1" ]; then
    echo "Need to supply a docker-machine name"
    exit 1
fi
psql -h $(docker-machine ip $1) -U liquiddev
