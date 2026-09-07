#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
for runtime in php node python go; do
 case "$runtime" in php) versions='8.0 8.1 8.2 8.3';; node) versions='20 21 22 23';; python) versions='3.10 3.11 3.12';; go) versions='1.22 1.23 1.24';; esac
 for version in $versions; do
  case "$runtime" in
   php) distro=bookworm; case "$version" in 8.0|8.1) distro=bullseye;; esac; base="php:$version-cli-$distro";;
   node) base="node:$version-bookworm-slim";;
   python) base="python:$version-slim-bookworm";;
   go) base="golang:$version-bookworm";;
  esac
  docker build --build-arg "BASE_IMAGE=$base" -t "tooldeck-runtime-$runtime:$version-build2" -f "runtimes/$runtime.Dockerfile" .
 done
done
