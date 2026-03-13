# Telegram Message Formatting

Your markdown is auto-converted to Telegram's HTML format by `send_user_message`. Write standard markdown — the conversion handles the rest. If HTML rendering fails, the message falls back to plain text automatically.

## What works

| Markdown you write       | Telegram HTML output           |
|--------------------------|--------------------------------|
| `**bold**`               | `<b>bold</b>`                  |
| `*italic*`               | `<i>italic</i>`               |
| `***bold italic***`      | `<b><i>bold italic</i></b>`   |
| `~~strikethrough~~`      | `<s>strikethrough</s>`        |
| `` `inline code` ``      | `<code>inline code</code>`    |
| ` ```python ... ``` `    | `<pre><code class="language-python">...</code></pre>` |
| `> blockquote`           | `<blockquote>...</blockquote>` |
| `# Heading`              | rendered as `<b>Heading</b>`  |
| `- item` / `* item`     | `• item`                       |

## What does NOT work in Telegram

- **Markdown tables** — not supported; use code blocks for tabular data
- **Links** — `[text](url)` is NOT converted; use raw URLs
- **Images** — not supported inline; Telegram may show link previews for image URLs
- **Nested lists** — indentation is lost; keep lists flat
- **Ordered lists** (`1. item`) — not converted; use unordered lists or manual numbering

## Code blocks

Fenced code blocks with language tags are converted to Telegram's syntax-highlighted format. Telegram supports language hints (e.g. `python`, `javascript`, `bash`) and renders them with syntax highlighting in the app.

## HTML escaping

All text outside code blocks is HTML-escaped automatically. You don't need to worry about `<`, `>`, or `&` in your markdown — they're handled by the converter.

## Message size limit

Telegram caps messages at **4,096 characters**. Unlike Slack, the current implementation does NOT auto-split. Keep responses concise for Telegram.
