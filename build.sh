#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT_DIR"

mkdir -p workspace

echo "Building control binary: ./run"
go build -o run .

echo "Building daemon binary: ./goated_daemon"
go build -o goated_daemon ./cmd/daemon

echo "Building agent binary: ./workspace/goated"
go build -o workspace/goated ./cmd/goated

echo "Build complete."
echo "- $ROOT_DIR/run"
echo "- $ROOT_DIR/goated_daemon"
echo "- $ROOT_DIR/workspace/goated"
