#!/usr/bin/env bash

set -e

DIR=$(cd $(dirname $0) && pwd)
cd $DIR/..

if [ "$1" != "" ]; then
  os=$1
else
  case "$(uname -s)" in
      Linux*)     os=linux;;
      Darwin*)    os=darwin;;
      MINGW*)     os=windows;;
      *)          echo "unknown os"; exit 1;
  esac
fi

arch=x86_64

case "$os" in
    linux*)     python_dist=unknown-linux-gnu-pgo-full;;
    darwin*)    python_dist=apple-darwin-pgo-full;;
    windows*)   python_dist=pc-windows-msvc-shared-pgo-full;;
esac

mkdir -p download-python/$os
cd download-python/$os

PYTHON_STANDALONE_VERSION=20220227
PYTHON_VERSION=3.10.2

if [ ! -d python ]; then
  curl -L -o python.tar.zst https://github.com/indygreg/python-build-standalone/releases/download/$PYTHON_STANDALONE_VERSION/cpython-$PYTHON_VERSION+$PYTHON_STANDALONE_VERSION-$arch-$python_dist.tar.zst
  tar -xf python.tar.zst
fi

cd python/install

for i in test site-packages venv ensurepip idlelib distutils pydoc_data asyncio email tkinter lib2to3 xml multiprocessing unittest; do
  rm -rf lib/python3.*/$i
  rm -rf Lib/$i
done
