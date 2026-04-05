package msglog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// HookContentKeys are the top-level keys within a hook event body that hold
// user/agent content and should be recursively redacted.
var HookContentKeys = []string{
	"prompt",
	"tool_input",
	"tool_response",
	"last_assistant_message",
}

// MsglogContentKeys are the top-level keys in a LogEntry whose values contain
// user/agent content and should be selectively redacted.
var MsglogContentKeys = []string{
	"user_message",
	"agent_response",
	"command",
	"error",
}

// ReRedactDate re-redacts all log files for a single date. This catches
// credential values that were added after those log entries were written.
func ReRedactDate(logDir, workspaceDir, date string) {
	credsDir := filepath.Join(workspaceDir, "creds")
	r := NewRedactor(credsDir)
	if !r.HasSecrets() {
		return
	}

	msglogDir := filepath.Join(logDir, "message_logs")

	// Daily message log
	reRedactJSONLFile(filepath.Join(msglogDir, "daily", date+".jsonl"), r, MsglogContentKeys, false)

	// Audit message log
	reRedactJSONLFile(filepath.Join(msglogDir, "audit", date+".jsonl"), r, MsglogContentKeys, false)

	// Session files for this date
	sessDir := filepath.Join(msglogDir, "sessions")
	if entries, err := os.ReadDir(sessDir); err == nil {
		prefix := date + "-"
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), prefix) && strings.HasSuffix(e.Name(), ".jsonl") {
				reRedactJSONLFile(filepath.Join(sessDir, e.Name()), r, MsglogContentKeys, false)
			}
		}
	}

	// Hook logs
	hookFile := filepath.Join(logDir, "claude_session", "hooks", date+".jsonl")
	reRedactJSONLFile(hookFile, r, HookContentKeys, true)

	// Run output files with modtime on the given date
	runsDir := filepath.Join(logDir, "claude_session", "runs")
	if entries, err := os.ReadDir(runsDir); err == nil {
		for _, e := range entries {
			info, err := e.Info()
			if err != nil {
				continue
			}
			if info.ModTime().Format("2006-01-02") == date {
				reRedactRawFile(filepath.Join(runsDir, e.Name()), r)
			}
		}
	}

	fmt.Fprintf(os.Stderr, "[%s] re-redacted logs for date %s\n",
		time.Now().Format(time.RFC3339), date)
}

// ReRedactRecentSessions re-redacts the most recent N session JSONL files.
func ReRedactRecentSessions(logDir, workspaceDir string, n int) {
	credsDir := filepath.Join(workspaceDir, "creds")
	r := NewRedactor(credsDir)
	if !r.HasSecrets() {
		return
	}

	sessDir := filepath.Join(logDir, "message_logs", "sessions")
	files := ListSessionFiles(sessDir)
	if len(files) == 0 {
		return
	}

	// Take the last N
	start := len(files) - n
	if start < 0 {
		start = 0
	}
	for _, path := range files[start:] {
		reRedactJSONLFile(path, r, MsglogContentKeys, false)
	}

	fmt.Fprintf(os.Stderr, "[%s] re-redacted %d recent session file(s)\n",
		time.Now().Format(time.RFC3339), len(files)-start)
}

// ReRedactRecentAll re-redacts the most recent N files of each log type.
func ReRedactRecentAll(logDir, workspaceDir string, n int) {
	credsDir := filepath.Join(workspaceDir, "creds")
	r := NewRedactor(credsDir)
	if !r.HasSecrets() {
		return
	}

	msglogDir := filepath.Join(logDir, "message_logs")

	// Daily logs
	reRedactRecentJSONL(filepath.Join(msglogDir, "daily"), r, MsglogContentKeys, false, n)

	// Audit logs
	reRedactRecentJSONL(filepath.Join(msglogDir, "audit"), r, MsglogContentKeys, false, n)

	// Session logs
	reRedactRecentJSONL(filepath.Join(msglogDir, "sessions"), r, MsglogContentKeys, false, n)

	// Hook logs
	reRedactRecentJSONL(filepath.Join(logDir, "claude_session", "hooks"), r, HookContentKeys, true, n)

	// Run output (raw files)
	reRedactRecentRaw(filepath.Join(logDir, "claude_session", "runs"), r, n)

	fmt.Fprintf(os.Stderr, "[%s] hourly re-redact: scrubbed recent %d file(s) per log type\n",
		time.Now().Format(time.RFC3339), n)
}

