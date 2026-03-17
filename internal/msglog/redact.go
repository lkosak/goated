package msglog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const redactedPlaceholder = "[REDACTED]"
const minSecretLen = 4

// Redactor replaces credential values in serialized JSON strings.
// It auto-reloads secrets when the creds directory is modified.
type Redactor struct {
	credsDir    string
	lastModTime time.Time
	mu          sync.Mutex

	// secrets holds pairs of (raw, json-escaped) forms, sorted longest-first.
	secrets []secretPair
}

type secretPair struct {
	raw     string
	escaped string // JSON-escaped form (may differ from raw if it contains quotes, backslashes, etc)
}

// NewRedactor reads all *.txt files from credsDir, builds a redactor that
// replaces any matching credential values with [REDACTED].
func NewRedactor(credsDir string) *Redactor {
	r := &Redactor{credsDir: credsDir}
	r.loadSecrets()
	return r
}

// loadSecrets reads all *.txt files from the creds directory and rebuilds
// the secrets list. Must be called with r.mu held or during init.
func (r *Redactor) loadSecrets() {
	r.secrets = nil

	info, err := os.Stat(r.credsDir)
	if err != nil {
		return // no creds dir — nothing to redact
	}
	r.lastModTime = info.ModTime()

	entries, err := os.ReadDir(r.credsDir)
	if err != nil {
		return
	}

	var rawSecrets []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(r.credsDir, e.Name()))
		if err != nil {
			continue
		}
		val := strings.TrimSpace(string(data))
		if len(val) < minSecretLen {
			continue
		}
		rawSecrets = append(rawSecrets, val)
	}

	// Sort longest-first so longer secrets are replaced before substrings
	sort.Slice(rawSecrets, func(i, j int) bool {
		return len(rawSecrets[i]) > len(rawSecrets[j])
	})

	for _, raw := range rawSecrets {
		escaped := jsonEscape(raw)
		r.secrets = append(r.secrets, secretPair{raw: raw, escaped: escaped})
	}
}

// maybeReload checks if the creds directory has been modified and reloads if so.
func (r *Redactor) maybeReload() {
	if r.credsDir == "" {
		return
	}
	info, err := os.Stat(r.credsDir)
	if err != nil {
		return
	}
	if !info.ModTime().Equal(r.lastModTime) {
		r.loadSecrets()
	}
}

// ForceReload reloads secrets from the creds directory unconditionally.
func (r *Redactor) ForceReload() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.loadSecrets()
}

// AddSecrets injects additional secret values into the redactor. Useful for
// catching credential values that appear in content before they've been
// written to the creds directory (e.g. in PreToolUse or UserPromptSubmit
// hook events containing "goat creds set <key> <value>").
func (r *Redactor) AddSecrets(values []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, raw := range values {
		if len(raw) < minSecretLen {
			continue
		}
		escaped := jsonEscape(raw)
		r.secrets = append(r.secrets, secretPair{raw: raw, escaped: escaped})
	}
	sort.Slice(r.secrets, func(i, j int) bool {
		return len(r.secrets[i].raw) > len(r.secrets[j].raw)
	})
}

// HasSecrets returns true if the redactor has any loaded secrets.
func (r *Redactor) HasSecrets() bool {
	r.mu.Lock()
	r.maybeReload()
	n := len(r.secrets)
	r.mu.Unlock()
	return n > 0
}

// Redact replaces all credential values in the given string with [REDACTED].
func (r *Redactor) Redact(s string) string {
	r.mu.Lock()
	r.maybeReload()
	secrets := r.secrets
	r.mu.Unlock()

	for _, sp := range secrets {
		s = strings.ReplaceAll(s, sp.raw, redactedPlaceholder)
		if sp.escaped != sp.raw {
			s = strings.ReplaceAll(s, sp.escaped, redactedPlaceholder)
		}
	}
	return s
}

// RedactMapContainers selectively redacts content containers within a JSON map.
// For each key in containerKeys, if it exists in m:
//   - string values are redacted directly
//   - map/slice values are recursively walked, redacting ALL strings within
//
// Metadata keys not listed in containerKeys are left untouched.
func (r *Redactor) RedactMapContainers(m map[string]any, containerKeys []string) {
	r.mu.Lock()
	r.maybeReload()
	secrets := r.secrets
	r.mu.Unlock()

	for _, key := range containerKeys {
		val, ok := m[key]
		if !ok {
			continue
		}
		m[key] = redactValue(val, secrets)
	}
}

// redactValue recursively redacts all string values in a JSON-like structure.
func redactValue(v any, secrets []secretPair) any {
	switch val := v.(type) {
	case string:
		return redactString(val, secrets)
	case map[string]any:
		for k, inner := range val {
			val[k] = redactValue(inner, secrets)
		}
		return val
	case []any:
		for i, inner := range val {
			val[i] = redactValue(inner, secrets)
		}
		return val
	default:
		return v
	}
}

// redactString replaces credential values in a single string.
func redactString(s string, secrets []secretPair) string {
	for _, sp := range secrets {
		s = strings.ReplaceAll(s, sp.raw, redactedPlaceholder)
		if sp.escaped != sp.raw {
			s = strings.ReplaceAll(s, sp.escaped, redactedPlaceholder)
		}
	}
	return s
}

// jsonEscape returns the JSON-encoded form of a string, without surrounding quotes.
func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	// Strip surrounding quotes
	if len(b) >= 2 {
		return string(b[1 : len(b)-1])
	}
	return s
}
