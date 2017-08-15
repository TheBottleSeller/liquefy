#!/usr/bin/env bash

#This will make it fail if build yml failes
set -e

if [ -z "$ELASTIC_SEARCH_IP" ]; then
    echo "Need to pass slave id"
    exit 1
fi

#Generate the YML
./generate_filebeatyml --masterip=$ELASTIC_SEARCH_IP

#Start filebeat
filebeat -c "/etc/filebite.yml" -e
