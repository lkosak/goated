# run + goated

This repo has two apps:
- `./run`: control-plane CLI (bootstrap, gateway, cron runner)
- `workspace/goated`: agent-facing CLI (credentials + cron management convenience commands)

## Quick start

1. Set env vars:
   - `GOAT_TELEGRAM_BOT_TOKEN`
2. Bootstrap workspace files:
   - `go run . bootstrap` (builds `./run` and `workspace/goated`)
3. Start with saved defaults from `.env`:
   - `./run start`
4. (Optional) Run gateway explicitly:
   - Dev (long polling): `./run gateway telegram --mode polling`
   - Prod (webhook): `./run gateway telegram --mode webhook --webhook-public-url https://<your-domain>`
5. Run minutely cron from system cron:
   - `* * * * * cd /path/to/repo && /path/to/repo/run cron run`

## Telegram commands

- `/clear` start a new Claude tmux session and rotate chat log
- `/context` approximate context window usage
- `/schedule <cron_expr> | <prompt>` store scheduled jobs in `goat.sqlite`

User-visible replies are extracted only from:

- `:START_USER_MESSAGE:`
- `:END_USER_MESSAGE:`

## Telegram Modes

- `polling` (default): no public URL needed; best for local dev.
- `webhook`: requires `--webhook-public-url` (or `GOAT_TELEGRAM_WEBHOOK_URL`), optional `--webhook-listen-addr` and `--webhook-path`.

## Agent CLI

- Set secret: `workspace/goated creds set GITHUB_API_KEY ghp_xxx`
- Read secret: `workspace/goated creds get GITHUB_API_KEY`
- List secrets: `workspace/goated creds list`
- Add cron: `workspace/goated cron add --chat <chat_id> --schedule \"0 8 * * *\" --prompt \"Send me Berkeley weather\"`
- Run headless helper: `workspace/goated spawn-subagent --prompt \"Check the weather API and summarize\"`
