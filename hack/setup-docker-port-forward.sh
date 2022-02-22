#!/usr/bin/env bash

set -e

echo "Forwarding ports"

# pre-create this to avoid races in the background ssh calls
mkdir -p $HOME/.ssh

# docker
tail -F ssh-log-2375 &
nohup /usr/bin/ssh -i kluctl-ci.pem -o StrictHostKeyChecking=no -L2375:/run/docker.sock -N kluctl-ci@docker.ci.kluctl.io &> ssh-log-2375 &

while ! curl http://localhost:2375 &> /dev/null; do
  echo "Waiting for ports to get available..."
  sleep 5
done

# keep ports alive
nohup bash -c "while true; do curl http://localhost:2375 &> /dev/null ; sleep 5; done" &
