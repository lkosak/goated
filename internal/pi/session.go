package pi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"goated/internal/agent"
	"goated/internal/db"
	"goated/internal/msglog"
)

const postSendWaitTimeout = 6 * time.Minute
const piWarmupPrompt = "You are being initialized. Say 'ready' — nothing else."
const piActiveSessionMetaKey = "runtime.pi.active_session_id"
const piSessionTurnCountKey = "runtime.pi.session_turn_count"

// piDeliveryPreamble is prepended to every user/batch prompt envelope sent to
// Pi. Pi's default behavior when handed a JSON envelope is to answer inline in
// its own output — which never reaches the user because Goated delivers
// replies through the shell command in the envelope's `respond_with` field.
// This preamble forces Pi to shell out instead of answering in its own
// response text.
const piDeliveryPreamble = `CRITICAL DELIVERY CONTRACT — READ BEFORE RESPONDING

You are running inside the Goated runtime. Your chat output is NOT visible to
the user. The only way to deliver a reply is to execute the shell command in
the envelope's ` + "`respond_with`" + ` field, piping your markdown answer into it via
stdin.

Required workflow for every user message:

1. Parse the envelope JSON below.
2. Read the file named in ` + "`formatting`" + ` (e.g. SLACK_MESSAGE_FORMATTING.md) if you
   have not already read it in this session.
3. Write your markdown reply and deliver it by piping into respond_with. For
   example, if respond_with is ` + "`./goat send_user_message --chat CXXXX`" + `, run:

       echo "Your markdown reply here" | ./goat send_user_message --chat CXXXX

   Use a heredoc for multi-line replies:

       ./goat send_user_message --chat CXXXX <<'EOF'
       Your markdown reply here.
       EOF

4. After the command exits successfully, stop. Do not restate the reply in
   your own output.

Rules:
- Any text you write outside the respond_with command is invisible to the user
  and will cause the message to appear dropped.
- Always send SOMETHING via respond_with, even for trivial questions. Silence
  is a bug.
- If the task will take more than ~30 seconds, send a short plan message
  through respond_with first, then do the work, then send the final reply.

ENVELOPE:
`

type SessionRuntime struct {
	workspaceDir string
	logDir       string
	dbPath       string
	provider     string // optional: pin pi --provider
	model        string // optional: pin pi --model
	redactor     *msglog.Redactor

	mu         sync.Mutex
	proc       *exec.Cmd
	procErr    error
	lastStderr string
	done       chan struct{}
}

func NewSessionRuntime(workspaceDir, logDir, dbPath, provider, model string) *SessionRuntime {
	credsDir := filepath.Join(workspaceDir, "creds")
	return &SessionRuntime{
		workspaceDir: workspaceDir,
		logDir:       logDir,
		dbPath:       dbPath,
		provider:     strings.TrimSpace(provider),
		model:        strings.TrimSpace(model),
		redactor:     msglog.NewRedactor(credsDir),
	}
}

func (r *SessionRuntime) Descriptor() agent.RuntimeDescriptor {
	return agent.RuntimeDescriptor{
		Provider:    agent.RuntimePi,
		DisplayName: "Pi",
		SessionName: "",
		Capabilities: agent.Capabilities{
			SupportsInteractiveSession: false,
			SupportsContextEstimate:    false,
			SupportsCompaction:         false,
			SupportsReset:              true,
		},
	}
}

func (r *SessionRuntime) sessionDir() string {
	return filepath.Join(r.logDir, "pi_session")
}

func (r *SessionRuntime) runsDir() string {
	return filepath.Join(r.sessionDir(), "runs")
}

func (r *SessionRuntime) piSessionsRootDir() string {
	return filepath.Join(r.sessionDir(), "sessions")
}

func (r *SessionRuntime) sessionDirFor(id string) string {
	return filepath.Join(r.piSessionsRootDir(), id)
}

