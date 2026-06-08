#!/usr/bin/env bash

echo "starting node cleanup"
echo "current disk usage"
df -h

rm -rf /usr/share/dotnet /usr/local/lib/android /opt/ghc /opt/hostedtoolcache/CodeQL
echo "disk space after rm -rf"
df -h
