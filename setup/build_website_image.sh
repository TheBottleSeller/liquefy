#!/bin/sh
if [ -z "$1" ]; then
    echo "Need to pass docker-machine name"
    exit 1
fi

echo "Building website image on $1"
eval $(docker-machine env $1)
docker build --no-cache -t liquefy-website ../../web/website
