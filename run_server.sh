#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
unset CLAUDECODE
./build.sh
./goated_daemon
