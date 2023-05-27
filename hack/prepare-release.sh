#!/bin/sh

set -e

VERSION=$1

VERSION_REGEX='v([0-9]*)\.([0-9]*)\.([0-9]*)'
VERSION_REGEX_SED='v\([0-9]*\)\.\([0-9]*\)\.\([0-9]*\)'

if [ ! -z "$(git status --porcelain)" ]; then
  echo "working directory is dirty!"
  exit 1
fi

if [ -z "$VERSION" ]; then
  echo "No version specified, using 'git sv next-version'"
  VERSION=v$(git sv next-version)
fi

if [[ ! ($VERSION =~ $VERSION_REGEX) ]]; then
  echo "version is invalid"
  exit 1
fi

echo VERSION=$VERSION

FILES="install/controller/.kluctl.yaml docs/installation.md"

for f in $FILES; do
  cat $f | sed "s/$VERSION_REGEX_SED/$VERSION/g" > $f.tmp
  mv $f.tmp $f

  git add $f
done

if [ -z "$(git status --porcelain)" ]; then
  echo "nothing has changed!"
  exit 1
fi

echo "committing"
git commit -o -m "build: Preparing release $VERSION" -- $FILES

echo "tagging"
git tag -f $VERSION
