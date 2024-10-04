#!/bin/bash
set -eufo pipefail

PROJECT_DIR="$(cd -- "$(dirname -- "$0")/.." && pwd)"
. "$PROJECT_DIR/scripts/.common.sh"
cd "$PROJECT_DIR"

if ! command -v golangci-lint &>/dev/null; then
	echo -e "${YELLOW}Installing golangci-lint ...${RESET}"
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
fi

if ! command -v govulncheck &>/dev/null; then
	echo -e "${YELLOW}Installing govulncheck ...${RESET}"
	go install golang.org/x/vuln/cmd/govulncheck@latest
fi

trap cleanup EXIT
cleanup() {
	if [ "$?" -ne 0 ]; then
		echo -e "${RED}LINTING FAILED${RESET}"
	fi
}

echo -e "${YELLOW}golangci-lint ...${RESET}"
golangci-lint run ./...
echo -e "${YELLOW}govulncheck ...${RESET}"
govulncheck ./...
