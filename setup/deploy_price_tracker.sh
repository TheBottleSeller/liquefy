#!/bin/sh

if [ -z "$1" ]; then
    echo "Need to pass docker-machine master name"
    exit 1
fi

PRICE_TRACKER_PATH=$GOPATH/src/bargain/liquefy/liquidengine/priceTracker

MACHINE="$1"
MACHINE_CLOUD=$(docker-machine inspect -f {{.DriverName}} $MACHINE)
MACHINE_IP=$(docker-machine ip $MACHINE)

if [[ "${MACHINE_CLOUD}" == "amazonec2" ]]; then
	MACHINE_PRIVATE_IP=$(docker-machine inspect -f {{.Driver.PrivateIPAddress}} $MACHINE)
elif [[ "${MACHINE_CLOUD}" == "virtualbox" ]]; then
	MACHINE_PRIVATE_IP=$MACHINE_IP
else
	echo "The cloud ${MACHINE_CLOUD} is not supported"
	exit 1
fi

VM_HOME=$(docker-machine ssh $MACHINE 'echo $PWD')
VM_INFLUXDB_PATH="/mnt/influxdb"

./build_price-tracker_image.sh $MACHINE

cd ${PRICE_TRACKER_PATH}

echo "Connecting to ${MACHINE}"
eval $(docker-machine env $MACHINE)

echo "Starting influxdb"
docker stop influxdb
docker rm influxdb
docker-machine ssh $MACHINE "mkdir ${VM_INFLUXDB_PATH}"
docker run -d --name influxdb -v ${VM_INFLUXDB_PATH}:/data -p 8083:8083 -p 8086:8086 tutum/influxdb

echo "Building telegraf price tracker"
docker build -t liquefy-telegraf .

echo "Starting telegraf price tracker"
docker stop telegraf
docker rm telegraf
docker run -d --name telegraf -e INFLUXDB_HOST=${MACHINE_PRIVATE_IP} liquefy-telegraf

echo "Starting chronograf"
docker stop chronograf
docker rm chronograf
docker run -d --name chronograf -p 81:80 lukasmartinelli/chronograf /opt/chronograf/chronograf -bind 0.0.0.0:80

echo "Done deploying the price tracker. See ${MACHINE_IP}:81 to view the AWS Spot Prices data in Chronograf."
echo "NOTE: Set the database host to ${MACHINE_PRIVATE_IP}:8086"
