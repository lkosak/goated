package util

import (
	"regexp"
	"strings"
)

var (
	// Slack mrkdwn uses *bold*, _italic_, ~strike~, `code`, ```code blocks```
	// Standard markdown uses **bold**, *italic*, ~~strike~~
	mdBoldItalicToSlack = regexp.MustCompile(`\*\*\*(.+?)\*\*\*`)
	mdBoldToSlack       = regexp.MustCompile(`\*\*(.+?)\*\*`)
	mdStrikeToSlack     = regexp.MustCompile(`~~(.+?)~~`)
	// Markdown backslash escapes that should be stripped for Slack.
	// Agents often emit \! \. \- \( \) etc. which Slack shows literally.
	mdBackslashEscape = regexp.MustCompile(`\\([!.\-()#\[\]{}+>|_~])`)
)

// isTableLine returns true if the line looks like a markdown table row (starts/ends with |).
func isTableLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return len(trimmed) > 1 && trimmed[0] == '|' && trimmed[len(trimmed)-1] == '|'
}

// isTableSeparator returns true for lines like |---|---|---| (separator rows).
func isTableSeparator(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !isTableLine(line) {
		return false
	}
	inner := trimmed[1 : len(trimmed)-1]
	for _, c := range inner {
		if c != '-' && c != ':' && c != ' ' && c != '|' {
			return false
		}
	}
	return true
}

// parseTableCells splits a markdown table row into trimmed cell values.
func parseTableCells(line string) []string {
	trimmed := strings.TrimSpace(line)
	// Strip leading/trailing pipes
	inner := trimmed[1 : len(trimmed)-1]
	parts := strings.Split(inner, "|")
	cells := make([]string, len(parts))
	for i, p := range parts {
		cells[i] = strings.TrimSpace(p)
	}
	return cells
}

// formatTableBlock converts collected markdown table rows into a clean
// aligned code block without pipe characters.
func formatTableBlock(rows [][]string) []string {
	if len(rows) == 0 {
		return nil
	}

	// Find max width for each column
	numCols := 0
	for _, row := range rows {
		if len(row) > numCols {
			numCols = len(row)
		}
	}
	widths := make([]int, numCols)
	for _, row := range rows {
		for i, cell := range row {
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Build aligned lines
	var result []string
	result = append(result, "```")
	for _, row := range rows {
		var parts []string
		for i := 0; i < numCols; i++ {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			// Pad to column width
			padded := cell + strings.Repeat(" ", widths[i]-len(cell))
			parts = append(parts, padded)
		}
		result = append(result, strings.Join(parts, "   "))
	}
	result = append(result, "```")
	return result
}

// MarkdownToSlackMrkdwn converts standard markdown to Slack's mrkdwn format.
// Handles bold, italic, strikethrough, code blocks, headers, lists, and tables.
// Markdown tables are automatically converted to clean aligned code blocks
// since Slack has no native table support.
func MarkdownToSlackMrkdwn(md string) string {
	lines := strings.Split(md, "\n")
	var out []string
	inCodeBlock := false
	var tableRows [][]string

	for _, line := range lines {
		// Code blocks pass through unchanged
		if strings.HasPrefix(line, "```") {
			inCodeBlock = !inCodeBlock
			out = append(out, line)
			continue
		}
		if inCodeBlock {
			out = append(out, line)
			continue
		}

		// Table handling: collect rows, then flush as aligned code block
		if isTableLine(line) {
			if isTableSeparator(line) {
				continue
			}
			tableRows = append(tableRows, parseTableCells(line))
			continue
		} else if len(tableRows) > 0 {
			out = append(out, formatTableBlock(tableRows)...)
			tableRows = nil
		}

		// Headers → bold (Slack has no header rendering)
		if m := headerRe.FindStringSubmatch(line); m != nil {
			out = append(out, "*"+convertSlackInline(m[2])+"*")
			continue
		}

		out = append(out, convertSlackInline(line))
	}

	// Flush any trailing table
	if len(tableRows) > 0 {
		out = append(out, formatTableBlock(tableRows)...)
	}

	return strings.Join(out, "\n")
}

func convertSlackInline(line string) string {
	// Strip markdown backslash escapes (e.g. \! \. \-) before other conversions
	line = mdBackslashEscape.ReplaceAllString(line, "${1}")
	// Bold+italic: ***text*** → *_text_*
	line = mdBoldItalicToSlack.ReplaceAllString(line, "*_${1}_*")
	// Bold: **text** → *text*
	line = mdBoldToSlack.ReplaceAllString(line, "*${1}*")
	// Italic: *text* stays as _text_ in Slack
	// Only convert standalone *text* that isn't already part of **bold**
	// After bold conversion, remaining single * pairs are italic
	// Strikethrough: ~~text~~ → ~text~
	line = mdStrikeToSlack.ReplaceAllString(line, "~${1}~")
	return line
}
