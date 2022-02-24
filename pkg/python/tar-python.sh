#!/usr/bin/env bash

set -e

DIR=$(cd $(dirname $0) && pwd)

cd $DIR/../../build-python/$2

tar czf $DIR/$1 $3
