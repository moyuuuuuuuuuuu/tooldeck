#!/bin/sh
set -eu
socat TCP4-LISTEN:18081,bind=127.0.0.1,reuseaddr,fork UNIX-CONNECT:/job/proxy.sock &
export HTTP_PROXY=http://127.0.0.1:18081 HTTPS_PROXY=http://127.0.0.1:18081 http_proxy=http://127.0.0.1:18081 https_proxy=http://127.0.0.1:18081 NODE_USE_ENV_PROXY=1
cp -R /source/. /build/
cd /build
# stdout is reserved for the artifact archive.
{
case "$TOOLDECK_RUNTIME" in
 node)
  node --version
  if [ -f package.json ]; then
    [ -f package-lock.json ] || { echo 'package-lock.json is required'; exit 1; }
    npm ci --no-audit --no-fund
  fi
  ;;
 php)
  php --version
  if [ -f composer.json ]; then
    [ -f composer.lock ] || { echo 'composer.lock is required'; exit 1; }
    composer install --no-dev --no-interaction --prefer-dist --no-progress
  fi
  ;;
 python)
  python --version
  if [ -f requirements.txt ]; then
    python -m pip install --disable-pip-version-check --require-hashes --target .tooldeck-python -r requirements.txt
  fi
  ;;
 go)
  go version
  if [ -f go.mod ]; then
    [ -f go.sum ] || { echo 'go.sum is required'; exit 1; }
    export GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org CGO_ENABLED=0 GOTOOLCHAIN=local
    go mod download
    go build -mod=readonly -o .tooldeck-bin "./$(dirname "$1")"
  else
    CGO_ENABLED=0 go build -o .tooldeck-bin "$1"
  fi
  ;;
esac
if [ -n "${2:-}" ]; then /bin/sh -c "$2"; fi
[ -f "$1" ] || { echo 'entrypoint not found after build'; exit 1; }
} >&2
tar -cf - .
