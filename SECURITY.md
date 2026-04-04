# Security Policy

Please do not open public issues for suspected vulnerabilities, credential exposure, or bypasses involving Slack, Telegram, local credential storage, or runtime execution behavior.

Report security issues privately to the maintainers instead. Include:

- A clear description of the issue
- Reproduction steps or a proof of concept
- Expected impact
- Any relevant logs, versions, or environment details

If you are unsure whether something is security-sensitive, report it privately first.

## Scope

Security-sensitive areas in this repo include:

- `workspace/creds/` secret handling
- Slack and Telegram token usage
- Runtime command execution and sandboxing behavior
- Daemon, watchdog, and cron execution paths
- Log redaction and persistence of sensitive data

## Handling

The goal is to acknowledge reports quickly, reproduce them, and ship a fix before public disclosure when practical.

## Logging and sensitive data

Goated writes operational logs and structured message logs to local disk. Those logs should be treated as sensitive.

At a high level:

- Goated attempts to redact credential values sourced from workspace creds before writing structured logs and runtime output.
- Recent logs are periodically re-scrubbed so newly added credentials can be removed from recent agent output and message history.
- Message and audit logs are partitioned by date and session for manageability, but this file does not guarantee any specific retention or deletion policy.
- Plain daemon, watchdog, and job logs may still contain operational details and should be handled accordingly.

Implementation details for credential storage, agent guardrails, and log redaction live in [docs/SECURITY.md](docs/SECURITY.md).