func (r *SessionRuntime) baseArgs(sessionDir string) []string {
	args := []string{
		"--mode", "json",
		"--session-dir", sessionDir,
	}
	if r.provider != "" {
		args = append(args, "--provider", r.provider)
	}
	if r.model != "" {
		args = append(args, "--model", r.model)
	}
	return args
}

func (r *SessionRuntime) hasSavedSession(sessionDir string) bool {
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		return false
	}
	return len(entries) > 0
}

func (r *SessionRuntime) EnsureSession(ctx context.Context) error {
	for _, dir := range []string{r.sessionDir(), r.runsDir(), r.piSessionsRootDir()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}
	sessionID, err := r.ensureActiveSessionID()
	if err != nil {
		return err
	}
	sessionDir := r.sessionDirFor(sessionID)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", sessionDir, err)
	}
	if !r.hasSavedSession(sessionDir) {
		// New or cleared session — reset turn count so the first real
		// message gets the workspace-file preamble.
		r.resetTurnCount()
		if err := r.warmUpSession(ctx, sessionDir); err != nil {
			fmt.Fprintf(os.Stderr, "[%s] pi session warm-up failed: %v\n", time.Now().Format(time.RFC3339), err)
			// Non-fatal — the first real message will still create the session.
		}
	}
	return nil
}

func (r *SessionRuntime) warmUpSession(ctx context.Context, sessionDir string) error {
	args := []string{"-p", piWarmupPrompt}
	args = append(args, r.baseArgs(sessionDir)...)

	cmd := exec.CommandContext(ctx, "pi", args...)
	cmd.Dir = r.workspaceDir

	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	if _, err := cmd.Output(); err != nil {
		return fmt.Errorf("warm-up pi: %w (stderr: %s)", err, stderrBuf.String())
	}
	if !r.hasSavedSession(sessionDir) {
		return fmt.Errorf("warm-up did not create a persisted pi session")
	}
	return nil
}

func (r *SessionRuntime) store() (*db.Store, error) {
	return db.Open(r.dbPath)
}

func (r *SessionRuntime) activeSessionID() (string, error) {
	store, err := r.store()
	if err != nil {
		return "", err
	}
	defer store.Close()
	return strings.TrimSpace(store.GetMeta(piActiveSessionMetaKey)), nil
}

func (r *SessionRuntime) setActiveSessionID(id string) error {
	store, err := r.store()
	if err != nil {
		return err
	}
	defer store.Close()
	if err := store.SetMeta(piActiveSessionMetaKey, id); err != nil {
		return err
	}
	// Reset turn count for new session
	return store.SetMeta(piSessionTurnCountKey, "0")
}

func (r *SessionRuntime) sessionTurnCount() int {
	store, err := r.store()
	if err != nil {
		return -1
	}
	defer store.Close()
	val := store.GetMeta(piSessionTurnCountKey)
	n, _ := fmt.Sscanf(val, "%d", new(int))
	if n == 0 {
		return 0
	}
	var count int
	fmt.Sscanf(val, "%d", &count)
	return count
}

func (r *SessionRuntime) resetTurnCount() {
	store, err := r.store()
	if err != nil {
		return
	}
	defer store.Close()
	store.SetMeta(piSessionTurnCountKey, "0")
}

func (r *SessionRuntime) incrementTurnCount() {
	store, err := r.store()
	if err != nil {
		return
	}
	defer store.Close()
	val := store.GetMeta(piSessionTurnCountKey)
	var count int
	fmt.Sscanf(val, "%d", &count)
	store.SetMeta(piSessionTurnCountKey, fmt.Sprintf("%d", count+1))
}

func (r *SessionRuntime) ensureActiveSessionID() (string, error) {
	id, err := r.activeSessionID()
	if err != nil {
		return "", err
	}
	if id != "" {
		return id, nil
	}
	id, err = newPiSessionID()
	if err != nil {
		return "", err
	}
	if err := r.setActiveSessionID(id); err != nil {
		return "", err
	}
	return id, nil
}

