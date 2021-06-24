#!/bin/bash
set -e

IMAGE=jc21/gotools:latest

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$DIR/../.common.sh"

cd $DIR/../..

if [ "$BUILD_COMMIT" = "" ]; then
	BUILD_COMMIT=$(git log -n 1 --format=%h)
fi

if [ "$BUILD_VERSION" = "" ]; then
	BUILD_VERSION=$(cat .version)
fi

echo -e "${BLUE}❯ ${GREEN}test: ${YELLOW}${1:-}${RESET}"
echo "  BUILD_COMMIT:  ${BUILD_COMMIT:-notset}"
echo "  BUILD_VERSION: $BUILD_VERSION"
echo "  CGO_ENABLED:   ${CGO_ENABLED:-not set}"
echo "  GO111MODULE:   ${GO111MODULE:-}"
echo "  GOPRIVATE:     ${GOPRIVATE:-}"
echo "  GOPROXY:       ${GOPROXY:-}"

if [ "${1:-}" = "--inside-docker" ]; then
	mkdir -p /workspace
	echo -e "${BLUE}❯ ${CYAN}Nancy setup${RESET}"
	cd /workspace
	# go get github.com/sonatype-nexus-community/nancy
	cp /app/go.mod /app/go.sum /app/.nancy-ignore .
	go mod download

	echo -e "${BLUE}❯ ${CYAN}Nancy testing${RESET}"
	go list -json -m all | nancy sleuth --quiet --username "${NANCY_USER}" --token "${NANCY_TOKEN:-}"
	rm -rf /workspace

	echo -e "${BLUE}❯ ${CYAN}Testing code${RESET}"
	cd /app
	[ -z "$(go tool fix -diff ./internal)" ]
	richgo test -cover -v ./internal/...
	richgo test -bench=. ./internal/...
	golangci-lint -v run ./...
else
	# run this script from within docker
	docker pull "${IMAGE}"
	docker run --rm \
		-e BUILD_COMMIT="${BUILD_COMMIT:-notset}" \
		-e BUILD_DATE="$BUILD_DATE" \
		-e BUILD_VERSION="$BUILD_VERSION" \
		-e GOARCH="${2}" \
		-e GOOS="${1}" \
		-e GOPRIVATE="${GOPRIVATE:-}" \
		-e GOPROXY="${GOPROXY:-}" \
		-e NOW="$NOW" \
		-e SENTRY_DSN="${SENTRY_DSN:-}" \
		-e TZ="${TZ:-Australia/Brisbane}" \
		-v "$(pwd):/app" \
		-w '/app' \
		"${IMAGE}" \
		/app/scripts/ci/test.sh --inside-docker
fi

echo -e "${BLUE}❯ ${GREEN}test ${YELLOW}${1:-} ${GREEN}completed${RESET}"
exit 0
