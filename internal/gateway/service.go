package gateway

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"goated/internal/claude"
	"goated/internal/db"
)

type Service struct {
	Bridge          *claude.TmuxBridge
	Store           *db.Store
	DefaultTimezone string

	// DrainCtx is a context that stays alive during graceful shutdown so
	// in-flight handlers can finish. Set this to a context that only cancels
	// after the flush timeout. If nil, the caller-provided ctx is used.
	DrainCtx context.Context

	inflight sync.WaitGroup
}

// WaitInflight blocks until all in-flight message handlers have completed.
func (s *Service) WaitInflight() {
	s.inflight.Wait()
}

func (s *Service) handleCtx(callerCtx context.Context) context.Context {
	if s.DrainCtx != nil {
		return s.DrainCtx
	}
	return callerCtx
}

func (s *Service) HandleMessage(ctx context.Context, msg IncomingMessage, responder Responder) error {
	s.inflight.Add(1)
	defer s.inflight.Done()

	// Use drain context so in-flight work survives gateway shutdown
	ctx = s.handleCtx(ctx)

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
	case strings.EqualFold(text, "/chatid"):
		return responder.SendMessage(ctx, msg.ChatID, fmt.Sprintf("Your chat ID is: %s", msg.ChatID))
	case strings.EqualFold(text, "/context"):
		pct := s.Bridge.ContextUsagePercent(msg.ChatID)
		return responder.SendMessage(ctx, msg.ChatID, fmt.Sprintf("Approx context usage: %d%%", pct))
	case strings.HasPrefix(text, "/schedule "):
		return s.handleScheduleCommand(ctx, msg, responder)
	}

	if err := s.Bridge.SendAndWait(ctx, msg.ChatID, text, 30*time.Minute); err != nil {
		return responder.SendMessage(ctx, msg.ChatID, friendlyError(err))
	}
	// Claude sends its response directly via ./goat send_user_message
	return nil
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

func friendlyError(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "The bot was restarted while processing your message. Please send it again."
	case errors.Is(err, context.DeadlineExceeded):
		return "Claude took too long to respond (timed out). Try again or simplify your request."
	case strings.Contains(err.Error(), "timed out waiting for claude response"):
		return "Claude didn't finish in time. Try again or use /clear to start a fresh session."
	case strings.Contains(err.Error(), "timed out waiting for Claude session readiness"):
		return "Claude session failed to start. Try /clear to reset, or check that the daemon is healthy."
	case strings.Contains(err.Error(), "paste not received"):
		return "Failed to send your message to Claude. The session may be stuck — try /clear."
	default:
		return "Something went wrong talking to Claude: " + err.Error()
	}
}
