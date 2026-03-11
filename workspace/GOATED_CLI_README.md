# GOATED_CLI_README.md

goated (agent-facing CLI) overview:
- /workspace/goated creds set GITHUB_API_KEY ghp_xxx
- /workspace/goated creds get GITHUB_API_KEY
- /workspace/goated creds list
- /workspace/goated cron add --chat <chat_id> --schedule "0 8 * * *" --prompt "Send me Berkeley weather"
- /workspace/goated cron list --chat <chat_id>
- /workspace/goated cron disable <id>
- /workspace/goated cron enable <id>
- /workspace/goated cron remove <id>
- /workspace/goated spawn-subagent --prompt "Run a headless task"

Credential storage:
- /workspace/creds/*.txt

Control-plane CLI is /run.
