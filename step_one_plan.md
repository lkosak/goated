# Step One Plan

## Objective
Build a simple OpenClaw-like system in Go with:
- Telegram connector -> long-running Claude Code `tmux` session in `workspace/`
- Delimiter-based user message extraction
- Session controls (`/clear`, `/context`)
- SQLite cron storage and minutely execution (`run cron run`)
- Pluggable gateway abstraction for future connectors (Slack/WhatsApp)

## Current Status (Implemented in this step)
1. CLI and project skeleton using Cobra.
2. `workspace` bootstrap files:
   - `CLAUDE.md` (Pacific timezone, delimiter contract, IDENTITY/MEMORY references, GOAT CLI ref)
   - `CRON.md`, `IDENTITY.md`, `MEMORY.md`, `GOATED_CLI_README.md`
3. Telegram connector + generic gateway handler interfaces.
4. Long-running Claude bridge via tmux:
   - per-chat tmux sessions
   - `claude --dangerously-skip-permissions` startup in session workspace
   - `tmux pipe-pane` logs for transcript capture
   - prompt injection via buffer/paste
   - response extraction from delimiters
   - `/clear` session reset + log rotation
   - `/context` heuristic percentage from log size
5. SQLite schema (`goat.sqlite`) for `crons` and `cron_runs`.
6. Minutely cron runner (`run cron run`):
   - evaluates crontab expressions in stored timezone
   - executes `claude --dangerously-skip-permissions -p ...`
   - persists run result and per-job log under `logs/cron/jobs/*.log`
   - appends `logs/cron/runs.jsonl` only when one or more jobs were due
   - optionally notifies Telegram with extracted user message + job log path

## Next Steps
1. Improve `/context` accuracy by querying Claude-native usage output (if available) rather than log-size heuristic.
2. Add NL scheduling intent parser so plain text like “every morning at 8am send me Berkeley weather” can create cron rows without `/schedule` format.
3. Add connector plugin registry and separate outbound queue abstraction for Slack/WhatsApp adapters.
4. Add webhook mode option for Telegram (for cloud hosting) in addition to long polling.
5. Add integration tests using tmux and a fake Telegram transport.
6. Add retention policy for rotated chat logs and cron job logs.
7. Add simple admin command set (`/jobs`, `/unschedule <id>`, `/schedules`).

## Ops Notes
- Run minutely via system cron:
  - `* * * * * cd /path/to/repo && /path/to/run cron run`
- Gateway process:
  - `run gateway telegram`
- Bootstrap once:
  - `run bootstrap`