// reRedactRecentJSONL re-redacts the most recent N JSONL files in a directory.
func reRedactRecentJSONL(dir string, r *Redactor, contentKeys []string, hookMode bool, n int) {
	files := recentFiles(dir, ".jsonl", n)
	for _, path := range files {
		reRedactJSONLFile(path, r, contentKeys, hookMode)
	}
}

// reRedactRecentRaw re-redacts the most recent N files in a directory using full-string redaction.
func reRedactRecentRaw(dir string, r *Redactor, n int) {
	files := recentFiles(dir, "", n)
	for _, path := range files {
		reRedactRawFile(path, r)
	}
}

// recentFiles returns the most recent N files in a directory, sorted by mod time (newest last).
// If suffix is non-empty, only files ending with that suffix are included.
func recentFiles(dir string, suffix string, n int) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	type fileInfo struct {
		path    string
		modTime time.Time
	}
	var files []fileInfo
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if suffix != "" && !strings.HasSuffix(e.Name(), suffix) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, fileInfo{
			path:    filepath.Join(dir, e.Name()),
			modTime: info.ModTime(),
		})
	}

	// Sort by mod time ascending
	for i := 1; i < len(files); i++ {
		for j := i; j > 0 && files[j].modTime.Before(files[j-1].modTime); j-- {
			files[j], files[j-1] = files[j-1], files[j]
		}
	}

	start := len(files) - n
	if start < 0 {
		start = 0
	}
	var result []string
	for _, f := range files[start:] {
		result = append(result, f.path)
	}
	return result
}

// reRedactJSONLFile reads a JSONL file, re-redacts each line, and atomically
// rewrites the file. In hook mode, the redactable content is nested under
// the "body" key rather than at the top level.
func reRedactJSONLFile(path string, r *Redactor, contentKeys []string, hookMode bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return // file doesn't exist or can't read — skip silently
	}

	fl := NewFileLock(path)
	if err := fl.Lock(); err != nil {
		fmt.Fprintf(os.Stderr, "[reredact] lock error for %s: %v\n", path, err)
		return
	}
	defer fl.Unlock()

	lines := bytes.Split(data, []byte("\n"))
	var out [][]byte
	changed := false

	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		var m map[string]any
		if err := json.Unmarshal(line, &m); err != nil {
			// Can't parse — keep as-is
			out = append(out, line)
			continue
		}

		original := string(line)

		if hookMode {
			// Hook entries wrap content under "body"
			if body, ok := m["body"].(map[string]any); ok {
				r.RedactMapContainers(body, contentKeys)
			}
		} else {
			r.RedactMapContainers(m, contentKeys)
		}

		redacted, err := json.Marshal(m)
		if err != nil {
			out = append(out, line)
			continue
		}

		if string(redacted) != original {
			changed = true
		}
		out = append(out, redacted)
	}

	if !changed {
		return
	}

	// Atomic rewrite: write to temp file, then rename
	var buf bytes.Buffer
	for _, line := range out {
		buf.Write(line)
		buf.WriteByte('\n')
	}

	tmpPath := path + ".reredact.tmp"
	if err := os.WriteFile(tmpPath, buf.Bytes(), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "[reredact] write tmp %s: %v\n", tmpPath, err)
		return
	}
	if err := os.Rename(tmpPath, path); err != nil {
		fmt.Fprintf(os.Stderr, "[reredact] rename %s: %v\n", path, err)
		_ = os.Remove(tmpPath)
	}
}

// reRedactRawFile reads a file, applies full-string redaction, and atomically
// rewrites it if anything changed.
func reRedactRawFile(path string, r *Redactor) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	original := string(data)
	redacted := r.Redact(original)

	if redacted == original {
		return
	}

	fl := NewFileLock(path)
	if err := fl.Lock(); err != nil {
		fmt.Fprintf(os.Stderr, "[reredact] lock error for %s: %v\n", path, err)
		return
	}
	defer fl.Unlock()

	tmpPath := path + ".reredact.tmp"
	if err := os.WriteFile(tmpPath, []byte(redacted), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "[reredact] write tmp %s: %v\n", tmpPath, err)
		return
	}
	if err := os.Rename(tmpPath, path); err != nil {
		fmt.Fprintf(os.Stderr, "[reredact] rename %s: %v\n", path, err)
		_ = os.Remove(tmpPath)
	}
}
