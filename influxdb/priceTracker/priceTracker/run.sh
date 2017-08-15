#!/bin/sh

if [ -z "${INFLUXDB_HOST}" ]; then
	echo "Need to pass in the INFLUXDB_HOST"
	exit 1
fi

sed -i "s/localhost/${INFLUXDB_HOST}/g" /config/telegraf.toml

exec /opt/telegraf/telegraf -config /config/telegraf.toml
