package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/slack-go/slack"
	"github.com/spf13/cobra"

	"goated/internal/app"
)

var slackCmd = &cobra.Command{
	Use:   "slack",
	Short: "Inspect Slack history via the configured bot token",
	Long:  "Slack inspection helpers for replaying prior messages without dropping to curl.",
}

var (
	slackHistoryChat      string
	slackHistoryLimit     int
	slackHistoryLatest    string
	slackHistoryOldest    string
	slackHistoryInclusive bool
	slackHistoryCursor    string
	slackHistoryJSON      bool
	slackHistoryReverse   bool
)

var slackHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "Fetch message history for a Slack DM/channel",
	Long: strings.TrimSpace(`
Fetch Slack message history for the configured DM/channel or an explicit --chat ID.

Use --latest/--oldest with raw Slack timestamps to replay a slice of history, or
use --cursor/--json when you need pagination.`),
	Example: strings.TrimSpace(`
  ./goat slack history
  ./goat slack history --limit 50 --reverse
  ./goat slack history --latest 1774060301.552199 --limit 10
  ./goat slack history --oldest 1774060301.552199 --inclusive --limit 20
  ./goat slack history --json | jq '.messages[].text'`),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := app.LoadConfig()
		if cfg.SlackBotToken == "" {
			return fmt.Errorf("GOAT_SLACK_BOT_TOKEN is required")
		}

		channelID := strings.TrimSpace(slackHistoryChat)
		if channelID == "" {
			channelID = strings.TrimSpace(cfg.SlackChannelID)
		}
		if channelID == "" {
			return fmt.Errorf("--chat is required (or set GOAT_SLACK_CHANNEL_ID)")
		}

		api := slack.New(cfg.SlackBotToken)
		params := slack.GetConversationHistoryParameters{
			ChannelID: channelID,
			Limit:     slackHistoryLimit,
			Inclusive: slackHistoryInclusive,
			Cursor:    strings.TrimSpace(slackHistoryCursor),
		}
		if strings.TrimSpace(slackHistoryLatest) != "" {
			params.Latest = strings.TrimSpace(slackHistoryLatest)
		}
		if strings.TrimSpace(slackHistoryOldest) != "" {
			params.Oldest = strings.TrimSpace(slackHistoryOldest)
		}

		history, err := api.GetConversationHistory(&params)
		if err != nil {
			return fmt.Errorf("get slack history: %w", err)
		}

		if slackHistoryJSON {
			payload := map[string]any{
				"ok":                true,
				"channel":           channelID,
				"has_more":          history.HasMore,
				"pin_count":         history.PinCount,
				"latest":            history.Latest,
				"response_metadata": history.ResponseMetaData,
				"messages":          history.Messages,
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(payload)
		}

		msgs := append([]slack.Message(nil), history.Messages...)
		if slackHistoryReverse {
			sort.SliceStable(msgs, func(i, j int) bool {
				return msgs[i].Timestamp < msgs[j].Timestamp
			})
		}

		fmt.Printf("Channel: %s\n", channelID)
		fmt.Printf("Messages: %d\n", len(msgs))
		fmt.Printf("Has More: %t\n", history.HasMore)
		if history.Latest != "" {
			fmt.Printf("Latest: %s\n", history.Latest)
		}
		if cursor := strings.TrimSpace(history.ResponseMetaData.NextCursor); cursor != "" {
			fmt.Printf("Next Cursor: %s\n", cursor)
		}
		fmt.Println()

		if len(msgs) == 0 {
			fmt.Println("(no messages)")
			return nil
		}

		for _, msg := range msgs {
			renderSlackMessage(msg)
		}
		return nil
	},
}

func renderSlackMessage(msg slack.Message) {
	ts := strings.TrimSpace(msg.Timestamp)
	when := slackTimestamp(ts)
	actor := firstNonEmpty(
		strings.TrimSpace(msg.Username),
		strings.TrimSpace(msg.User),
		strings.TrimSpace(msg.BotID),
		"unknown",
	)

	fmt.Printf("[%s] %s", when, actor)
	if subtype := strings.TrimSpace(msg.SubType); subtype != "" {
		fmt.Printf(" subtype=%s", subtype)
	}
	fmt.Printf(" ts=%s\n", ts)

	text := strings.TrimSpace(msg.Text)
	if text != "" {
		fmt.Println(text)
	}

	if len(msg.Files) > 0 {
		for _, f := range msg.Files {
			name := firstNonEmpty(strings.TrimSpace(f.Name), strings.TrimSpace(f.Title), strings.TrimSpace(f.ID), "file")
			fmt.Printf("[file] %s\n", name)
		}
	}

	if threadTS := strings.TrimSpace(msg.ThreadTimestamp); threadTS != "" && threadTS != ts {
		fmt.Printf("[thread_ts] %s\n", threadTS)
	}

	fmt.Println()
}

func slackTimestamp(ts string) string {
	if ts == "" {
		return "unknown-time"
	}
	secsPart := ts
	nanosPart := ""
	if i := strings.IndexByte(ts, '.'); i >= 0 {
		secsPart = ts[:i]
		nanosPart = ts[i+1:]
	}
	secs, err := strconv.ParseInt(secsPart, 10, 64)
	if err != nil {
		return ts
	}
	if nanosPart == "" {
		return time.Unix(secs, 0).UTC().Format(time.RFC3339)
	}
	if len(nanosPart) > 9 {
		nanosPart = nanosPart[:9]
	}
	for len(nanosPart) < 9 {
		nanosPart += "0"
	}
	nanos, err := strconv.ParseInt(nanosPart, 10, 64)
	if err != nil {
		return time.Unix(secs, 0).UTC().Format(time.RFC3339)
	}
	return time.Unix(secs, nanos).UTC().Format(time.RFC3339)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func init() {
	slackHistoryCmd.Flags().StringVar(&slackHistoryChat, "chat", "", "Slack channel/DM ID (defaults to configured Slack channel)")
	slackHistoryCmd.Flags().IntVar(&slackHistoryLimit, "limit", 20, "Maximum number of messages to fetch")
	slackHistoryCmd.Flags().StringVar(&slackHistoryLatest, "latest", "", "Latest Slack timestamp boundary (exclusive unless --inclusive)")
	slackHistoryCmd.Flags().StringVar(&slackHistoryOldest, "oldest", "", "Oldest Slack timestamp boundary (exclusive unless --inclusive)")
	slackHistoryCmd.Flags().BoolVar(&slackHistoryInclusive, "inclusive", false, "Include messages matching --latest/--oldest exactly")
	slackHistoryCmd.Flags().StringVar(&slackHistoryCursor, "cursor", "", "Slack pagination cursor from a previous response")
	slackHistoryCmd.Flags().BoolVar(&slackHistoryJSON, "json", false, "Emit raw JSON response")
	slackHistoryCmd.Flags().BoolVar(&slackHistoryReverse, "reverse", false, "Render messages oldest-first")
	slackCmd.AddCommand(slackHistoryCmd)
	rootCmd.AddCommand(slackCmd)
}
