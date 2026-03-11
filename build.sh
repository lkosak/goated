#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT_DIR"

mkdir -p workspace

echo "Building control binary: ./goated"
go build -o goated .

echo "Building daemon binary: ./goated_daemon"
go build -o goated_daemon ./cmd/daemon

echo "Building agent binary: ./workspace/goat"
go build -o workspace/goat ./cmd/goated

chmod +x goated goated_daemon workspace/goat

echo "Build complete."
echo "- $ROOT_DIR/goated"
echo "- $ROOT_DIR/goated_daemon"
echo "- $ROOT_DIR/workspace/goat"
