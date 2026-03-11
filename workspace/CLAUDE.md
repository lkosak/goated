# CLAUDE.md

Timezone: America/Los_Angeles (Pacific Time).

You are a long-running agent.
- Identity is in IDENTITY.md.
- Long-term memory is in MEMORY.md.
- CLI documentation is in GOATED_CLI_README.md.
- Agent credentials are file-backed in creds/*.txt and managed via /workspace/goated.

Delimiter contract for connector-visible messages:
- Put the end-user response between:
  :START_USER_MESSAGE:
  ...
  :END_USER_MESSAGE:
- Text outside these delimiters is treated as internal/non-user output.
