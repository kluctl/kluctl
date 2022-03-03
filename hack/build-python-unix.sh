#!/usr/bin/env bash

set -e

DIR=$(cd $(dirname $0) && pwd)
cd $DIR/..

case "$(uname -s)" in
    Linux*)     os=linux;;
    Darwin*)    os=darwin;;
    MINGW*)     os=windows;;
    *)          echo "unknown os"; exit 1;
esac

mkdir -p build-python/$os
cd build-python/$os

PYTHON_VERSION=3.10.2

if [ ! -d cpython ]; then
  git clone -bv$PYTHON_VERSION --single-branch --depth 1 https://github.com/python/cpython.git cpython
fi

cd cpython

if [ "$os" = "darwin" ]; then
  export CPPFLAGS="-I$(brew --prefix readline)/include"
  export LDFLAGS="-L$(brew --prefix readline)/lib"
fi
./configure $CONFIGURE_FLAGS --enable-shared --disable-test-modules --without-static-libpython --prefix $DIR/../build-python/$os/cpython-install
make -j4
make install

cd ..
cd cpython-install
find . -name __pycache__ -exec rm -rf {} \;
find . -name '*.a' -exec rm {} \;
