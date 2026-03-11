package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"goated/internal/app"
	"goated/internal/db"
	"goated/internal/subagent"
)

var spawnSubagentCmd = &cobra.Command{
	Use:   "spawn-subagent",
	Short: "Run a headless Claude subagent in the background",
	RunE: func(cmd *cobra.Command, args []string) error {
		prompt, _ := cmd.Flags().GetString("prompt")
		chatID, _ := cmd.Flags().GetString("chat")

		if prompt == "" {
			return fmt.Errorf("--prompt is required")
		}

		cfg := app.LoadConfig()

		store, err := db.Open(cfg.DBPath)
		if err != nil {
			return fmt.Errorf("open db: %w", err)
		}
		defer store.Close()

		logDir := filepath.Join(cfg.LogDir, "subagent", "jobs")
		if err := os.MkdirAll(logDir, 0o755); err != nil {
			return fmt.Errorf("mkdir subagent log dir: %w", err)
		}

		logFile := filepath.Join(logDir, time.Now().Format("20060102-150405")+".log")
		fullPrompt := subagent.BuildPrompt("", prompt, chatID, "subagent", logFile)

		pid, err := subagent.RunBackground(store, subagent.RunOpts{
			WorkspaceDir: cfg.WorkspaceDir,
			Prompt:       fullPrompt,
			LogPath:      logFile,
			Source:       "cli",
			ChatID:       chatID,
		})
		if err != nil {
			return err
		}

		fmt.Printf("Subagent started (pid=%d, log=%s)\n", pid, logFile)
		return nil
	},
}

func init() {
	spawnSubagentCmd.Flags().String("prompt", "", "Task prompt for the subagent (required)")
	spawnSubagentCmd.Flags().String("chat", "", "Telegram chat ID to send results to (optional)")
	rootCmd.AddCommand(spawnSubagentCmd)
}