func newPiSessionID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("generate pi session id: %w", err)
	}
	return hex.EncodeToString(buf[:]), nil
}

func (r *SessionRuntime) SendUserPrompt(ctx context.Context, channel, chatID, userPrompt string, attachments *agent.MessageAttachments, messageID, threadID string) error {
	envelope := agent.BuildPromptEnvelope(channel, chatID, userPrompt, attachments, messageID, threadID)
	return r.sendEnvelope(ctx, piDeliveryPreamble+envelope)
}

func (r *SessionRuntime) SendBatchPrompt(ctx context.Context, channel, chatID string, messages []agent.PromptMessage) error {
	envelope := agent.BuildBatchEnvelope(channel, chatID, messages)
	return r.sendEnvelope(ctx, piDeliveryPreamble+envelope)
}

func (r *SessionRuntime) SendSystemNotice(ctx context.Context, channel, chatID, source, message string, metadata map[string]string) error {
	envelope := agent.BuildSystemNoticeEnvelope(channel, chatID, source, message, metadata)
	return r.sendEnvelope(ctx, envelope)
}

func (r *SessionRuntime) sendEnvelope(ctx context.Context, envelope string) error {
	if err := r.EnsureSession(ctx); err != nil {
		return err
	}
	sessionID, err := r.ensureActiveSessionID()
	if err != nil {
		return err
	}
	sessionDir := r.sessionDirFor(sessionID)

	firstMessage := r.sessionTurnCount() == 0
	if firstMessage {
		// Insert preamble between delivery contract and envelope so the
		// workspace context (including onboarding instructions) sits right
		// next to the user message rather than being buried at the top.
		envelope = envelope + "\n\n" + agent.BuildSessionPreamble(r.workspaceDir)
	}
	r.incrementTurnCount()

	args := []string{"-p", envelope}
	if r.hasSavedSession(sessionDir) {
		args = append(args, "-c")
	}
	args = append(args, r.baseArgs(sessionDir)...)

	cmd := exec.CommandContext(ctx, "pi", args...)
	cmd.Dir = r.workspaceDir
	if reqID := msglog.RequestIDFromContext(ctx); reqID != "" {
		cmd.Env = append(filterEnv(os.Environ(), "GOAT_REQUEST_ID"), "GOAT_REQUEST_ID="+reqID)
	}

	runFile := filepath.Join(r.runsDir(), fmt.Sprintf("%d.jsonl", time.Now().UnixNano()))
	outFile, err := os.Create(runFile)
	if err != nil {
		return fmt.Errorf("create run log: %w", err)
	}

	var stdoutBuf strings.Builder
	cmd.Stdout = &teeWriter{file: outFile, buf: &stdoutBuf, redactor: r.redactor}

	var stderrBuf strings.Builder
	cmd.Stderr = &stderrWriter{file: outFile, buf: &stderrBuf, redactor: r.redactor}

	r.mu.Lock()
	for r.proc != nil {
		done := r.done
		r.mu.Unlock()

		select {
		case <-done:
		case <-time.After(postSendWaitTimeout):
			outFile.Close()
			return fmt.Errorf("timed out waiting for previous pi process to finish")
		case <-ctx.Done():
			outFile.Close()
			return ctx.Err()
		}

		r.mu.Lock()
	}

	if err := cmd.Start(); err != nil {
		r.mu.Unlock()
		outFile.Close()
		return fmt.Errorf("start pi: %w", err)
	}

	r.proc = cmd
	r.procErr = nil
	r.lastStderr = ""
	r.done = make(chan struct{})
	done := r.done
	r.mu.Unlock()

	go func() {
		runErr := cmd.Wait()
		outFile.Close()

		r.mu.Lock()
		r.proc = nil
		r.procErr = runErr
		r.lastStderr = stderrBuf.String()
		r.mu.Unlock()

		close(done)
	}()

	return nil
}

