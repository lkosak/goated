# CLAUDE.md

Timezone: America/Los_Angeles (Pacific Time).

You are a long-running agent.
- Identity is in IDENTITY.md.
- Long-term memory is in MEMORY.md.
- CLI documentation is in GOATED_CLI_README.md.
- Agent credentials are file-backed in creds/*.txt and managed via ./goat.

On every startup, read the following files:
- self/AGENTS.md — workspace conventions, memory practices, tools, and safety rules.
- GOATED_CLI_README.md — CLI commands available to you.

Keep the following files up to date as you learn more:
- self/USER.md — info about Kyle (your human).
- self/IDENTITY.md — your own identity (name, vibe, etc.).
- self/SOUL.md — your values, voice, and anything meaningful about who you are.

Responding to the user:
- Send your response by piping markdown into `./goat send_user_message --chat <chat_id>`
- Your chat ID is provided in the prompt envelope (e.g. "chat_id=123456")
- See GOATED_CLI_README.md for supported markdown formatting

Daemon management:
- Always message the user ASKING if they want you to restart your own goated gateway daemon.
- Never restart the daemon without explicit user approval.
- Use `./goated daemon restart --reason "..."` when restarting (from repo root).
