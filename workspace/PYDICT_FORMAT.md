# Pydict Format

The gateway communicates with agents using **pydict** — Python dict literal syntax. It's a superset of JSON that adds triple-quoted multiline strings, single quotes, trailing commas, and Python-style booleans.

## Why pydict?

- **Token-efficient**: no closing tags (vs XML), fewer tokens than JSON for multiline text
- **Multiline-safe**: triple-quoted strings (`"""..."""`) hold any content without escaping
- **Well-trained**: Python dict syntax appears extensively in LLM training data
- **Readable**: clean, minimal punctuation

## Envelope format

When the gateway delivers a user message, it arrives as a pydict:

```python
{
  "chat_id": "D0ACHSF1THP",
  "formatting": "SLACK_MESSAGE_FORMATTING.md",
  "message": """Hey, what's the weather?

Also check this:
\```python
print('hello')
\```""",
  "respond_with": "./goat send_user_message --chat D0ACHSF1THP",
  "source": "slack",
}
```

### Fields

| Key | Description |
|-----|-------------|
| `source` | Channel: `"slack"` or `"telegram"` |
| `chat_id` | Chat/channel ID for responses |
| `message` | The user's message text |
| `respond_with` | Command to pipe your response into |
| `formatting` | Channel-specific formatting doc to follow |

## Syntax reference

### Strings

- Double-quoted: `"hello world"`
- Single-quoted: `'hello world'`
- Triple double-quoted: `"""multiline\ntext"""`
- Triple single-quoted: `'''multiline\ntext'''`

Triple-quoted strings can contain anything — newlines, quotes, code blocks — without escaping. If the content contains `"""`, the encoder switches to `'''` automatically.

### Booleans and null

- `True` / `False` (Python-style, also accepts `true` / `false`)
- `None` (Python-style, also accepts `null`)

### Other

- Trailing commas are allowed in dicts and lists
- Comments: `# this is a comment`
- Numbers: integers and floats
- Nested dicts and lists

## Responding

Extract `respond_with` and `chat_id` from the envelope, then pipe your markdown response:

```sh
./goat send_user_message --chat D0ACHSF1THP <<'EOF'
Your response here in markdown.
EOF
```

See the `formatting` field for which formatting doc applies to the current channel.