func (r *SessionRuntime) WaitForAwaitingInput(ctx context.Context, timeout time.Duration) (agent.SessionState, error) {
	r.mu.Lock()
	done := r.done
	proc := r.proc
	r.mu.Unlock()

	if proc == nil || done == nil {
		return agent.SessionState{Kind: agent.SessionStateAwaitingInput, Summary: "idle (no process)"}, nil
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-done:
		r.mu.Lock()
		lastErr := r.procErr
		r.mu.Unlock()
		if lastErr != nil {
			return agent.SessionState{Kind: agent.SessionStateAwaitingInput, Summary: fmt.Sprintf("process exited with error: %v", lastErr)}, nil
		}
		return agent.SessionState{Kind: agent.SessionStateAwaitingInput, Summary: "process completed"}, nil
	case <-timer.C:
		return agent.SessionState{Kind: agent.SessionStateGenerating, Summary: "timed out waiting for process"}, fmt.Errorf("timed out waiting for pi process to finish")
	case <-ctx.Done():
		return agent.SessionState{}, ctx.Err()
	}
}

func (r *SessionRuntime) GetSessionState(ctx context.Context) (agent.SessionState, error) {
	r.mu.Lock()
	proc := r.proc
	done := r.done
	lastErr := r.procErr
	lastStderr := r.lastStderr
	r.mu.Unlock()

	if proc != nil && done != nil {
		select {
		case <-done:
		default:
			return agent.SessionState{Kind: agent.SessionStateGenerating, Summary: "processing"}, nil
		}
	}

	for _, pat := range []string{"authentication", "login", "api key", "unauthorized"} {
		if strings.Contains(strings.ToLower(lastStderr), pat) {
			return agent.SessionState{Kind: agent.SessionStateBlockedIntervene, Summary: "Pi requires manual login or API key setup"}, nil
		}
	}

	if lastErr != nil {
		return agent.SessionState{Kind: agent.SessionStateAwaitingInput, Summary: fmt.Sprintf("last process error: %v", lastErr)}, nil
	}

	return agent.SessionState{Kind: agent.SessionStateAwaitingInput, Summary: "idle"}, nil
}

func (r *SessionRuntime) StopSession(ctx context.Context) error {
	r.mu.Lock()
	proc := r.proc
	done := r.done
	r.mu.Unlock()

	if proc != nil && proc.Process != nil {
		_ = proc.Process.Kill()
		if done != nil {
			select {
			case <-done:
			case <-time.After(5 * time.Second):
			}
		}
	}
	return nil
}

func (r *SessionRuntime) RestartSession(ctx context.Context) error {
	if err := r.StopSession(ctx); err != nil {
		return err
	}
	return r.EnsureSession(ctx)
}

func (r *SessionRuntime) ResetConversation(ctx context.Context, chatID string) (agent.ResetResult, error) {
	oldID, err := r.activeSessionID()
	if err != nil {
		return agent.ResetResult{}, err
	}
	if oldID != "" {
		if err := os.RemoveAll(r.sessionDirFor(oldID)); err != nil && !os.IsNotExist(err) {
			return agent.ResetResult{}, err
		}
	}
	newID, err := newPiSessionID()
	if err != nil {
		return agent.ResetResult{}, err
	}
	if err := r.setActiveSessionID(newID); err != nil {
		return agent.ResetResult{}, err
	}
	if err := os.MkdirAll(r.sessionDirFor(newID), 0o755); err != nil {
		return agent.ResetResult{}, err
	}
	return agent.ResetResult{
		Scope:   agent.ResetScopeHard,
		Summary: "Pi session cleared. The next message starts a fresh Goated-managed session.",
	}, nil
}

func (r *SessionRuntime) SendControlCommand(ctx context.Context, text string) error {
	return nil
}

func (r *SessionRuntime) GetContextEstimate(ctx context.Context, chatID string) (agent.ContextEstimate, error) {
	return agent.ContextEstimate{
		State:      agent.ContextEstimateUnknown,
		RawSummary: "pi manages context automatically",
	}, nil
}

