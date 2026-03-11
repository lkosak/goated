package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"goated/internal/app"
	"goated/internal/db"
)

var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Initialize database, workspace, and .env configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		reader := bufio.NewReader(os.Stdin)

		fmt.Println("=== goated bootstrap ===")
		fmt.Println()

		// Load existing .env values as defaults
		existing := loadExistingEnv(".env")

		token := prompt(reader, "Telegram bot token", existing["GOAT_TELEGRAM_BOT_TOKEN"])
		if token == "" {
			return fmt.Errorf("telegram bot token is required")
		}

		tz := prompt(reader, "Default timezone", withDefault(existing["GOAT_DEFAULT_TIMEZONE"], "America/Los_Angeles"))
		mode := prompt(reader, "Telegram mode (polling/webhook)", withDefault(existing["GOAT_TELEGRAM_MODE"], "polling"))

		var webhookURL, webhookAddr, webhookPath string
		if mode == "webhook" {
			webhookURL = prompt(reader, "Webhook public URL", existing["GOAT_TELEGRAM_WEBHOOK_URL"])
			webhookAddr = prompt(reader, "Webhook listen address", withDefault(existing["GOAT_TELEGRAM_WEBHOOK_LISTEN_ADDR"], ":8080"))
			webhookPath = prompt(reader, "Webhook path", withDefault(existing["GOAT_TELEGRAM_WEBHOOK_PATH"], "/telegram/webhook"))
		}

		// Write .env
		var b strings.Builder
		b.WriteString("# goated configuration\n")
		b.WriteString(fmt.Sprintf("GOAT_TELEGRAM_BOT_TOKEN=%s\n", token))
		b.WriteString(fmt.Sprintf("GOAT_DEFAULT_TIMEZONE=%s\n", tz))
		b.WriteString(fmt.Sprintf("GOAT_TELEGRAM_MODE=%s\n", mode))
		if mode == "webhook" {
			b.WriteString(fmt.Sprintf("GOAT_TELEGRAM_WEBHOOK_URL=%s\n", webhookURL))
			b.WriteString(fmt.Sprintf("GOAT_TELEGRAM_WEBHOOK_LISTEN_ADDR=%s\n", webhookAddr))
			b.WriteString(fmt.Sprintf("GOAT_TELEGRAM_WEBHOOK_PATH=%s\n", webhookPath))
		}

		if err := os.WriteFile(".env", []byte(b.String()), 0o600); err != nil {
			return fmt.Errorf("write .env: %w", err)
		}
		fmt.Println()
		fmt.Println("Wrote .env")

		// Init DB
		cfg := app.LoadConfig()
		store, err := db.Open(cfg.DBPath)
		if err != nil {
			return err
		}
		defer store.Close()
		fmt.Println("Database initialized at", cfg.DBPath)

		// Ensure workspace dir exists
		if err := os.MkdirAll(cfg.WorkspaceDir, 0o755); err != nil {
			return fmt.Errorf("mkdir workspace: %w", err)
		}
		fmt.Println("Workspace directory:", cfg.WorkspaceDir)

		fmt.Println()
		fmt.Println("Bootstrap complete. Run ./goated_daemon to start.")
		return nil
	},
}

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

func withDefault(val, fallback string) string {
	if val != "" {
		return val
	}
	return fallback
}

func loadExistingEnv(path string) map[string]string {
	m := make(map[string]string)
	f, err := os.Open(path)
	if err != nil {
		return m
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
		m[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	return m
}

func init() {
	rootCmd.AddCommand(bootstrapCmd)
}
