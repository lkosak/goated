# goated

This repo has two apps:
- `./goated`: control-plane CLI (bootstrap, daemon management, gateway, cron runner)
- `workspace/goat`: agent-facing CLI (messaging, credentials, cron management)

## Quick start

1. Set env vars:
   - `GOAT_TELEGRAM_BOT_TOKEN`
2. Bootstrap workspace files:
   - `go run . bootstrap` (builds `./goated` and `workspace/goat`)
3. Start with saved defaults from `.env`:
   - `./goated start`
4. (Optional) Run gateway explicitly:
   - Dev (long polling): `./goated gateway telegram --mode polling`
   - Prod (webhook): `./goated gateway telegram --mode webhook --webhook-public-url https://<your-domain>`
5. Run minutely cron from system cron:
   - `* * * * * cd /path/to/repo && /path/to/repo/goated cron run`

## Telegram commands

- `/clear` start a new Claude tmux session and rotate chat log
- `/context` approximate context window usage
- `/schedule <cron_expr> | <prompt>` store scheduled jobs

Claude sends replies directly via `./goat send_user_message --chat <chat_id>`, which pushes markdown through Telegram.

## Telegram Modes

- `polling` (default): no public URL needed; best for local dev.
- `webhook`: requires `--webhook-public-url` (or `GOAT_TELEGRAM_WEBHOOK_URL`), optional `--webhook-listen-addr` and `--webhook-path`.

## Daemon management

Use `daemon restart` with a reason to restart the daemon. The restart waits for any in-flight messages to flush before stopping the old process.

```sh
# Restart with a logged reason
./goated daemon restart --reason "deployed new build"

# Stop without restarting
./goated daemon stop

# Check status and recent restart history
./goated daemon status
```

Restart reasons are logged to `logs/restarts.jsonl` (gitignored). Starting the daemon while one is already running will tell you to use `daemon restart` instead.

## Agent CLI

- Send message: `workspace/goat send_user_message --chat <chat_id>` (reads markdown from stdin)
- Set secret: `workspace/goat creds set GITHUB_API_KEY ghp_xxx`
- Read secret: `workspace/goat creds get GITHUB_API_KEY`
- List secrets: `workspace/goat creds list`
- Add cron: `workspace/goat cron add --chat <chat_id> --schedule "0 8 * * *" --prompt "Send me Berkeley weather"`
- Run headless helper: `workspace/goat spawn-subagent --prompt "Check the weather API and summarize"`
