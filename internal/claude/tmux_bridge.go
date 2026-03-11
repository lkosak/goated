package claude

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"goat/internal/util"
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
	logPath := b.sessionLogPath()
	offset := int64(0)
	if st, err := os.Stat(logPath); err == nil {
		offset = st.Size()
	}

	wrapped := buildPromptEnvelope(userPrompt)
	if err := b.sendKeys(ctx, wrapped); err != nil {
		return "", err
	}
	return waitForUserMessage(ctx, logPath, offset, timeout)
}

func (b *TmuxBridge) EnsureSession(ctx context.Context) error {
	if err := os.MkdirAll(b.WorkspaceDir, 0o755); err != nil {
		return fmt.Errorf("mkdir workspace dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(b.sessionLogPath()), 0o755); err != nil {
		return fmt.Errorf("mkdir log dir: %w", err)
	}

	session := b.sessionName()
	created := false
	if err := runTmux(ctx, "has-session", "-t", session); err != nil {
		cmd := fmt.Sprintf("cd %q && claude --dangerously-skip-permissions", b.WorkspaceDir)
		if err := runTmux(ctx, "new-session", "-d", "-s", session, cmd); err != nil {
			return fmt.Errorf("start claude tmux session: %w", err)
		}
		created = true
	}
	if err := b.attachPipe(ctx); err != nil {
		return err
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
	_ = runTmux(ctx, "pipe-pane", "-t", session+":0.0")
	_ = runTmux(ctx, "kill-session", "-t", session)

	logPath := b.sessionLogPath()
	if st, err := os.Stat(logPath); err == nil && st.Size() > 0 {
		ts := time.Now().Format("20060102-150405")
		archived := strings.TrimSuffix(logPath, ".log") + "-" + ts + ".log"
		if err := os.Rename(logPath, archived); err != nil {
			return fmt.Errorf("rotate log: %w", err)
		}
	}
	return b.EnsureSession(ctx)
}

func (b *TmuxBridge) ContextUsagePercent(_ string) int {
	path := b.sessionLogPath()
	st, err := os.Stat(path)
	if err != nil {
		return 0
	}
	estTokens := int(st.Size() / 4)
	if b.ContextWindowTokens <= 0 {
		return 0
	}
	pct := estTokens * 100 / b.ContextWindowTokens
	if pct > 100 {
		return 100
	}
	if pct < 0 {
		return 0
	}
	return pct
}

func (b *TmuxBridge) sessionName() string {
	return "goat_main"
}

func (b *TmuxBridge) sessionLogPath() string {
	return filepath.Join(b.LogDir, "telegram", "main.log")
}

func (b *TmuxBridge) attachPipe(ctx context.Context) error {
	target := b.sessionName() + ":0.0"
	logPath := b.sessionLogPath()
	pipe := fmt.Sprintf("cat >> %q", logPath)
	if err := runTmux(ctx, "pipe-pane", "-t", target); err != nil {
		return fmt.Errorf("stop existing pipe-pane: %w", err)
	}
	if err := runTmux(ctx, "pipe-pane", "-t", target, "-o", pipe); err != nil {
		return fmt.Errorf("attach pipe-pane: %w", err)
	}
	return nil
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
	if err := runTmux(ctx, "send-keys", "-t", target, "C-m"); err != nil {
		return fmt.Errorf("send enter: %w", err)
	}
	return nil
}

func waitForUserMessage(ctx context.Context, logPath string, offset int64, timeout time.Duration) (string, error) {
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

		chunk, err := readFromOffset(logPath, offset)
		if err == nil {
			if msg := util.ExtractUserMessage(chunk); msg != "" {
				return msg, nil
			}
		}
		time.Sleep(1200 * time.Millisecond)
	}
}

func readFromOffset(path string, offset int64) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, f); err != nil {
		return "", err
	}
	return buf.String(), nil
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

func waitForClaudeReady(ctx context.Context, target string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		out, err := runTmuxOutput(ctx, "capture-pane", "-t", target, "-p", "-S", "-80")
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
	_ = runTmux(ctx, "pipe-pane", "-t", session+":0.0")
	if err := runTmux(ctx, "kill-session", "-t", session); err != nil {
		// Ignore "no such session" style failures.
		if strings.Contains(err.Error(), "can't find session") || strings.Contains(err.Error(), "no server running") {
			return nil
		}
		return err
	}
	return nil
}
