#!/bin/sh

if [ -z "$1" ]; then
	MASTERHOST="vbox-master"
else
	MASTERHOST="$1"
fi

API_SERVER_HOME="$GOPATH/src/bargain/web/apiserver"

cd $API_SERVER_HOME
npm install
npm start $(docker-machine ip $MASTERHOST)