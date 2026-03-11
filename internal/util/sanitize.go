package util

import "regexp"

var nonWord = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func SafeName(in string) string {
	if in == "" {
		return "default"
	}
	return nonWord.ReplaceAllString(in, "_")
}
