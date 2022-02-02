#!/usr/bin/env bash

DIR=$(cd $(dirname $0) && pwd)
cd $DIR

export PATH=$DIR/venv/bin:$PATH

protoc -I=. --python_out=. ./jinja2_server.proto
python -m grpc_tools.protoc -I=. --python_out=. --grpc_python_out=. jinja2_server.proto
