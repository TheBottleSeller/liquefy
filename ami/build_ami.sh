#!/bin/sh
if [ -z "$1" ]; then
    echo "Need to pass aws access key"
    exit 1
fi

if [ -z "$2" ]; then
    echo "Need to pass aws secret key"
    exit 1
fi

AWS_ACCESS_KEY="$1"
AWS_SECRET_KEY="$2"

packer build \
	-var "aws_access_key=$AWS_ACCESS_KEY" \
	-var "aws_secret_key=$AWS_SECRET_KEY" \
	liquefy-ami.json