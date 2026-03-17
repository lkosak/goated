package msglog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Logger writes structured JSONL entries to three destinations: daily, session, and audit.
type Logger struct {
	baseDir    string // logs/message_logs/
	tz         *time.Location
	redactor   *Redactor
	sessionMgr *SessionFileManager
}

// NewLogger creates a Logger that writes to logs/message_logs/ under logDir.
// It initializes the redactor from workspace/creds/*.txt.
func NewLogger(logDir, workspaceDir, timezone string) (*Logger, error) {
	tz, err := time.LoadLocation(timezone)
	if err != nil {
		tz = time.FixedZone("UTC", 0)
	}

	baseDir := filepath.Join(logDir, "message_logs")
	for _, sub := range []string{"daily", "sessions", "audit"} {
		if err := os.MkdirAll(filepath.Join(baseDir, sub), 0o755); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", sub, err)
		}
	}

	credsDir := filepath.Join(workspaceDir, "creds")
	redactor := NewRedactor(credsDir)

	sessionDir := filepath.Join(baseDir, "sessions")
	sessionMgr := NewSessionFileManager(sessionDir, tz)

	return &Logger{
		baseDir:    baseDir,
		tz:         tz,
		redactor:   redactor,
		sessionMgr: sessionMgr,
	}, nil
}

// NewCLILogger creates a lightweight Logger for the goat CLI process.
// It writes to daily + audit + session (reading current session seq from marker file).
func NewCLILogger(logDir, timezone, workspaceDir string) *Logger {
	tz, err := time.LoadLocation(timezone)
	if err != nil {
		tz = time.FixedZone("UTC", 0)
	}

	baseDir := filepath.Join(logDir, "message_logs")
	credsDir := filepath.Join(workspaceDir, "creds")

	// Ensure dirs exist (best-effort for CLI)
	for _, sub := range []string{"daily", "sessions", "audit"} {
		_ = os.MkdirAll(filepath.Join(baseDir, sub), 0o755)
	}

	sessionDir := filepath.Join(baseDir, "sessions")
	redactor := NewRedactor(credsDir)
	sessionMgr := NewSessionFileManager(sessionDir, tz)

	return &Logger{
		baseDir:    baseDir,
		tz:         tz,
		redactor:   redactor,
		sessionMgr: sessionMgr,
	}
}

// SessionManager returns the session file manager for lifecycle coordination.
func (l *Logger) SessionManager() *SessionFileManager {
	return l.sessionMgr
}

// DailyDir returns the path to the daily logs directory.
func (l *Logger) DailyDir() string {
	return filepath.Join(l.baseDir, "daily")
}

// AuditDir returns the path to the audit logs directory.
func (l *Logger) AuditDir() string {
	return filepath.Join(l.baseDir, "audit")
}

// LogUserMessage logs an incoming user message to all three destinations.
func (l *Logger) LogUserMessage(requestID string, data UserMessageData, status MessageStatus) {
	entry := l.newEntry(requestID, EntryUserMessage, status)
	entry.UserMessage = &data
	l.writeConversation(entry)
	l.writeAudit(entry)
}

// LogAgentResponse logs an agent response to all three destinations.
func (l *Logger) LogAgentResponse(requestID string, data AgentResponseData, status MessageStatus, errMsg string) {
	entry := l.newEntry(requestID, EntryAgentResponse, status)
	entry.AgentResponse = &data
	entry.Error = errMsg
	l.writeConversation(entry)
	l.writeAudit(entry)
}

// LogCommand logs a command invocation to the audit log only.
func (l *Logger) LogCommand(requestID string, data CommandData) {
	entry := l.newEntry(requestID, EntryCommand, "")
	entry.Command = &data
	l.writeAudit(entry)
}

// LogEvent logs a system event to the audit log only.
func (l *Logger) LogEvent(requestID string, data EventData) {
	entry := l.newEntry(requestID, EntryEvent, "")
	entry.Event = &data
	l.writeAudit(entry)
}

