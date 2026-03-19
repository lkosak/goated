#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "Building toolbox..."
(
  cd "$ROOT_DIR/tools/toolbox"
  go build -o ../toolbox ./cmd/toolbox
)

echo "Building notesmd..."
(
  cd "$ROOT_DIR/tools/notesmd-cli"
  go build -o ../notesmd .
)

echo "Built:"
echo "- $ROOT_DIR/tools/toolbox"
echo "- $ROOT_DIR/tools/notesmd"
