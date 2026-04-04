# Security Notes

This document describes Goated's credential handling, agent guardrails, and log redaction behavior in more detail than the public-facing `SECURITY.md`.

## Credential model

Goated uses file-backed credentials stored in `workspace/creds/*.txt`.

- Secrets are managed through `./goat creds set`, `./goat creds get`, and `./goat creds list`.
- The main config lives in `goated.json`, but secrets are kept out of that file.
- Environment variables can still override file-backed credentials when needed.
- The workspace credential directory is treated as a security-sensitive boundary by both the runtime and the docs.

This gives the system one consistent place to read secrets from during daemon runs, cron jobs, subagents, and agent-built tools.

## Agent guardrails around credentials

The workspace instructions are designed to push agents toward the credential system instead of ad hoc secret handling.

- [workspace/AGENTS.md](/Users/dorkitude/a/dev/goated/workspace/AGENTS.md) tells agents that credentials are file-backed in `creds/*.txt` and managed via `./goat`.
- [workspace/GOATED_CLI_README.md](/Users/dorkitude/a/dev/goated/workspace/GOATED_CLI_README.md) documents `./goat creds ...` as the supported interface.
- [workspace/TOOLS.md](/Users/dorkitude/a/dev/goated/workspace/TOOLS.md) tells agents building their own tools to shell out to `./goat creds get` at runtime instead of hardcoding or copying secrets into source files.

The intended pattern is:

- store credentials once in `workspace/creds/*.txt`
- read them on demand through `./goat creds get`
- avoid embedding secrets in code, config, prompts, or tool-specific state

These are guardrails, not a formal sandbox. An agent with filesystem access can still misuse secrets if instructed to do so. The security goal here is to make the safe path the default path and to reduce accidental exposure.

## Log redaction

Goated includes a redaction layer for message logs, hook logs, and runtime output.

- The redactor loads secret values from `workspace/creds/*.txt`.
- Secret values shorter than four characters are ignored.
- Redaction replaces matched secret values with `[REDACTED]`.
- Both raw and JSON-escaped forms of each secret are scrubbed.
- For structured JSON logs, redaction is applied selectively to content-bearing fields so metadata like timestamps and IDs are not mutated accidentally.

Redaction is used in several places:

- message logs under `logs/message_logs/`
- Claude hook logs written through `./goat log-hook`
- Claude and Codex runtime output captured to files

This is designed to reduce accidental credential leakage in normal operation. It is not a guarantee that every sensitive string in every possible log sink will always be removed.

## Periodic re-redaction of recent logs and agent output

Goated also performs recurring re-redaction of recent logs so newly added credentials can be scrubbed from recent history.

- On daemon startup, Goated re-scrubs yesterday's and today's date-based message logs and recent session logs.
- While the daemon is running, it performs an hourly scrub of the most recent log files across the main structured log types.

That behavior exists to catch cases where:

- a credential appears in output before it has been written to `workspace/creds/`
- a secret is added later and should be scrubbed from recent agent memory artifacts
- an actively used session log needs another pass after the known-secret set changes

This is a daemon-managed periodic background task. It behaves like a scheduled scrub, but it is not a separate external system cron job.

## Operational caveats

- Logs under `logs/` should be treated as sensitive local artifacts.
- Date partitioning and session partitioning improve manageability, not confidentiality.
- Daemon logs, watchdog logs, and per-job logs may still contain operational context even when credential redaction is functioning correctly.
- If you are handling especially sensitive credentials, review local logging and deployment practices rather than relying on redaction alone.
