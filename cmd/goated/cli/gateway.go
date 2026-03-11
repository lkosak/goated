package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"goated/internal/app"
	"goated/internal/claude"
	"goated/internal/db"
	"goated/internal/gateway"
	"goated/internal/telegram"
)

var gatewayCmd = &cobra.Command{
	Use:   "gateway",
	Short: "Run a gateway connector",
}

var gatewayTelegramCmd = &cobra.Command{
	Use:   "telegram",
	Short: "Run the Telegram gateway",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := app.LoadConfig()

		if cfg.TelegramBotToken == "" {
			return fmt.Errorf("GOAT_TELEGRAM_BOT_TOKEN is required")
		}

		database, err := db.Open(cfg.DBPath)
		if err != nil {
			return err
		}
		defer database.Close()

		bridge := &claude.TmuxBridge{
			WorkspaceDir:        cfg.WorkspaceDir,
			LogDir:              cfg.LogDir,
			ContextWindowTokens: cfg.ContextWindowTokens,
		}

		svc := &gateway.Service{
			Bridge:          bridge,
			Store:           database,
			DefaultTimezone: cfg.DefaultTimezone,
		}

		conn, err := telegram.NewConnector(cfg.TelegramBotToken)
		if err != nil {
			return err
		}

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		mode := telegram.RunModePolling
		if cfg.TelegramMode == "webhook" {
			mode = telegram.RunModeWebhook
		}

		fmt.Fprintf(os.Stderr, "Starting Telegram gateway (mode=%s)\n", mode)
		return conn.Run(ctx, svc, mode, telegram.WebhookOptions{
			PublicURL:  cfg.TelegramWebhookURL,
			ListenAddr: cfg.TelegramWebhookAddr,
			Path:       cfg.TelegramWebhookPath,
		})
	},
}

func init() {
	gatewayCmd.AddCommand(gatewayTelegramCmd)
	rootCmd.AddCommand(gatewayCmd)
}
