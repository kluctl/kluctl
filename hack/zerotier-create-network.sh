#!/usr/bin/env bash

set -e

NETWORK_NAME=$1

NETWORK=$(curl -H"Authorization: bearer $CI_ZEROTIER_API_KEY" -XPOST -d"{}" https://my.zerotier.com/api/v1/network)
NETWORK_ID=$(echo $NETWORK | jq '.config.id' -r)

echo "Configuring network"
cat << EOF | curl -H"Authorization: bearer $CI_ZEROTIER_API_KEY" -XPOST -d@- https://my.zerotier.com/api/v1/network/$NETWORK_ID
{
  "config": {
    "private": false,
    "name": "$NETWORK_NAME",
    "ipAssignmentPools": [
      {
        "ipRangeStart":"10.147.17.1",
        "ipRangeEnd":"10.147.17.254"
      }
    ],
    "routes": [
      {
        "target":"10.147.17.0/24"
      }
    ]
  }
}
EOF