// UpdateStatus appends a status-change entry for the given request ID.
func (l *Logger) UpdateStatus(requestID string, entryType EntryType, status MessageStatus) {
	entry := l.newEntry(requestID, entryType, status)
	l.writeConversation(entry)
	l.writeAudit(entry)
}

// newEntry creates a LogEntry with timestamps and session info.
func (l *Logger) newEntry(requestID string, typ EntryType, status MessageStatus) LogEntry {
	now := time.Now().In(l.tz)
	return LogEntry{
		TS:         now.Format(time.RFC3339),
		TSUnix:     now.Unix(),
		RequestID:  requestID,
		Type:       typ,
		Status:     status,
		SessionID:  l.sessionMgr.SessionID(),
		SessionSeq: l.sessionMgr.CurrentSeq(),
	}
}

// writeConversation writes to daily + session logs (conversation entries only).
func (l *Logger) writeConversation(entry LogEntry) {
	now := time.Now().In(l.tz)
	dailyPath := filepath.Join(l.baseDir, "daily", now.Format("2006-01-02")+".jsonl")
	l.appendEntry(dailyPath, entry)

	if sessionPath := l.sessionMgr.CurrentPath(); sessionPath != "" {
		l.appendEntry(sessionPath, entry)
	}
}

// writeAudit writes to the audit log.
func (l *Logger) writeAudit(entry LogEntry) {
	now := time.Now().In(l.tz)
	auditPath := filepath.Join(l.baseDir, "audit", now.Format("2006-01-02")+".jsonl")
	l.appendEntry(auditPath, entry)
}

// msglogContentKeys aliases the exported MsglogContentKeys for local use.
var msglogContentKeys = MsglogContentKeys

// appendEntry serializes an entry as JSON, selectively redacts content
// containers, and appends under an exclusive file lock.
func (l *Logger) appendEntry(path string, entry LogEntry) {
	line, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[msglog] marshal error: %v\n", err)
		return
	}

	// Selective redaction: unmarshal to map, redact only content containers,
	// re-marshal. This prevents metadata fields (timestamps, IDs, counts)
	// from being touched even if a short credential value appears in them.
	var m map[string]any
	if err := json.Unmarshal(line, &m); err == nil {
		l.redactor.RedactMapContainers(m, msglogContentKeys)
		if redacted, err := json.Marshal(m); err == nil {
			line = redacted
		}
	}

	fl := NewFileLock(path)
	if err := fl.Lock(); err != nil {
		fmt.Fprintf(os.Stderr, "[msglog] lock error for %s: %v\n", path, err)
		return
	}
	defer fl.Unlock()

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[msglog] open error for %s: %v\n", path, err)
		return
	}
	defer f.Close()

	_, _ = f.WriteString(string(line) + "\n")

	// Update sidecar (best-effort)
	l.updateSidecar(path, entry)
}

// updateSidecar reads or creates the .meta.json sidecar and updates it.
func (l *Logger) updateSidecar(jsonlPath string, entry LogEntry) {
	metaPath := jsonlPath + ".meta.json"
	now := time.Now().In(l.tz)

	var meta SidecarMeta

	// Try to read existing
	if data, err := os.ReadFile(metaPath); err == nil {
		_ = json.Unmarshal(data, &meta)
	}

	if meta.CreatedAt == "" {
		meta.CreatedAt = now.Format(time.RFC3339)
	}
	meta.LastModified = now.Format(time.RFC3339)
	meta.Timezone = l.tz.String()
	meta.EntryCount++

	if entry.SessionID != "" && meta.SessionID == "" {
		meta.SessionID = entry.SessionID
	}
	if entry.SessionSeq != "" && meta.SessionSeq == "" {
		meta.SessionSeq = entry.SessionSeq
	}

	// Atomic write via temp + rename
	content, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return
	}
	content = append(content, '\n')

	tmpPath := metaPath + ".tmp"
	if err := os.WriteFile(tmpPath, content, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmpPath, metaPath)
}
