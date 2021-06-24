#!/bin/bash

BUILD_COMMIT=$(git rev-parse --short HEAD)
BUILD_VERSION=$(cat .version)

go build \
	-ldflags "-w -s -X main.commit=${BUILD_COMMIT} -X main.version=${BUILD_VERSION}" \
	-o bin/dnsrouter \
	./cmd/dnsrouter
