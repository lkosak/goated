package telegram

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"goat/internal/gateway"
)

type Connector struct {
	bot *tgbotapi.BotAPI
}

type RunMode string

const (
	RunModePolling RunMode = "polling"
	RunModeWebhook RunMode = "webhook"
)

type WebhookOptions struct {
	PublicURL  string
	ListenAddr string
	Path       string
}

func NewConnector(token string) (*Connector, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("init telegram bot: %w", err)
	}
	return &Connector{bot: bot}, nil
}

func (c *Connector) Run(ctx context.Context, handler gateway.Handler, mode RunMode, webhookOpts WebhookOptions) error {
	switch mode {
	case RunModePolling:
		return c.runPolling(ctx, handler)
	case RunModeWebhook:
		return c.runWebhook(ctx, handler, webhookOpts)
	default:
		return fmt.Errorf("unsupported telegram mode %q", mode)
	}
}

func (c *Connector) runPolling(ctx context.Context, handler gateway.Handler) error {
	if _, err := c.bot.Request(tgbotapi.DeleteWebhookConfig{DropPendingUpdates: false}); err != nil {
		return fmt.Errorf("delete webhook before polling: %w", err)
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30
	updates := c.bot.GetUpdatesChan(u)
	defer c.bot.StopReceivingUpdates()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case update := <-updates:
			if err := c.processUpdate(ctx, handler, update); err != nil {
				chatID := "unknown"
				if update.Message != nil {
					chatID = strconv.FormatInt(update.Message.Chat.ID, 10)
				}
				if chatID != "unknown" {
					_ = c.SendMessage(ctx, chatID, "Error: "+err.Error())
				}
			}
		}
	}
}

func (c *Connector) runWebhook(ctx context.Context, handler gateway.Handler, opts WebhookOptions) error {
	if strings.TrimSpace(opts.PublicURL) == "" {
		return fmt.Errorf("webhook mode requires public URL")
	}
	if strings.TrimSpace(opts.ListenAddr) == "" {
		opts.ListenAddr = ":8080"
	}
	if strings.TrimSpace(opts.Path) == "" {
		opts.Path = "/telegram/webhook"
	}

	webhook, err := tgbotapi.NewWebhook(strings.TrimRight(opts.PublicURL, "/") + opts.Path)
	if err != nil {
		return fmt.Errorf("build webhook config: %w", err)
	}
	if _, err := c.bot.Request(webhook); err != nil {
		return fmt.Errorf("set telegram webhook: %w", err)
	}

	updates := c.bot.ListenForWebhook(opts.Path)
	server := &http.Server{
		Addr:              opts.ListenAddr,
		ReadHeaderTimeout: 10 * time.Second,
	}

	serverErrCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrCh <- err
			return
		}
		serverErrCh <- nil
	}()

	for {
		select {
		case <-ctx.Done():
			_, _ = c.bot.Request(tgbotapi.DeleteWebhookConfig{DropPendingUpdates: false})
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = server.Shutdown(shutdownCtx)
			return ctx.Err()
		case err := <-serverErrCh:
			if err != nil {
				return fmt.Errorf("webhook server: %w", err)
			}
			return nil
		case update := <-updates:
			if err := c.processUpdate(ctx, handler, update); err != nil {
				chatID := "unknown"
				if update.Message != nil {
					chatID = strconv.FormatInt(update.Message.Chat.ID, 10)
				}
				if chatID != "unknown" {
					_ = c.SendMessage(ctx, chatID, "Error: "+err.Error())
				}
			}
		}
	}
}

func (c *Connector) processUpdate(ctx context.Context, handler gateway.Handler, update tgbotapi.Update) error {
	if update.Message == nil {
		return nil
	}
	text := strings.TrimSpace(update.Message.Text)
	if text == "" {
		return nil
	}
	msg := gateway.IncomingMessage{
		Channel: "telegram",
		ChatID:  strconv.FormatInt(update.Message.Chat.ID, 10),
		UserID:  strconv.FormatInt(int64(update.Message.From.ID), 10),
		Text:    text,
	}
	stopTyping := c.startTypingLoop(ctx, update.Message.Chat.ID)
	defer stopTyping()
	return handler.HandleMessage(ctx, msg, c)
}

func (c *Connector) SendMessage(_ context.Context, chatID, text string) error {
	chat, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid chat id %q: %w", chatID, err)
	}
	msg := tgbotapi.NewMessage(chat, text)
	_, err = c.bot.Send(msg)
	if err != nil {
		return fmt.Errorf("send telegram message: %w", err)
	}
	return nil
}

func (c *Connector) sendTyping(chatID int64) {
	action := tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping)
	_, _ = c.bot.Send(action)
}

func (c *Connector) startTypingLoop(ctx context.Context, chatID int64) func() {
	c.sendTyping(chatID)
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-ticker.C:
				c.sendTyping(chatID)
			}
		}
	}()
	return func() {
		close(done)
	}
}
