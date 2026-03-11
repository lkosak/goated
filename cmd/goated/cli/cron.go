package cli

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"goated/internal/app"
	cronpkg "goated/internal/cron"
	"goated/internal/db"
)

var cronCmd = &cobra.Command{
	Use:   "cron",
	Short: "Cron job management",
}

var cronRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Execute due cron jobs for the current minute",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := app.LoadConfig()

		database, err := db.Open(cfg.DBPath)
		if err != nil {
			return err
		}
		defer database.Close()

		runner := &cronpkg.Runner{
			Store:        database,
			WorkspaceDir: cfg.WorkspaceDir,
			LogDir:       cfg.LogDir,
		}

		now := time.Now()
		fmt.Printf("Running cron check for %s\n", now.Format(time.RFC3339))
		return runner.Run(context.Background(), now)
	},
}

var (
	cronAddChat     string
	cronAddSchedule string
	cronAddPrompt   string
)

var cronAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new cron job",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cronAddChat == "" || cronAddSchedule == "" || cronAddPrompt == "" {
			return fmt.Errorf("--chat, --schedule, and --prompt are required")
		}
		cfg := app.LoadConfig()
		database, err := db.Open(cfg.DBPath)
		if err != nil {
			return err
		}
		defer database.Close()

		id, err := database.AddCron(cronAddChat, cronAddSchedule, cronAddPrompt, cfg.DefaultTimezone)
		if err != nil {
			return err
		}
		fmt.Printf("Created cron %d\n", id)
		return nil
	},
}

var cronListChat string

var cronListCmd = &cobra.Command{
	Use:   "list",
	Short: "List cron jobs",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := app.LoadConfig()
		database, err := db.Open(cfg.DBPath)
		if err != nil {
			return err
		}
		defer database.Close()

		jobs, err := database.AllCrons()
		if err != nil {
			return err
		}

		if cronListChat != "" {
			var filtered []db.CronJob
			for _, j := range jobs {
				if j.ChatID == cronListChat {
					filtered = append(filtered, j)
				}
			}
			jobs = filtered
		}

		if len(jobs) == 0 {
			fmt.Println("(no cron jobs)")
			return nil
		}
		for _, j := range jobs {
			status := "active"
			if !j.Active {
				status = "disabled"
			}
			fmt.Printf("#%d [%s] schedule=%q chat=%s prompt=%q\n", j.ID, status, j.Schedule, j.ChatID, j.Prompt)
		}
		return nil
	},
}

var cronEnableCmd = &cobra.Command{
	Use:   "enable ID",
	Short: "Enable a cron job",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid ID: %w", err)
		}
		cfg := app.LoadConfig()
		database, err := db.Open(cfg.DBPath)
		if err != nil {
			return err
		}
		defer database.Close()

		if err := database.SetCronActive(id, true); err != nil {
			return err
		}
		fmt.Printf("Enabled cron %d\n", id)
		return nil
	},
}

var cronDisableCmd = &cobra.Command{
	Use:   "disable ID",
	Short: "Disable a cron job",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid ID: %w", err)
		}
		cfg := app.LoadConfig()
		database, err := db.Open(cfg.DBPath)
		if err != nil {
			return err
		}
		defer database.Close()

		if err := database.SetCronActive(id, false); err != nil {
			return err
		}
		fmt.Printf("Disabled cron %d\n", id)
		return nil
	},
}

var cronRemoveCmd = &cobra.Command{
	Use:   "remove ID",
	Short: "Remove a cron job",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseUint(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid ID: %w", err)
		}
		cfg := app.LoadConfig()
		database, err := db.Open(cfg.DBPath)
		if err != nil {
			return err
		}
		defer database.Close()

		if err := database.DeleteCron(id); err != nil {
			return err
		}
		fmt.Printf("Removed cron %d\n", id)
		return nil
	},
}

func init() {
	cronAddCmd.Flags().StringVar(&cronAddChat, "chat", "", "Chat ID for notifications")
	cronAddCmd.Flags().StringVar(&cronAddSchedule, "schedule", "", "Cron schedule (5-field)")
	cronAddCmd.Flags().StringVar(&cronAddPrompt, "prompt", "", "Prompt to execute")

	cronListCmd.Flags().StringVar(&cronListChat, "chat", "", "Filter by chat ID (optional)")

	cronCmd.AddCommand(cronRunCmd, cronAddCmd, cronListCmd, cronEnableCmd, cronDisableCmd, cronRemoveCmd)
	rootCmd.AddCommand(cronCmd)
}
