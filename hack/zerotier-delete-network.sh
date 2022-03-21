#!/usr/bin/env bash

set -e

NETWORK_NAME=$1

echo "Searching network "
NETWORKS=$(curl -H"Authorization: bearer $CI_ZEROTIER_API_KEY" https://my.zerotier.com/api/v1/network)
NETWORK_ID=$(echo $NETWORKS | jq ".[] | select(.config.name == \"$NETWORK_NAME\") | .config.id" -r)

if [ "$NETWORK_ID" = "" ]; then
  exit 0
fi

echo "Deleting network"
curl -H"Authorization: bearer $CI_ZEROTIER_API_KEY" -XDELETE https://my.zerotier.com/api/v1/network/$NETWORK_ID
