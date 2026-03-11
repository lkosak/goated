package util

import (
	"regexp"
	"strconv"
	"strings"
)

var userMessageRe = regexp.MustCompile(`(?s):START_USER_MESSAGE:\s*(.*?)\s*:END_USER_MESSAGE:`)
var oldUserMessageRe = regexp.MustCompile(`(?s)<<<START_USER_MESSAGE>>>\s*(.*?)\s*<<<END_USER_(?:M|)ESSAGE>>>`)
var legacyBlockRe = regexp.MustCompile(`(?s)<<>>\s*(.*?)\s*<<>>`)
var cursorForwardRe = regexp.MustCompile(`\x1b\[([0-9]*)C`)
var ansiCsiRe = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)
var ansiSingleRe = regexp.MustCompile(`\x1b[@-Z\\-_]`)
var controlExceptNewlineTabRe = regexp.MustCompile(`[\x00-\x08\x0B-\x1F\x7F]`)

func ExtractUserMessage(s string) string {
	matches := userMessageRe.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		matches = oldUserMessageRe.FindAllStringSubmatch(s, -1)
	}
	if len(matches) == 0 {
		legacy := legacyBlockRe.FindAllStringSubmatch(s, -1)
		if len(legacy) == 0 {
			return ""
		}
		msg := sanitizeUserMessage(legacy[len(legacy)-1][1])
		if isPlaceholderOnly(msg) {
			return ""
		}
		return msg
	}
	msg := sanitizeUserMessage(matches[len(matches)-1][1])
	if isPlaceholderOnly(msg) {
		return ""
	}
	return msg
}

func sanitizeUserMessage(in string) string {
	msg := strings.TrimSpace(in)
	msg = cursorForwardRe.ReplaceAllStringFunc(msg, func(seq string) string {
		m := cursorForwardRe.FindStringSubmatch(seq)
		if len(m) < 2 || m[1] == "" {
			return " "
		}
		n, err := strconv.Atoi(m[1])
		if err != nil || n <= 0 {
			return " "
		}
		return strings.Repeat(" ", n)
	})
	msg = ansiCsiRe.ReplaceAllString(msg, "")
	msg = ansiSingleRe.ReplaceAllString(msg, "")
	msg = controlExceptNewlineTabRe.ReplaceAllString(msg, "")
	msg = strings.Join(strings.Fields(msg), " ")
	return strings.TrimSpace(msg)
}

func isPlaceholderOnly(msg string) bool {
	candidate := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(msg), " ", ""))
	return candidate == "(yourresponse)" || candidate == "yourresponse"
}
