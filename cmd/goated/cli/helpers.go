package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

var errSecretInputInterrupted = errors.New("secret input interrupted")

const secretMaskPrefix = "🔒 "

func prompt(reader *bufio.Reader, label, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("  %s [%s]: ", label, defaultVal)
	} else {
		fmt.Printf("  %s: ", label)
	}
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal
	}
	return line
}

func promptSecret(reader *bufio.Reader, label string) string {
	fmt.Printf("  %s: ", label)

	if term.IsTerminal(int(os.Stdin.Fd())) {
		line, err := readMaskedSecret(int(os.Stdin.Fd()), label)
		if err == nil {
			return strings.TrimSpace(line)
		}
		if errors.Is(err, errSecretInputInterrupted) {
			return ""
		}
		fmt.Println("  (warning: failed to hide input; falling back to visible input)")
	}

	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

func withDefault(val, fallback string) string {
	if val != "" {
		return val
	}
	return fallback
}

func readMaskedSecret(fd int, label string) (string, error) {
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return "", err
	}
	defer term.Restore(fd, oldState)

	input := bufio.NewReader(os.Stdin)
	var secret []rune

	for {
		r, _, err := input.ReadRune()
		if err != nil {
			if err == io.EOF {
				fmt.Println()
				return string(secret), nil
			}
			return "", err
		}

		next, action := applySecretKey(secret, r)
		secret = next

		switch action {
		case secretInputSubmit:
			fmt.Print("\r\n")
			return string(secret), nil
		case secretInputInterrupt:
			fmt.Print("\r\n")
			return "", errSecretInputInterrupted
		case secretInputClear:
			redrawMaskedSecretPrompt(label, 0)
		case secretInputAppend:
			redrawMaskedSecretPrompt(label, len(secret))
		}
	}
}

func redrawMaskedSecretPrompt(label string, runeCount int) {
	fmt.Printf("\r\033[2K%s", formatMaskedSecretPrompt(label, runeCount))
}

func formatMaskedSecretPrompt(label string, runeCount int) string {
	return fmt.Sprintf("  %s: %s%s", label, secretMaskPrefix, strings.Repeat("*", runeCount))
}

type secretInputAction int

const (
	secretInputNoop secretInputAction = iota
	secretInputAppend
	secretInputClear
	secretInputSubmit
	secretInputInterrupt
)

func applySecretKey(secret []rune, r rune) ([]rune, secretInputAction) {
	switch r {
	case '\r', '\n':
		return secret, secretInputSubmit
	case 3:
		return secret, secretInputInterrupt
	case 127, '\b':
		if len(secret) == 0 {
			return secret, secretInputNoop
		}
		return secret[:0], secretInputClear
	default:
		return append(secret, r), secretInputAppend
	}
}
