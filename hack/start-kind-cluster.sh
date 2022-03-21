#!/usr/bin/env bash

set -e

NAME=$1
IP=$2
PORT=$3

cat << EOF > kind-cluster.yml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  apiServerAddress: "0.0.0.0"
  apiServerPort: $PORT
EOF

rm -f $(pwd)/kind-kubeconfig
export KUBECONFIG=$KIND_KUBECONFIG
kind delete cluster --name $NAME || true
kind create cluster --config kind-cluster.yml --name $NAME

# Rewrite cluster info to point to docker host
# This also fully disables TLS verification
kubectl config view -ojson --raw \
  | jq ".clusters[0].cluster.\"insecure-skip-tls-verify\"=true" \
  | jq "del(.clusters[0].cluster.\"certificate-authority-data\")" \
  | jq ".clusters[0].cluster.server=\"https://$IP:$PORT\"" \
> kind-kubeconfig2
mv kind-kubeconfig2 kind-kubeconfig
