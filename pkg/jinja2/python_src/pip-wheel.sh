#!/usr/bin/env bash

DIR=$(cd $(dirname $0) && pwd)

mkdir -p $DIR/wheel
cd $DIR/wheel

rm *.whl
pip3 wheel -r ../requirements.txt

for f in *.whl; do
  unzip $f
  rm $f
done
