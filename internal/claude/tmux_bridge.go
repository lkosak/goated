package claude

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"goated/internal/util"
)

type TmuxBridge struct {
	WorkspaceDir        string
	LogDir              string
	ContextWindowTokens int
}

func (b *TmuxBridge) SendAndWait(ctx context.Context, _ string, userPrompt string, timeout time.Duration) (string, error) {
	if err := b.EnsureSession(ctx); err != nil {
		return "", err
	}

	target := b.sessionName() + ":0.0"

	// Track line count before sending so we only search new lines
	beforeSnap, _ := capturePane(ctx, target)
	startLine := strings.Count(beforeSnap, "\n")

	wrapped := buildPromptEnvelope(userPrompt)
	if err := b.sendKeys(ctx, wrapped); err != nil {
		return "", err
	}
	return b.waitForResponse(ctx, target, startLine, timeout)
}

func (b *TmuxBridge) EnsureSession(ctx context.Context) error {
	if err := os.MkdirAll(b.WorkspaceDir, 0o755); err != nil {
		return fmt.Errorf("mkdir workspace dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(b.LogDir, "telegram"), 0o755); err != nil {
		return fmt.Errorf("mkdir log dir: %w", err)
	}

	session := b.sessionName()
	created := false
	if err := runTmux(ctx, "has-session", "-t", session); err != nil {
		cmd := fmt.Sprintf("cd %q && unset CLAUDECODE && claude --dangerously-skip-permissions", b.WorkspaceDir)
		if err := runTmux(ctx, "new-session", "-d", "-s", session, cmd); err != nil {
			return fmt.Errorf("start claude tmux session: %w", err)
		}
		created = true
	}
	if created {
		if err := waitForClaudeReady(ctx, session+":0.0", 25*time.Second); err != nil {
			return err
		}
	}
	return nil
}

func (b *TmuxBridge) ClearSession(ctx context.Context, _ string) error {
	session := b.sessionName()
	_ = runTmux(ctx, "kill-session", "-t", session)
	return b.EnsureSession(ctx)
}

func (b *TmuxBridge) ContextUsagePercent(_ string) int {
	// Rough estimate from scrollback size
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	target := b.sessionName() + ":0.0"
	out, err := capturePane(ctx, target)
	if err != nil {
		return 0
	}
	estTokens := len(out) / 4
	if b.ContextWindowTokens <= 0 {
		return 0
	}
	pct := estTokens * 100 / b.ContextWindowTokens
	if pct > 100 {
		return 100
	}
	return pct
}

func (b *TmuxBridge) sessionName() string {
	return "goat_main"
}

func (b *TmuxBridge) sendKeys(ctx context.Context, prompt string) error {
	tmp, err := os.CreateTemp("", "goat-prompt-*.txt")
	if err != nil {
		return fmt.Errorf("create temp prompt: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := io.WriteString(tmp, prompt); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp prompt: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp prompt: %w", err)
	}

	target := b.sessionName() + ":0.0"
	if err := runTmux(ctx, "load-buffer", "-b", "goat", tmp.Name()); err != nil {
		return fmt.Errorf("load-buffer: %w", err)
	}
	if err := runTmux(ctx, "paste-buffer", "-b", "goat", "-t", target); err != nil {
		return fmt.Errorf("paste-buffer: %w", err)
	}
	// Wait until Claude Code's input box shows the pasted text
	firstLine := strings.SplitN(prompt, "\n", 2)[0]
	if err := waitForPaneContains(ctx, target, firstLine, 5*time.Second); err != nil {
		return fmt.Errorf("paste not received: %w", err)
	}
	if err := runTmux(ctx, "send-keys", "-t", target, "Enter"); err != nil {
		return fmt.Errorf("send enter: %w", err)
	}
	return nil
}

// waitForResponse polls capture-pane for delimiters only in lines after startLine.
func (b *TmuxBridge) waitForResponse(ctx context.Context, target string, startLine int, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)

	for {
		if time.Now().After(deadline) {
			return "", fmt.Errorf("timed out waiting for claude response")
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		snap, err := capturePane(ctx, target)
		if err == nil {
			lines := strings.SplitAfter(snap, "\n")
			if len(lines) > startLine {
				newText := strings.Join(lines[startLine:], "")
				if msg := util.ExtractUserMessage(newText); msg != "" {
					return msg, nil
				}
			}
		}
		time.Sleep(1500 * time.Millisecond)
	}
}

// capturePane returns the full scrollback of a tmux pane as clean text.
func capturePane(ctx context.Context, target string) (string, error) {
	return runTmuxOutput(ctx, "capture-pane", "-t", target, "-p", "-S", "-")
}

func buildPromptEnvelope(userPrompt string) string {
	return fmt.Sprintf(`User message from connector:
%s

Read CLAUDE.md and return the user-visible response only with the configured user-message delimiter contract.
Do not include placeholder text.
Do not echo delimiter instructions back to the user.
`, strings.TrimSpace(userPrompt))
}

func runTmux(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "tmux", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tmux %s failed: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func runTmuxOutput(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "tmux", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("tmux %s failed: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func waitForPaneContains(ctx context.Context, target, needle string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		out, err := capturePane(ctx, target)
		if err == nil && strings.Contains(out, needle) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for %q in pane", needle)
}

func waitForClaudeReady(ctx context.Context, target string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		out, err := capturePane(ctx, target)
		if err == nil {
			if strings.Contains(out, "Claude Code") && strings.Contains(out, "❯") {
				return nil
			}
		}
		time.Sleep(350 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for Claude session readiness")
}

func (b *TmuxBridge) StopSession(ctx context.Context) error {
	session := b.sessionName()
	if err := runTmux(ctx, "kill-session", "-t", session); err != nil {
		if strings.Contains(err.Error(), "can't find session") || strings.Contains(err.Error(), "no server running") {
			return nil
		}
		return err
	}
	return nil
}
