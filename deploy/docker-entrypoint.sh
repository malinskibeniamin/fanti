#!/bin/sh
set -eu

if [ "${1:-serve}" = "serve" ]; then
  /app/fanti seed --if-empty --download-dir /app/datasets
fi

exec /app/fanti "$@"
