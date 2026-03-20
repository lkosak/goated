package telegram

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"goated/internal/gateway"
	"goated/internal/util"
)

// OffsetStore persists the last processed Telegram update ID so restarts
// don't replay old messages.
type OffsetStore interface {
	GetMeta(key string) string
	SetMeta(key, value string) error
}

type Connector struct {
	bot   *tgbotapi.BotAPI
	store OffsetStore

	httpClient *http.Client

	attachmentsRootRel string
	attachmentsRootAbs string

	attachmentMaxBytes  int64
	attachmentTotalMax  int64
	attachmentRetention time.Duration
	sweepEvery          time.Duration
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

func NewConnector(token string, store OffsetStore, attachmentCfg AttachmentConfig) (*Connector, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("init telegram bot: %w", err)
	}

	rootPath := strings.TrimSpace(attachmentCfg.RootPath)
	if rootPath == "" {
		rootPath = defaultAttachmentsRootRel
	}
	rootAbs, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, fmt.Errorf("resolve attachments root: %w", err)
	}
	if err := os.MkdirAll(rootAbs, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir attachments root: %w", err)
	}
	rootRel := filepath.ToSlash(rootPath)
	if filepath.IsAbs(rootPath) {
		if cwd, err := os.Getwd(); err == nil {
			if rel, err := filepath.Rel(cwd, rootPath); err == nil {
				rootRel = filepath.ToSlash(rel)
			}
		}
	}
	maxBytes := attachmentCfg.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultAttachmentMaxBytes
	}
	maxTotalBytes := attachmentCfg.MaxTotalBytes
	if maxTotalBytes <= 0 {
		maxTotalBytes = defaultAttachmentTotalMax
	}

	return &Connector{
		bot:                 bot,
		store:               store,
		httpClient:          &http.Client{Timeout: 2 * time.Minute},
		attachmentsRootRel:  rootRel,
		attachmentsRootAbs:  rootAbs,
		attachmentMaxBytes:  maxBytes,
		attachmentTotalMax:  maxTotalBytes,
		attachmentRetention: defaultAttachmentRetention,
		sweepEvery:          defaultAttachmentSweepEvery,
	}, nil
}

func (c *Connector) Run(ctx context.Context, handler gateway.Handler, mode RunMode, webhookOpts WebhookOptions) error {
	go c.runAttachmentSweeper(ctx)

	switch mode {
	case RunModePolling:
		return c.runPolling(ctx, handler)
	case RunModeWebhook:
		return c.runWebhook(ctx, handler, webhookOpts)
	default:
		return fmt.Errorf("unsupported telegram mode %q", mode)
	}
}

const metaKeyTelegramOffset = "telegram_update_offset"

func (c *Connector) loadOffset() int {
	if c.store == nil {
		return 0
	}
	raw := c.store.GetMeta(metaKeyTelegramOffset)
	if raw == "" {
		return 0
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return v
}

func (c *Connector) saveOffset(offset int) {
	if c.store == nil {
		return
	}
	_ = c.store.SetMeta(metaKeyTelegramOffset, strconv.Itoa(offset))
}

func (c *Connector) runPolling(ctx context.Context, handler gateway.Handler) error {
	if _, err := c.bot.Request(tgbotapi.DeleteWebhookConfig{DropPendingUpdates: false}); err != nil {
		return fmt.Errorf("delete webhook before polling: %w", err)
	}

	offset := c.loadOffset()
	u := tgbotapi.NewUpdate(offset)
	u.Timeout = 30
	updates := c.bot.GetUpdatesChan(u)
	defer c.bot.StopReceivingUpdates()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case update := <-updates:
			// Persist offset so restarts don't replay old messages
			c.saveOffset(update.UpdateID + 1)

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
		text = strings.TrimSpace(update.Message.Caption)
	}

	attachments, attachmentResults, failed, succeeded := c.processAttachments(ctx, update.Message)
	if text == "" && len(attachments) == 0 && len(failed) == 0 {
		return nil
	}

	if len(attachments)+len(failed) > 0 {
		fmt.Fprintf(os.Stderr,
			"telegram attachments summary: count=%d accepted=%d failed=%d bytes=%d\n",
			len(attachmentResults), len(succeeded), len(failed), sumAttachmentBytes(succeeded),
		)
	}
	msg := gateway.IncomingMessage{
		Channel:              "telegram",
		ChatID:               strconv.FormatInt(update.Message.Chat.ID, 10),
		UserID:               strconv.FormatInt(int64(update.Message.From.ID), 10),
		Text:                 text,
		MessageID:            strconv.Itoa(update.Message.MessageID),
		Attachments:          attachments,
		AttachmentResults:    attachmentResults,
		AttachmentsFailed:    failed,
		AttachmentsSucceeded: succeeded,
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

	// Try HTML-formatted message first
	htmlText := util.MarkdownToTelegramHTML(text)
	msg := tgbotapi.NewMessage(chat, htmlText)
	msg.ParseMode = "HTML"
	_, err = c.bot.Send(msg)
	if err == nil {
		return nil
	}

	// Fallback to plain text if Telegram rejects the HTML
	msg = tgbotapi.NewMessage(chat, text)
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
