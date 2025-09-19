#!/usr/bin/env bash
set -euo pipefail
KVROCKS_CONSUL_ADDR=${KVROCKS_CONSUL_ADDR:-127.0.0.1:8500}
GOCACHE=${GOCACHE:-$(pwd)/.gocache}

go test ./store/engine/consul -count=1
