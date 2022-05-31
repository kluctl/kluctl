#!/usr/bin/env bash

set -e

NAME=$1
IP=$2
PORT=$3
export KUBECONFIG=$4

cat << EOF > kind-cluster-$NAME.yml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  apiServerAddress: "0.0.0.0"
  apiServerPort: $PORT
EOF

rm -f $(pwd)/kind-kubeconfig
kind create cluster --config kind-cluster-$NAME.yml --name $NAME

# Rewrite cluster info to point to docker host
# This also fully disables TLS verification
kubectl config view -ojson --raw \
  | jq ".clusters[0].cluster.\"insecure-skip-tls-verify\"=true" \
  | jq "del(.clusters[0].cluster.\"certificate-authority-data\")" \
  | jq ".clusters[0].cluster.server=\"https://$IP:$PORT\"" \
> $KUBECONFIG-tmp
mv $KUBECONFIG-tmp $KUBECONFIG
