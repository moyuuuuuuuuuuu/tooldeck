#!/bin/sh
set -eu
if [ -S /job/proxy.sock ]; then
  socat TCP4-LISTEN:18081,bind=127.0.0.1,reuseaddr,fork UNIX-CONNECT:/job/proxy.sock &
  attempt=0
  until socat -T1 - TCP4:127.0.0.1:18081 </dev/null >/dev/null 2>/dev/null; do
    attempt=$((attempt+1))
    [ "$attempt" -lt 20 ] || exit 1
    sleep 0.05
  done
fi
run_tool() {
  case "$TOOLDECK_RUNTIME" in
    php) php "$@" ;;
    node) node "$@" ;;
    python) PYTHONPATH=/tool/.tooldeck-python python "$@" ;;
    go)
      if [ -x /tool/.tooldeck-bin ]; then /tool/.tooldeck-bin; return $?; fi
      if [ -f go.mod ]; then
        go build -mod=vendor -o /tmp/tool "./$(dirname "$1")" >&2 || return $?
      else
        go build -o /tmp/tool "$1" >&2 || return $?
      fi
      /tmp/tool
      ;;
  esac
}
run_tool "$@" > /tmp/tooldeck-result.json
# Export before the container stops: tmpfs contents disappear on stop.
tar -cf - -C /tmp tooldeck-result.json -C /job output
