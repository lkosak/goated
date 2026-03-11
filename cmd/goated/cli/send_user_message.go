package cli

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"goated/internal/util"
)

var sendUserMessageCmd = &cobra.Command{
	Use:   "send_user_message",
	Short: "Send a markdown message to the user via Telegram",
	Long: `Send a message to a Telegram chat. The message is read from stdin as markdown.
Code blocks, bold, italic, and other standard markdown formatting are converted
to Telegram HTML automatically.

Example:
  echo "Hello **world**" | ./goat send_user_message --chat 123456
  ./goat send_user_message --chat 123456 <<'EOF'
  Here is a code example:
  ` + "```python" + `
  print("hello")
  ` + "```" + `
  EOF`,
	RunE: func(cmd *cobra.Command, args []string) error {
		chatID, _ := cmd.Flags().GetString("chat")
		if chatID == "" {
			return fmt.Errorf("--chat is required")
		}

		// Validate chat ID is a number
		if _, err := strconv.ParseInt(chatID, 10, 64); err != nil {
			return fmt.Errorf("invalid chat ID %q: must be a number", chatID)
		}

		token := os.Getenv("GOAT_TELEGRAM_BOT_TOKEN")
		if token == "" {
			return fmt.Errorf("GOAT_TELEGRAM_BOT_TOKEN is required")
		}

		// Read message from stdin
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("reading stdin: %w", err)
		}
		text := strings.TrimSpace(string(data))
		if text == "" {
			return fmt.Errorf("empty message; pipe markdown into stdin")
		}

		bot, err := tgbotapi.NewBotAPI(token)
		if err != nil {
			return fmt.Errorf("init telegram: %w", err)
		}

		chat, _ := strconv.ParseInt(chatID, 10, 64)

		// Try HTML-formatted message first
		htmlText := util.MarkdownToTelegramHTML(text)
		msg := tgbotapi.NewMessage(chat, htmlText)
		msg.ParseMode = "HTML"
		if _, err := bot.Send(msg); err == nil {
			fmt.Fprintf(os.Stderr, "Message sent to chat %s (%d chars)\n", chatID, len(text))
			return nil
		}

		// Fallback to plain text
		msg = tgbotapi.NewMessage(chat, text)
		if _, err := bot.Send(msg); err != nil {
			return fmt.Errorf("send message: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Message sent to chat %s (%d chars, plain text fallback)\n", chatID, len(text))
		return nil
	},
}

func init() {
	sendUserMessageCmd.Flags().String("chat", "", "Telegram chat ID to send to (required)")
	rootCmd.AddCommand(sendUserMessageCmd)
}
