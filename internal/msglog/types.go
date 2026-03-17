package msglog

// EntryType identifies the kind of log entry.
type EntryType string

const (
	EntryUserMessage   EntryType = "user_message"
	EntryAgentResponse EntryType = "agent_response"
	EntryCommand       EntryType = "command"
	EntryEvent         EntryType = "event"
)

// MessageStatus tracks the delivery lifecycle of a message.
type MessageStatus string

const (
	StatusPending       MessageStatus = "pending"
	StatusSentToAgent   MessageStatus = "sent_to_agent"
	StatusAgentReceived MessageStatus = "agent_received"
	StatusSent          MessageStatus = "sent"
	StatusFailed        MessageStatus = "failed"
)

// LogEntry is the core struct serialized as one JSONL line per append.
type LogEntry struct {
	TS        string        `json:"ts"`
	TSUnix    int64         `json:"ts_unix"`
	RequestID string        `json:"request_id"`
	Type      EntryType     `json:"type"`
	Status    MessageStatus `json:"status,omitempty"`
	SessionID string        `json:"session_id,omitempty"`
	SessionSeq string       `json:"session_seq,omitempty"`

	UserMessage   *UserMessageData   `json:"user_message,omitempty"`
	AgentResponse *AgentResponseData `json:"agent_response,omitempty"`
	Command       *CommandData       `json:"command,omitempty"`
	Event         *EventData         `json:"event,omitempty"`
	Error         string             `json:"error,omitempty"`
}

// UserMessageData captures details about an incoming user message.
type UserMessageData struct {
	Channel         string `json:"channel"`
	ChatID          string `json:"chat_id"`
	UserID          string `json:"user_id,omitempty"`
	Text            string `json:"text"`
	MessageID       string `json:"message_id,omitempty"`
	ThreadID        string `json:"thread_id,omitempty"`
	HasAttachments  bool   `json:"has_attachments,omitempty"`
	AttachmentCount int    `json:"attachment_count,omitempty"`
}

// AgentResponseData captures details about an agent response delivery.
type AgentResponseData struct {
	ChatID  string `json:"chat_id"`
	Gateway string `json:"gateway"`
	Text    string `json:"text"`
	TextLen int    `json:"text_len"`
}

// CommandData captures a command invocation (e.g. /clear, /chatid).
type CommandData struct {
	Name   string `json:"name"`
	ChatID string `json:"chat_id"`
}

// EventData captures internal system events (compact, replay, health check, etc).
type EventData struct {
	Name   string `json:"name"`
	Detail string `json:"detail,omitempty"`
}

// SidecarMeta is the structure written to .meta.json alongside each JSONL file.
type SidecarMeta struct {
	CreatedAt      string `json:"created_at"`
	Timezone       string `json:"timezone"`
	GoatedVersion  string `json:"goated_version,omitempty"`
	Runtime        string `json:"runtime,omitempty"`
	SessionID      string `json:"session_id,omitempty"`
	SessionSeq     string `json:"session_seq,omitempty"`
	ChatID         string `json:"chat_id,omitempty"`
	Channel        string `json:"channel,omitempty"`
	LastModified   string `json:"last_modified"`
	EntryCount     int    `json:"entry_count"`
}
