#!/bin/bash
set -eufo pipefail

PROJECT_DIR="$(cd -- "$(dirname -- "$0")/.." && pwd)"
. "$PROJECT_DIR/scripts/.common.sh"
cd "$PROJECT_DIR"

if ! command -v go &>/dev/null; then
	echo -e "${RED}go command not found${RESET}"
	exit 1
fi

DNSROUTER_PORT=5353
DNSROUTER_LOG_LEVEL=debug
export DNSROUTER_PORT DNSROUTER_LOG_LEVEL

go run ./cmd/dnsrouter -c "${PROJECT_DIR}/config.json" -v
