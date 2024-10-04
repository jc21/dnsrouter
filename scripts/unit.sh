#!/bin/bash
set -eufo pipefail

PROJECT_DIR="$(cd -- "$(dirname -- "$0")/.." && pwd)"
. "$PROJECT_DIR/scripts/.common.sh"
cd "$PROJECT_DIR"

if ! command -v go-test-coverage &>/dev/null; then
	echo -e "${YELLOW}Installing go-test-coverage ...${RESET}"
	go install github.com/vladopajic/go-test-coverage/v2@latest
fi

if ! command -v tparse &>/dev/null; then
	echo -e "${YELLOW}Installing tparse ...${RESET}"
	go install github.com/mfridman/tparse@latest
fi

if ! command -v go-junit-report &>/dev/null; then
	echo -e "${YELLOW}Installing go-junit-report ...${RESET}"
	go install github.com/jstemmer/go-junit-report/v2@latest
fi

trap cleanup EXIT
cleanup() {
	if [ "$?" -ne 0 ]; then
		echo -e "${RED}UNIT TESTING FAILED - check output and consider minimum coverage requirements${RESET}"
	fi
	rm -f cover.out
}

mkdir -p "${PROJECT_DIR}/test-results"
go test -json -cover -coverprofile="./cover.out" ./... | tparse || true
go tool cover -html="./cover.out" -o "${PROJECT_DIR}/test-results/coverage.html"
go test -v -covermode=atomic ./... 2>&1 | go-junit-report >"${PROJECT_DIR}/test-results/unit-results.xml"

# this enforces minimum coverage requirements
go-test-coverage -c .testcoverage.yml
