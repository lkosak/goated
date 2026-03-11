package subagent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"goated/internal/db"
)

// BuildPrompt constructs the prompt for a headless subagent.
// preamble is an optional prefix (e.g. "Read CRON.md before executing.").
// chatID, if non-empty, adds send_user_message instructions.
// source and logPath are passed through so the main session gets context.
func BuildPrompt(preamble, userPrompt, chatID, source, logPath string) string {
	var b strings.Builder
	if preamble != "" {
		b.WriteString(preamble)
		b.WriteString("\n\n")
	}
	b.WriteString(strings.TrimSpace(userPrompt))
	b.WriteString("\n")
	if chatID != "" {
		b.WriteString("\nSend your response to the user by piping markdown into:\n")
		sendCmd := fmt.Sprintf("  ./goat send_user_message --chat %s", chatID)
		if source != "" {
			sendCmd += fmt.Sprintf(" --source %s", source)
		}
		if logPath != "" {
			sendCmd += fmt.Sprintf(" --log %s", logPath)
		}
		b.WriteString(sendCmd + "\n")
		b.WriteString("\nSee GOATED_CLI_README.md for formatting details.\n")
	}
	return b.String()
}

// RunSync runs a subagent synchronously, blocking until it completes.
// Tracks the run in the database if store is non-nil.
func RunSync(ctx context.Context, store *db.Store, opts RunOpts) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "claude", "--dangerously-skip-permissions", "-p", opts.Prompt)
	cmd.Dir = opts.WorkspaceDir
	cmd.Env = filterEnv(os.Environ(), "CLAUDECODE")

	// Start (not Run) so we can capture PID for tracking
	outFile, err := os.Create(opts.LogPath)
	if err != nil {
		return nil, fmt.Errorf("create log %s: %w", opts.LogPath, err)
	}
	cmd.Stdout = outFile
	cmd.Stderr = outFile

	if err := cmd.Start(); err != nil {
		outFile.Close()
		return nil, fmt.Errorf("start subagent: %w", err)
	}

	var runID uint64
	if store != nil {
		runID, _ = store.RecordSubagentStart(cmd.Process.Pid, opts.Source, opts.CronID, opts.ChatID, opts.Prompt, opts.LogPath)
	}

	runErr := cmd.Wait()
	outFile.Close()

	output, _ := os.ReadFile(opts.LogPath)

	if store != nil && runID > 0 {
		status := "ok"
		if runErr != nil {
			status = "error"
		}
		_ = store.RecordSubagentFinish(runID, status)
	}

	return output, runErr
}

// RunBackground starts a subagent in the background and returns immediately.
// Tracks the run in the database if store is non-nil.
func RunBackground(store *db.Store, opts RunOpts) (pid int, err error) {
	f, err := os.Create(opts.LogPath)
	if err != nil {
		return 0, fmt.Errorf("create log file: %w", err)
	}

	cmd := exec.Command("claude", "--dangerously-skip-permissions", "-p", opts.Prompt)
	cmd.Dir = opts.WorkspaceDir
	cmd.Stdout = f
	cmd.Stderr = f
	cmd.Env = filterEnv(os.Environ(), "CLAUDECODE")

	if err := cmd.Start(); err != nil {
		f.Close()
		return 0, fmt.Errorf("start subagent: %w", err)
	}

	pid = cmd.Process.Pid

	var runID uint64
	if store != nil {
		runID, _ = store.RecordSubagentStart(pid, opts.Source, opts.CronID, opts.ChatID, opts.Prompt, opts.LogPath)
	}

	go func() {
		runErr := cmd.Wait()
		f.Close()
		if store != nil && runID > 0 {
			status := "ok"
			if runErr != nil {
				status = "error"
			}
			_ = store.RecordSubagentFinish(runID, status)
		}
	}()

	return pid, nil
}

// RunOpts configures a subagent run.
type RunOpts struct {
	WorkspaceDir string
	Prompt       string
	LogPath      string
	Source       string // "cron", "cli", "gateway"
	CronID       uint64 // only for cron-sourced runs
	ChatID       string
}

func filterEnv(env []string, remove string) []string {
	prefix := remove + "="
	var out []string
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			out = append(out, e)
		}
	}
	return out
}
