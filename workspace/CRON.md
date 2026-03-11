# CRON.md

When running as a scheduled job:
- Read user prompt and execute it.
- Include user-facing result only between:
  :START_USER_MESSAGE:
  ...
  :END_USER_MESSAGE:
- Keep any additional execution detail outside delimiters.
