#!/bin/sh
if [ -z "$1" ]; then
	DB_IP="localhost"
else
	DB_IP=$(docker-machine ip $1)
fi

go run $GOPATH/src/bargain/liquefy/api/entrypoint/apiService.go --dbIp=$DB_IP
