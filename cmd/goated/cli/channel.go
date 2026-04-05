package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"goated/internal/app"
	"goated/internal/db"
)

var channelCmd = &cobra.Command{
	Use:   "channel",
	Short: "Manage messaging channels (Telegram, Slack)",
}

var channelListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured channels",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := app.LoadConfig()
		store, err := db.Open(cfg.DBPath)
		if err != nil {
			return err
		}
		defer store.Close()

		channels, err := store.AllChannels()
		if err != nil {
			return err
		}
		if len(channels) == 0 {
			fmt.Println("No channels configured. Run: goated channel add")
			return nil
		}

		activeChannelName := store.GetMeta("active_channel")

		for _, ch := range channels {
			marker := "  "
			if ch.Name == activeChannelName {
				marker = "* "
			}
			fmt.Printf("%s%-20s  type=%-10s  created=%s\n", marker, ch.Name, ch.Type, ch.CreatedAt)

			switch ch.Type {
			case "telegram":
				mode := ch.Config["mode"]
				if mode == "" {
					mode = "polling"
				}
				fmt.Printf("  %-20s  mode=%s\n", "", mode)
			case "slack":
				chID := ch.Config["channel_id"]
				if chID != "" {
					fmt.Printf("  %-20s  channel=%s\n", "", chID)
				}
			}
		}

		fmt.Println()
		fmt.Println("* = active channel")
		return nil
	},
}

var channelAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new messaging channel",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := app.LoadConfig()
		store, err := db.Open(cfg.DBPath)
		if err != nil {
			return err
		}
		defer store.Close()

		ch, err := promptChannel(bufio.NewReader(os.Stdin))
		if err != nil {
			return err
		}

		if err := store.AddChannel(*ch); err != nil {
			return err
		}

		fmt.Printf("\nChannel %q (%s) added.\n", ch.Name, ch.Type)
		fmt.Println("To activate it, run: goated channel switch", ch.Name)
		return nil
	},
}

var channelSwitchCmd = &cobra.Command{
	Use:   "switch <name>",
	Short: "Switch the active messaging channel",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		cfg := app.LoadConfig()
		store, err := db.Open(cfg.DBPath)
		if err != nil {
			return err
		}
		defer store.Close()

		ch, err := store.GetChannel(name)
		if err != nil {
			return err
		}

		if err := writeChannelConfig(ch); err != nil {
			return err
		}

		// Write channel secrets to creds files
		credsDir := filepath.Join(cfg.WorkspaceDir, "creds")
		if err := writeChannelCreds(credsDir, ch); err != nil {
			return err
		}

		if err := store.SetMeta("active_channel", name); err != nil {
			return err
		}

		fmt.Printf("Switched to channel %q (%s).\n", ch.Name, ch.Type)
		fmt.Println("Restart the daemon for changes to take effect.")
		return nil
	},
}

var channelDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a configured channel",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		cfg := app.LoadConfig()
		store, err := db.Open(cfg.DBPath)
		if err != nil {
			return err
		}
		defer store.Close()

		activeChannel := store.GetMeta("active_channel")
		if name == activeChannel {
			return fmt.Errorf("cannot delete the active channel %q — switch to another channel first", name)
		}

		if err := store.DeleteChannel(name); err != nil {
			return err
		}

		fmt.Printf("Channel %q deleted.\n", name)
		return nil
	},
}

// promptChannel runs the interactive prompts for adding a channel.
func promptChannel(reader *bufio.Reader) (*db.Channel, error) {
	fmt.Println("=== Add Channel ===")
	fmt.Println()

	name := promptRequired(reader, "Channel name (e.g. my-telegram, work-slack)", "")

	chType := promptOneOf(reader, "Type (telegram/slack)", "slack", []string{"telegram", "slack"})

	config := make(map[string]string)

	switch chType {
	case "telegram":
		fmt.Println()
		fmt.Println("  To get a Telegram bot token:")
		fmt.Println("  1. Open Telegram and message @BotFather")
		fmt.Println("  2. Send /newbot and follow the prompts")
		fmt.Println("  3. Copy the bot token it gives you")
		fmt.Println("  https://core.telegram.org/bots#botfather")
		fmt.Println()

		config["bot_token"] = promptSecretRequired(reader, "Telegram bot token")

		mode := prompt(reader, "Mode (polling/webhook)", "polling")
		config["mode"] = mode

		if mode == "webhook" {
			config["webhook_url"] = prompt(reader, "Webhook public URL", "")
			config["webhook_addr"] = prompt(reader, "Webhook listen address", ":8080")
			config["webhook_path"] = prompt(reader, "Webhook path", "/telegram/webhook")
		}

	case "slack":
		appName := prompt(reader, "Slack app display name", name)

		manifest := slackAppManifest(appName)
		fmt.Println()
		fmt.Println("  Copy the manifest below and use it to create or update your Slack app:")
		fmt.Println()
		fmt.Println("  New app:      https://api.slack.com/apps → Create New App → From a manifest")
		fmt.Println("  Existing app: https://api.slack.com/apps → Your app → App Manifest → paste & save")
		fmt.Println()
		fmt.Println("  ── manifest.json ──")
		fmt.Println(manifest)
		fmt.Println("  ────────────────────")
		fmt.Println()
		fmt.Println("  After creating/updating the app:")
		fmt.Println("  1. Install the app to your workspace (OAuth & Permissions → Install)")
		fmt.Println("  2. Copy the Bot Token (xoxb-...) from OAuth & Permissions")
		fmt.Println("  3. Go to Socket Mode → generate an App Token (xapp-...)")
		fmt.Println("     Add the 'connections:write' scope when prompted")
		fmt.Println("  4. To find your channel ID, right-click a channel in Slack")
		fmt.Println("     → View channel details → copy the ID at the bottom")
		fmt.Println()

		config["bot_token"] = promptSecretRequired(reader, "Slack bot token (xoxb-...)")
		config["app_token"] = promptSecretRequired(reader, "Slack app token (xapp-...)")
		config["channel_id"] = promptRequired(reader, "Slack DM channel ID", "")
	}

	return &db.Channel{
		Name:   name,
		Type:   chType,
		Config: config,
	}, nil
}