func (r *SessionRuntime) GetHealth(ctx context.Context) (agent.HealthStatus, error) {
	if _, err := exec.LookPath("pi"); err != nil {
		return agent.HealthStatus{OK: false, Recoverable: false, Summary: "pi binary not found on PATH"}, nil
	}

	r.mu.Lock()
	lastStderr := r.lastStderr
	r.mu.Unlock()

	for _, pat := range []string{"authentication", "login", "api key", "unauthorized"} {
		if strings.Contains(strings.ToLower(lastStderr), pat) {
			return agent.HealthStatus{OK: false, Recoverable: false, Summary: fmt.Sprintf("auth error: %s", pat)}, nil
		}
	}

	// Real provider/model probe: ask pi for the model catalog and verify the
	// configured provider (and model, if pinned) is actually available. This
	// catches the "pi installed but no provider configured" case that used to
	// pass readiness and then fail on the first real message.
	if err := r.probeListModels(ctx); err != nil {
		return agent.HealthStatus{OK: false, Recoverable: false, Summary: err.Error()}, nil
	}

	return agent.HealthStatus{OK: true, Recoverable: true, Summary: "ok"}, nil
}

// probeListModels runs `pi --list-models` and verifies the configured provider
// (and model, if pinned) is present in the output. Returns nil on success, or
// an error describing the specific failure mode.
func (r *SessionRuntime) probeListModels(ctx context.Context) error {
	probeCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	cmd := exec.CommandContext(probeCtx, "pi", "--list-models")
	out, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(out))
		if trimmed == "" {
			trimmed = err.Error()
		}
		return fmt.Errorf("pi --list-models failed: %s", firstLine(trimmed))
	}

	text := string(out)
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("pi has no providers configured (see ./goated runtime pi configure)")
	}

	// If the user hasn't pinned a provider, we accept any non-empty catalog.
	if r.provider == "" {
		return nil
	}

	lowerText := strings.ToLower(text)
	if !strings.Contains(lowerText, strings.ToLower(r.provider)) {
		return fmt.Errorf("configured pi provider %q not found in pi --list-models output", r.provider)
	}
	if r.model != "" && !strings.Contains(lowerText, strings.ToLower(r.model)) {
		return fmt.Errorf("configured pi model %q not found in pi --list-models output", r.model)
	}
	return nil
}

func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return s[:idx]
	}
	return s
}

func (r *SessionRuntime) DetectRetryableError(ctx context.Context) string {
	r.mu.Lock()
	lastStderr := strings.ToLower(r.lastStderr)
	r.mu.Unlock()

	for _, pat := range []string{"server_error", "rate_limit", "overloaded", "status 500", "status 502", "status 503"} {
		if strings.Contains(lastStderr, pat) {
			return pat
		}
	}
	return ""
}

func (r *SessionRuntime) Version(ctx context.Context) string {
	cmd := exec.CommandContext(ctx, "pi", "--version")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
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

type teeWriter struct {
	file     *os.File
	buf      *strings.Builder
	redactor *msglog.Redactor
}

func (w *teeWriter) Write(p []byte) (int, error) {
	w.buf.Write(p)
	if w.redactor != nil {
		redacted := w.redactor.Redact(string(p))
		_, err := w.file.WriteString(redacted)
		if err != nil {
			return 0, err
		}
	} else {
		if _, err := w.file.Write(p); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

type stderrWriter struct {
	file     *os.File
	buf      *strings.Builder
	redactor *msglog.Redactor
}

func (w *stderrWriter) Write(p []byte) (int, error) {
	w.buf.Write(p)
	if w.redactor != nil {
		redacted := w.redactor.Redact(string(p))
		_, err := w.file.WriteString(redacted)
		if err != nil {
			return 0, err
		}
	} else {
		if _, err := w.file.Write(p); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}
