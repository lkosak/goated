# GOATED_CLI_README.md

`./goat` is the agent-facing CLI.

## Sending messages to the user

Use `send_user_message` to respond to the user via Telegram:

```sh
echo "Your markdown message" | ./goat send_user_message --chat <chat_id>
```

Or with a heredoc for multi-line messages:

```sh
./goat send_user_message --chat <chat_id> <<'EOF'
Here is a **bold** and *italic* example.

```python
print("code blocks work too")
```

And `inline code` as well.
EOF
```

### Supported markdown formatting

- **Bold**: `**text**`
- *Italic*: `*text*`
- ***Bold italic***: `***text***`
- ~~Strikethrough~~: `~~text~~`
- `Inline code`: `` `code` ``
- Fenced code blocks with language tags: ` ```python ... ``` `
- Blockquotes: `> text`
- Headers (rendered as bold): `# Heading`
- Lists: `- item` or `* item`

### Important notes

- The `--chat` flag is required. Your chat ID is provided in the prompt envelope.
- Message is read from **stdin** — always pipe or heredoc your content.

## Credential management

- Set secret: `./goat creds set GITHUB_API_KEY ghp_xxx`
- Read secret: `./goat creds get GITHUB_API_KEY`
- List secrets: `./goat creds list`

## Cron management

- Add cron: `./goat cron add --chat <chat_id> --schedule "0 8 * * *" --prompt "Send me Berkeley weather"`
- List crons: `./goat cron list --chat <chat_id>`
- Disable cron: `./goat cron disable <id>`
- Enable cron: `./goat cron enable <id>`
- Remove cron: `./goat cron remove <id>`

## Subagents

- Run headless helper: `./goat spawn-subagent --prompt "Run a headless task"`

## Control-plane CLI

The control-plane CLI is `./goated` (from the repo root).
