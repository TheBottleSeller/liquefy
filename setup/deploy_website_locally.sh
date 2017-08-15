#!/bin/sh
if [ -z "$1" ]; then
    echo "Need to pass docker-machine master name"
    exit 1
fi

MASTERHOST=$1
cd $GOPATH/src/bargain/web/website
./node_modules/.bin/webpack --watch --progress &
npm start $(docker-machine ip $MASTERHOST) $(docker-machine ip $MASTERHOST)