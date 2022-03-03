#!/usr/bin/env bash

set -e

DIR=$(cd $(dirname $0) && pwd)

cd $DIR/../../build-python/$2

tar --exclude '*/__pycache__' --exclude '*.a' -czf $DIR/$1 $3
