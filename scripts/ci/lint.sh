#!/bin/bash
set -eufo pipefail

PROJECT_DIR="$(cd -- "$(dirname -- "$0")/../.." && pwd)"
. "$PROJECT_DIR/scripts/.common.sh"
cd "$PROJECT_DIR"

docker run \
	--pull always \
	--rm \
	-v "$PROJECT_DIR:/workspace" \
	-w /workspace \
	"$GOTOOLS_IMAGE" \
	/workspace/scripts/lint.sh