// writeChannelConfig updates goated.json to activate the given channel.
// Preserves non-gateway settings (timezone, workspace, db path, etc.).
func writeChannelConfig(ch *db.Channel) error {
	configPath := "goated.json"
	existing, err := app.ReadConfigJSON(configPath)
	if err != nil {
		return err
	}

	// Update gateway type
	existing["gateway"] = ch.Type

	// Update gateway-specific settings
	switch ch.Type {
	case "telegram":
		telegram := make(map[string]any)
		if t, ok := existing["telegram"].(map[string]any); ok {
			telegram = t
		}
		mode := withDefault(ch.Config["mode"], "polling")
		telegram["mode"] = mode
		if mode == "webhook" {
			if v := ch.Config["webhook_addr"]; v != "" {
				telegram["webhook_addr"] = v
			}
			if v := ch.Config["webhook_path"]; v != "" {
				telegram["webhook_path"] = v
			}
		}
		existing["telegram"] = telegram

	case "slack":
		slack := make(map[string]any)
		if s, ok := existing["slack"].(map[string]any); ok {
			slack = s
		}
		if v := ch.Config["channel_id"]; v != "" {
			slack["channel_id"] = v
		}
		existing["slack"] = slack
	}

	return app.WriteConfigJSON(configPath, existing)
}

// writeChannelCreds writes channel secrets to workspace/creds/*.txt.
func writeChannelCreds(credsDir string, ch *db.Channel) error {
	switch ch.Type {
	case "telegram":
		if v := ch.Config["bot_token"]; v != "" {
			if err := app.WriteCred(credsDir, "GOAT_TELEGRAM_BOT_TOKEN", v); err != nil {
				return err
			}
		}
		if v := ch.Config["webhook_url"]; v != "" {
			if err := app.WriteCred(credsDir, "GOAT_TELEGRAM_WEBHOOK_URL", v); err != nil {
				return err
			}
		}
	case "slack":
		if v := ch.Config["bot_token"]; v != "" {
			if err := app.WriteCred(credsDir, "GOAT_SLACK_BOT_TOKEN", v); err != nil {
				return err
			}
		}
		if v := ch.Config["app_token"]; v != "" {
			if err := app.WriteCred(credsDir, "GOAT_SLACK_APP_TOKEN", v); err != nil {
				return err
			}
		}
		if err := app.RemoveCred(credsDir, "GOAT_SLACK_CHANNEL_ID"); err != nil {
			return err
		}
	}
	return nil
}

// slackAppManifest returns a JSON manifest for creating/updating a Slack app
// with all the scopes, events, and settings goated needs.
func slackAppManifest(displayName string) string {
	manifest := map[string]any{
		"display_information": map[string]any{
			"name": displayName,
		},
		"features": map[string]any{
			"app_home": map[string]any{
				"home_tab_enabled":               false,
				"messages_tab_enabled":           true,
				"messages_tab_read_only_enabled": false,
			},
			"bot_user": map[string]any{
				"display_name":  displayName,
				"always_online": true,
			},
		},
		"oauth_config": map[string]any{
			"scopes": map[string]any{
				"bot": []string{
					"channels:history",
					"channels:read",
					"chat:write",
					"files:read",
					"im:history",
					"im:read",
					"im:write",
				},
			},
		},
		"settings": map[string]any{
			"event_subscriptions": map[string]any{
				"bot_events": []string{
					"message.channels",
					"message.im",
				},
			},
			"org_deploy_enabled":  false,
			"socket_mode_enabled": true,
		},
	}

	data, _ := json.MarshalIndent(manifest, "  ", "  ")
	return "  " + string(data)
}

func init() {
	channelCmd.AddCommand(channelListCmd)
	channelCmd.AddCommand(channelAddCmd)
	channelCmd.AddCommand(channelSwitchCmd)
	channelCmd.AddCommand(channelDeleteCmd)
	rootCmd.AddCommand(channelCmd)
}
