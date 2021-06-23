#!/bin/bash
set -e

./scripts/build.sh

export DNSROUTER_PORT=5353
export DNSROUTER_LOG_LEVEL=debug

./bin/dnsrouter -c "$(pwd)/config.json" -v
