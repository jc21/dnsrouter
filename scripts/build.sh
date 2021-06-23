#!/bin/bash

go build \
	-ldflags "-w -s -X main.commit=${BUILD_COMMIT:-notset} -X main.version=${BUILD_VERSION}" \
	-o bin/dnsrouter \
	./cmd/dnsrouter
