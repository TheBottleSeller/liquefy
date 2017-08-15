#!/bin/sh
if [ -z "$1" ]; then
    echo "Need to pass docker-machine name"
    exit 1
fi

MASTERHOST=$1

echo "Building price tracker image on $1"
eval $(docker-machine env $MASTERHOST)
VM_HOME=$(docker-machine ssh $MASTERHOST pwd)

PRICE_TRACKER_OUTPUT=$GOPATH/src/bargain/liquefy/liquidengine/priceTracker/spotPriceTracker

# Delete existing binaries
rm $PRICE_TRACKER_OUTPUT

echo "Building liquefy image on ${MASTERHOST}"
docker build --no-cache -t liquefy-price-tracker ../priceTracker

echo "Copying spot price tracker binary from container to $MASTERHOST"
docker run \
    -v $VM_HOME:/host \
    liquefy-price-tracker \
    cp /root/spotPriceTracker /host/spotPriceTracker

echo "Copy binary from $MASTERHOST to localhost"
docker-machine scp $MASTERHOST:$VM_HOME/spotPriceTracker $PRICE_TRACKER_OUTPUT

exit 0