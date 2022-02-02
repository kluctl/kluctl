#!/usr/bin/env bash

DIR=$(cd $(dirname $0) && pwd)
cd $DIR/..

protoc -I=jinja2-server --go_out=pkg --go-grpc_out=pkg/jinja2_server --go-grpc_opt=paths=source_relative ./jinja2-server/jinja2_server.proto
