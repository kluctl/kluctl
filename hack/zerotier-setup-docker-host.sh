#!/usr/bin/env bash

set -e

NETWORK_NAME=$1

echo "Searching network "
NETWORKS=$(curl -H"Authorization: bearer $CI_ZEROTIER_API_KEY" https://my.zerotier.com/api/v1/network)
NETWORK_ID=$(echo $NETWORKS | jq ".[] | select(.config.name == \"$NETWORK_NAME\") | .config.id" -r)

echo "Waiting for IP to be assigned to docker member"
while true; do
  MEMBERS=$(curl -H"Authorization: bearer $CI_ZEROTIER_API_KEY" https://my.zerotier.com/api/v1/network/$NETWORK_ID/member)
  IP=$(echo $MEMBERS | jq ".[] | select(.name == \"docker\") | .config.ipAssignments[0]" -r)
  if [ "$IP" != "null" ]; then
    break
  fi
  sleep 5
  echo "Still waiting..."
done

echo "DOCKER_IP=$IP" >> $GITHUB_ENV
echo "DOCKER_HOST=tcp://$IP:2375" >> $GITHUB_ENV

echo "Waiting for docker to become available"
while ! nc -z $IP 2375; do
  sleep 5
  echo "Still waiting..."
done
