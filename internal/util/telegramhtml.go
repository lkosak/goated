package util

import (
	"html"
	"regexp"
	"strings"
)

var (
	// Block-level patterns (applied per-line)
	headerRe = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

	// Inline patterns (order matters — code first to avoid nested formatting)
	codeInlineRe   = regexp.MustCompile("`([^`]+)`")
	boldItalicRe   = regexp.MustCompile(`\*\*\*(.+?)\*\*\*`)
	boldRe         = regexp.MustCompile(`\*\*(.+?)\*\*`)
	italicRe       = regexp.MustCompile(`\*(.+?)\*`)
	strikethroughRe = regexp.MustCompile(`~~(.+?)~~`)
)

// MarkdownToTelegramHTML converts common markdown to Telegram-safe HTML.
// It handles headers, bold, italic, bold-italic, strikethrough, inline code,
// fenced code blocks, blockquotes, and unordered/ordered lists.
func MarkdownToTelegramHTML(md string) string {
	lines := strings.Split(md, "\n")
	var out []string
	inCodeBlock := false
	var codeLang string

	for _, line := range lines {
		// Fenced code blocks
		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				out = append(out, "</code></pre>")
				inCodeBlock = false
			} else {
				codeLang = strings.TrimSpace(strings.TrimPrefix(line, "```"))
				if codeLang != "" {
					out = append(out, "<pre><code class=\"language-"+html.EscapeString(codeLang)+"\">")
				} else {
					out = append(out, "<pre><code>")
				}
				inCodeBlock = true
			}
			continue
		}
		if inCodeBlock {
			out = append(out, html.EscapeString(line))
			continue
		}

		// Blockquotes
		if strings.HasPrefix(line, "> ") {
			inner := convertInline(html.EscapeString(strings.TrimPrefix(line, "> ")))
			out = append(out, "<blockquote>"+inner+"</blockquote>")
			continue
		}

		// Headers → bold
		if m := headerRe.FindStringSubmatch(line); m != nil {
			out = append(out, "<b>"+convertInline(html.EscapeString(m[2]))+"</b>")
			continue
		}

		// Unordered list
		trimmed := strings.TrimLeft(line, " ")
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			content := trimmed[2:]
			out = append(out, "• "+convertInline(html.EscapeString(content)))
			continue
		}

		// Regular line
		out = append(out, convertInline(html.EscapeString(line)))
	}

	if inCodeBlock {
		out = append(out, "</code></pre>")
	}

	return strings.Join(out, "\n")
}

func convertInline(escaped string) string {
	// Code inline (already escaped, so unescape backtick content for code tag)
	escaped = codeInlineRe.ReplaceAllString(escaped, "<code>$1</code>")
	// Bold+italic
	escaped = boldItalicRe.ReplaceAllString(escaped, "<b><i>$1</i></b>")
	// Bold
	escaped = boldRe.ReplaceAllString(escaped, "<b>$1</b>")
	// Italic
	escaped = italicRe.ReplaceAllString(escaped, "<i>$1</i>")
	// Strikethrough
	escaped = strikethroughRe.ReplaceAllString(escaped, "<s>$1</s>")
	return escaped
}
