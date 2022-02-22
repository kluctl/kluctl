#!/usr/bin/env bash

set -e

DIR=$(cd $(dirname $0) && pwd)
cd $DIR/..

mkdir -p build-python/windows
cd build-python/windows

PYTHON_VERSION=3.10.2

curl -L -o python.zip https://www.python.org/ftp/python/$PYTHON_VERSION/python-$PYTHON_VERSION-embed-amd64.zip
unzip -o python.zip
rm python.zip

tar czf $DIR/../pkg/python/python-lib-windows.tar.gz *
