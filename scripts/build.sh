#!/bin/bash
set -eufo pipefail

PROJECT_DIR="$(cd -- "$(dirname -- "$0")/.." && pwd)"
. "$PROJECT_DIR/scripts/.common.sh"
cd "$PROJECT_DIR"

if [ "${BUILD_COMMIT:-}" = "" ]; then
	BUILD_COMMIT=$(git log -n 1 --format=%h)
fi

if [ "${BUILD_VERSION:-}" = "" ]; then
	BUILD_VERSION=$(cat .version)
fi

echo -e "${BLUE}❯ ${GREEN}build:${RESET}"
echo "  BUILD_COMMIT:  ${BUILD_COMMIT:-notset}"
echo "  BUILD_VERSION: $BUILD_VERSION"
echo "  CGO_ENABLED:   ${CGO_ENABLED:-not set}"
echo "  GO111MODULE:   ${GO111MODULE:-}"
echo "  GOPRIVATE:     ${GOPRIVATE:-}"
echo "  GOPROXY:       ${GOPROXY:-}"

cleanup() {
	docker run --rm -v "$(pwd):/app" "$GOTOOLS_IMAGE" chown -R "$(id -u):$(id -g)" /app/bin
}

build() {
	echo -e "${BLUE}❯ ${CYAN}Building for ${YELLOW}${1}-${2} ...${RESET}"

	FILENAME="dnsrouter-v${BUILD_VERSION}_${1}_${2}"
	if [ "$1" = "windows" ]; then
		FILENAME="${FILENAME}.exe"
	fi

	docker run --rm \
		--pull always \
		-e BUILD_COMMIT="${BUILD_COMMIT:-notset}" \
		-e BUILD_VERSION="$BUILD_VERSION" \
		-e GOARCH="${2}" \
		-e GOOS="${1}" \
		-e GOPRIVATE="${GOPRIVATE:-}" \
		-e GOPROXY="${GOPROXY:-}" \
		-e TZ="${TZ:-Australia/Brisbane}" \
		-v "$(pwd):/workspace" \
		-w '/workspace' \
		"$GOTOOLS_IMAGE" \
		go build \
			-ldflags "-w -s -X main.commit=${BUILD_COMMIT:-notset} -X main.version=${BUILD_VERSION:-0.0.0}" \
			-o "/workspace/bin/$FILENAME" \
			./cmd/dnsrouter
}

build "darwin" "amd64"
build "darwin" "arm64"
build "linux" "amd64"
build "linux" "arm64"
build "linux" "arm"
build "openbsd" "amd64"
build "windows" "amd64"

cleanup

echo -e "${BLUE}❯ ${GREEN}build completed${RESET}"
exit 0

trap cleanup EXIT
