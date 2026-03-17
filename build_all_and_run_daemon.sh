#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
unset CLAUDECODE
./build.sh

# If daemon is already running, restart it; otherwise start fresh.
status_output=$(./goated daemon status 2>&1 || true)
if echo "$status_output" | grep -q "Daemon running"; then
  exec ./goated daemon restart --reason "build_all_and_run_daemon.sh"
fi
exec ./goated daemon run
