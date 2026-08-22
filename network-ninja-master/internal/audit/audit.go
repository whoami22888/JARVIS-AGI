// Package audit records append-only operation events and JSON evidence.
package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var secretKey = regexp.MustCompile(`(?i)(password|passwd|secret|token|api[_-]?key|authorization|cookie)`)

// Event is a non-secret audit record. It should be sufficient to explain a
// decision without recording credentials or unnecessary payload content.
type Event struct {
	Timestamp    time.Time `json:"timestamp"`
	EventID      string    `json:"event_id"`
	EngagementID string    `json:"engagement_id,omitempty"`
	Operator     string    `json:"operator,omitempty"`
	Operation    string    `json:"operation"`
	Target       string    `json:"target,omitempty"`
	Status       string    `json:"status"`
	Decision     string    `json:"decision"`
	EvidenceID   string    `json:"evidence_id,omitempty"`
	EvidenceSHA  string    `json:"evidence_sha256,omitempty"`
	Detail       any       `json:"detail,omitempty"`
}

// Store writes immutable-looking evidence files and append-only JSONL records.
type Store struct {
	root     string
	auditLog string
	mu       sync.Mutex
}

func Open(root string) (*Store, error) {
	for _, dir := range []string{filepath.Join(root, "audit"), filepath.Join(root, "evidence")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("create audit directory: %w", err)
		}
	}
	return &Store{root: root, auditLog: filepath.Join(root, "audit", "audit.jsonl")}, nil
}

func (s *Store) Record(event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	event.Timestamp = event.Timestamp.UTC()
	event.Detail = redact(event.Detail)
	line, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal audit event: %w", err)
	}
	f, err := os.OpenFile(s.auditLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open audit log: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("append audit event: %w", err)
	}
	return f.Sync()
}

// Evidence writes canonical JSON. The returned digest is calculated over the
// exact stored bytes and can be independently checked with sha256sum.
func (s *Store) Evidence(prefix string, value any) (id, digest string, err error) {
	canonical, err := json.MarshalIndent(redact(value), "", "  ")
	if err != nil {
		return "", "", fmt.Errorf("marshal evidence: %w", err)
	}
	canonical = append(canonical, '\n')
	sum := sha256.Sum256(canonical)
	digest = hex.EncodeToString(sum[:])
	stamp := time.Now().UTC().Format("20060102T150405.000000000Z")
	cleanPrefix := sanitizePrefix(prefix)
	id = fmt.Sprintf("%s-%s-%s", cleanPrefix, stamp, digest[:12])
	path := filepath.Join(s.root, "evidence", id+".json")
	if err := os.WriteFile(path, canonical, 0o600); err != nil {
		return "", "", fmt.Errorf("write evidence: %w", err)
	}
	return id, digest, nil
}

func sanitizePrefix(value string) string {
	value = strings.ToLower(value)
	value = regexp.MustCompile(`[^a-z0-9_-]+`).ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "evidence"
	}
	return value
}

func redact(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			if secretKey.MatchString(key) {
				result[key] = "[REDACTED]"
			} else {
				result[key] = redact(item)
			}
		}
		return result
	case map[string]string:
		result := make(map[string]string, len(typed))
		for key, item := range typed {
			if secretKey.MatchString(key) {
				result[key] = "[REDACTED]"
			} else {
				result[key] = item
			}
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, item := range typed {
			result[index] = redact(item)
		}
		return result
	default:
		return value
	}
}
