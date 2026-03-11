package app

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	WorkspaceDir        string
	DBPath              string
	LogDir              string
	TelegramBotToken    string
	Gateway             string
	TelegramMode        string
	TelegramWebhookURL  string
	TelegramWebhookAddr string
	TelegramWebhookPath string
	DefaultTimezone     string
	ContextWindowTokens int
}

func LoadConfig() Config {
	loadDotEnv(".env")

	cwd, _ := os.Getwd()
	workspace := getenvDefault("GOAT_WORKSPACE_DIR", filepath.Join(cwd, "workspace"))
	db := getenvDefault("GOAT_DB_PATH", filepath.Join(cwd, "goated.db"))
	logDir := getenvDefault("GOAT_LOG_DIR", filepath.Join(cwd, "logs"))
	tz := getenvDefault("GOAT_DEFAULT_TIMEZONE", "America/Los_Angeles")
	ctxTokens := getenvIntDefault("GOAT_CONTEXT_WINDOW_TOKENS", 200000)

	return Config{
		WorkspaceDir:        workspace,
		DBPath:              db,
		LogDir:              logDir,
		TelegramBotToken:    os.Getenv("GOAT_TELEGRAM_BOT_TOKEN"),
		Gateway:             getenvDefault("GOAT_GATEWAY", "telegram"),
		TelegramMode:        getenvDefault("GOAT_TELEGRAM_MODE", "polling"),
		TelegramWebhookURL:  os.Getenv("GOAT_TELEGRAM_WEBHOOK_URL"),
		TelegramWebhookAddr: getenvDefault("GOAT_TELEGRAM_WEBHOOK_LISTEN_ADDR", ":8080"),
		TelegramWebhookPath: getenvDefault("GOAT_TELEGRAM_WEBHOOK_PATH", "/telegram/webhook"),
		DefaultTimezone:     tz,
		ContextWindowTokens: ctxTokens,
	}
}

func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		// Environment variables already present should win over .env values.
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, value)
	}
}

func getenvDefault(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}

func getenvIntDefault(k string, fallback int) int {
	v := os.Getenv(k)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
