package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"goated/internal/app"
)

type restartRecord struct {
	Timestamp string `json:"timestamp"`
	OldPID    int    `json:"old_pid,omitempty"`
	NewPID    string `json:"new_pid,omitempty"`
	Reason    string `json:"reason"`
}

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the goated daemon",
}

var daemonRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the daemon with a logged reason",
	RunE: func(cmd *cobra.Command, args []string) error {
		reason, _ := cmd.Flags().GetString("reason")
		if reason == "" {
			return fmt.Errorf("--reason is required")
		}

		cfg := app.LoadConfig()
		pidPath := filepath.Join(cfg.LogDir, "goated_daemon.pid")
		restartLog := filepath.Join(cfg.LogDir, "restarts.jsonl")

		if err := os.MkdirAll(cfg.LogDir, 0o755); err != nil {
			return fmt.Errorf("mkdir log dir: %w", err)
		}

		rec := restartRecord{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Reason:    reason,
		}

		// Stop existing daemon gracefully
		if oldPID, err := stopDaemon(pidPath); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		} else if oldPID > 0 {
			rec.OldPID = oldPID
			fmt.Printf("Stopped daemon (pid=%d)\n", oldPID)
		} else {
			fmt.Println("No running daemon found.")
		}

		// Start new daemon — resolve binary next to this executable, then cwd, then PATH
		daemonBin := ""
		candidates := []string{
			filepath.Join(filepath.Dir(os.Args[0]), "goated_daemon"),
			"./goated_daemon",
		}
		// Resolve os.Args[0] to absolute path for a better candidate
		if exe, err := os.Executable(); err == nil {
			candidates = append([]string{filepath.Join(filepath.Dir(exe), "goated_daemon")}, candidates...)
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				daemonBin = c
				break
			}
		}
		if daemonBin == "" {
			if p, err := exec.LookPath("goated_daemon"); err == nil {
				daemonBin = p
			} else {
				return fmt.Errorf("goated_daemon binary not found; run build.sh first")
			}
		}

		out, err := exec.Command(daemonBin).Output()
		if err != nil {
			return fmt.Errorf("start daemon: %w", err)
		}
		rec.NewPID = stripNL(string(out))
		fmt.Print(string(out))

		// Append restart record
		if err := appendRestartRecord(restartLog, rec); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to write restart log: %v\n", err)
		}

		return nil
	},
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := app.LoadConfig()
		pidPath := filepath.Join(cfg.LogDir, "goated_daemon.pid")

		pid, err := stopDaemon(pidPath)
		if err != nil {
			return err
		}
		if pid == 0 {
			fmt.Println("No running daemon found.")
		} else {
			fmt.Printf("Stopped daemon (pid=%d)\n", pid)
		}
		return nil
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status and recent restarts",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := app.LoadConfig()
		pidPath := filepath.Join(cfg.LogDir, "goated_daemon.pid")
		restartLog := filepath.Join(cfg.LogDir, "restarts.jsonl")

		// Check if running
		if pid, running := readPID(pidPath); running {
			fmt.Printf("Daemon running (pid=%d)\n", pid)
		} else if pid > 0 {
			fmt.Printf("Daemon not running (stale pid=%d)\n", pid)
		} else {
			fmt.Println("Daemon not running (no pid file)")
		}

		// Show recent restarts
		data, err := os.ReadFile(restartLog)
		if err != nil {
			return nil // no restart log yet
		}
		lines := splitLines(string(data))
		start := 0
		if len(lines) > 10 {
			start = len(lines) - 10
		}
		if len(lines) > 0 {
			fmt.Printf("\nRecent restarts (last %d):\n", len(lines)-start)
			for _, line := range lines[start:] {
				var rec restartRecord
				if json.Unmarshal([]byte(line), &rec) == nil {
					fmt.Printf("  %s  pid %d→%s  %s\n", rec.Timestamp, rec.OldPID, rec.NewPID, rec.Reason)
				}
			}
		}
		return nil
	},
}

// stopDaemon sends SIGTERM and waits for the process to exit (up to 3 minutes
// to allow in-flight messages to flush). Returns the old PID (0 if none).
func stopDaemon(pidPath string) (int, error) {
	pid, running := readPID(pidPath)
	if !running {
		return 0, nil
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return 0, nil
	}

	// Graceful: SIGTERM first — daemon will drain in-flight messages
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return pid, fmt.Errorf("sending SIGTERM to pid %d: %w", pid, err)
	}

	fmt.Printf("Sent SIGTERM to daemon (pid=%d), waiting for in-flight messages to flush...\n", pid)

	// Wait up to 3 minutes for graceful exit (allows message flush)
	for i := 0; i < 1800; i++ {
		time.Sleep(100 * time.Millisecond)
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			_ = os.Remove(pidPath)
			return pid, nil
		}
	}

	// Force kill if still alive after 3 minutes
	fmt.Fprintf(os.Stderr, "Daemon (pid=%d) didn't stop after 3m, sending SIGKILL\n", pid)
	_ = proc.Signal(syscall.SIGKILL)
	time.Sleep(200 * time.Millisecond)
	_ = os.Remove(pidPath)
	return pid, nil
}

// readPID reads the pid file and checks if the process is alive.
func readPID(pidPath string) (int, bool) {
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(stripNL(string(data)))
	if err != nil {
		return 0, false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return pid, false
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return pid, false
	}
	return pid, true
}

func appendRestartRecord(path string, rec restartRecord) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(rec)
}

func splitLines(s string) []string {
	var lines []string
	for _, l := range splitOn(s, '\n') {
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

func splitOn(s string, sep byte) []string {
	var result []string
	for {
		i := indexOf(s, sep)
		if i < 0 {
			result = append(result, s)
			break
		}
		result = append(result, s[:i])
		s = s[i+1:]
	}
	return result
}

func indexOf(s string, b byte) int {
	for i := range s {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func stripNL(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

func init() {
	daemonRestartCmd.Flags().String("reason", "", "reason for restarting (required)")
	daemonCmd.AddCommand(daemonRestartCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
	rootCmd.AddCommand(daemonCmd)
}
