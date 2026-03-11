package gateway

import (
	"context"
	"fmt"
	"strings"
	"time"

	"goated/internal/claude"
	"goated/internal/db"
)

type Service struct {
	Bridge          *claude.TmuxBridge
	Store           *db.Store
	DefaultTimezone string
}

func (s *Service) HandleMessage(ctx context.Context, msg IncomingMessage, responder Responder) error {
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return nil
	}

	switch {
	case strings.EqualFold(text, "/clear"):
		if err := s.Bridge.ClearSession(ctx, msg.ChatID); err != nil {
			return responder.SendMessage(ctx, msg.ChatID, "Failed to clear session: "+err.Error())
		}
		return responder.SendMessage(ctx, msg.ChatID, "Started a new Claude session and rotated the chat log.")
	case strings.EqualFold(text, "/context"):
		pct := s.Bridge.ContextUsagePercent(msg.ChatID)
		return responder.SendMessage(ctx, msg.ChatID, fmt.Sprintf("Approx context usage: %d%%", pct))
	case strings.HasPrefix(text, "/schedule "):
		return s.handleScheduleCommand(ctx, msg, responder)
	}

	response, err := s.Bridge.SendAndWait(ctx, msg.ChatID, text, 3*time.Minute)
	if err != nil {
		return responder.SendMessage(ctx, msg.ChatID, "Claude session error: "+err.Error())
	}
	if response == "" {
		response = "No delimited user message found. Ensure Claude responded with :START_USER_MESSAGE: ... :END_USER_MESSAGE:."
	}
	return responder.SendMessage(ctx, msg.ChatID, response)
}

func (s *Service) handleScheduleCommand(ctx context.Context, msg IncomingMessage, responder Responder) error {
	payload := strings.TrimSpace(strings.TrimPrefix(msg.Text, "/schedule"))
	parts := strings.SplitN(payload, "|", 2)
	if len(parts) != 2 {
		return responder.SendMessage(ctx, msg.ChatID, "Usage: /schedule <cron_expr> | <prompt>")
	}
	schedule := strings.TrimSpace(parts[0])
	prompt := strings.TrimSpace(parts[1])
	if schedule == "" || prompt == "" {
		return responder.SendMessage(ctx, msg.ChatID, "Both cron expression and prompt are required.")
	}
	_, err := s.Store.AddCron(msg.ChatID, schedule, prompt, s.DefaultTimezone)
	if err != nil {
		return responder.SendMessage(ctx, msg.ChatID, "Failed to save schedule: "+err.Error())
	}
	return responder.SendMessage(ctx, msg.ChatID, "Saved scheduled job.")
}
