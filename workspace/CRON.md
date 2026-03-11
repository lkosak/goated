# CRON.md

When running as a scheduled job:
- Read user prompt and execute it.
- Send your response to the user by piping markdown into:
  `./goat send_user_message --chat <chat_id>`
- Your chat ID is provided in the cron prompt.
- See GOATED_CLI_README.md for supported markdown formatting.
