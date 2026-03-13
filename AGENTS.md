# AGENTS.md — Developer guide for AI agents

For codebase architecture, project structure, and how the system works, see [CODEBASE.md](CODEBASE.md).
For agent runtime instructions (responding to users, memory, identity), see [workspace/CLAUDE.md](workspace/CLAUDE.md).

## Building

**Always use `build.sh`** — never `go build` directly.

```bash
./build.sh              # builds all three binaries
```

Three binaries are produced:
- `./goated` — control CLI
- `./goated_daemon` — gateway daemon
- `./workspace/goat` — agent CLI (used by Claude inside workspace)

## Running the daemon

```bash
./build_all_and_run_daemon.sh   # builds everything, then starts daemon
```

Or if already built:
```bash
./goated_daemon                 # self-daemonizes, logs to logs/goated_daemon.log
```

The daemon self-backgrounds. No `nohup &` needed.

## Restarting the daemon

Use the daemon management command — it waits for in-flight messages to flush:

```bash
./goated daemon restart --reason "deployed new build"
```

## Verifying changes compile

```bash
go build ./...          # quick compile check
./build.sh              # full build
```

No automated tests yet. Test manually by sending messages through the gateway.
