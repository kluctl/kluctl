#!/usr/bin/env bash

set -e

NETWORK_NAME=$1
MEMBER_NAME=$2

while true; do
  echo "Searching network"
  NETWORKS=$(curl -H"Authorization: bearer $CI_ZEROTIER_API_KEY" https://my.zerotier.com/api/v1/network)
  NETWORK_ID=$(echo $NETWORKS | jq ".[] | select(.config.name == \"$NETWORK_NAME\") | .config.id" -r)
  if [ "$NETWORK_ID" != "" ]; then
    break
  fi
  sleep 5
done

SUDO=
if which sudo &> /dev/null; then
  SUDO=sudo
fi

echo "Joining network"
$SUDO zerotier-cli join $NETWORK_ID
MEMBER_ID=$($SUDO zerotier-cli status | awk '{print $3}')

echo "Renaming member $MEMBER_ID"
curl -H"Authorization: bearer $CI_ZEROTIER_API_KEY" -XPOST -d"{\"name\": \"$MEMBER_NAME\"}" https://my.zerotier.com/api/v1/network/$NETWORK_ID/member/$MEMBER_ID
