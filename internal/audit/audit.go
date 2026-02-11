package audit

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// LogEntry defines the structured audit log
type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	TenantID  string                 `json:"tenant_id,omitempty"` // For Multi-tenancy
	ActorID   string                 `json:"actor_id"`
	Action    string                 `json:"action"`   // method + path
	Resource  string                 `json:"resource"` // Path
	Status    int                    `json:"status"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Logger interface
type Logger interface {
	Log(entry LogEntry)
}

// JSONLogger writes to io.Writer
type JSONLogger struct {
	out io.Writer
}

func NewJSONLogger(w io.Writer) *JSONLogger {
	return &JSONLogger{out: w}
}

func (l *JSONLogger) Log(entry LogEntry) {
	// Mask sensitive metadata if any
	if entry.Metadata != nil {
		maskSensitive(entry.Metadata)
	}

	bytes, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Audit log error: %v\n", err)
		return
	}
	l.out.Write(bytes)
	l.out.Write([]byte("\n"))
}

func maskSensitive(m map[string]interface{}) {
	sensitiveKeys := []string{"api_key", "password", "token", "secret"}
	for k := range m {
		lowerK := strings.ToLower(k)
		for _, s := range sensitiveKeys {
			if strings.Contains(lowerK, s) {
				m[k] = "***REDACTED***"
				break
			}
		}
	}
}
