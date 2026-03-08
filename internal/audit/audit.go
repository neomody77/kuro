// Package audit provides structured audit logging for all operations.
package audit

import (
	"context"
	"fmt"
	"log"
	"time"
)

// EventType categorizes audit log entries.
type EventType string

const (
	TypeUserAction   EventType = "user_action"
	TypeAIResponse   EventType = "ai_response"
	TypeSkillExec    EventType = "skill_exec"
	TypePipelineNode EventType = "pipeline_node"
	TypeSystem       EventType = "system"
)

// Entry is a single audit log record.
type Entry struct {
	ID         string         `json:"id"`
	TraceID    string         `json:"trace_id"`
	Timestamp  time.Time      `json:"timestamp"`
	UserID     string         `json:"user_id"`
	Type       EventType      `json:"type"`
	SessionID  string         `json:"session_id"`
	Action     string         `json:"action"`
	Detail     string         `json:"detail"`
	DurationMs int64          `json:"duration_ms"`
	Meta       map[string]any `json:"meta,omitempty"`
}

// QueryFilter specifies filters for querying audit logs.
type QueryFilter struct {
	Type      EventType `json:"type,omitempty"`
	TraceID   string    `json:"trace_id,omitempty"`
	SessionID string    `json:"session_id,omitempty"`
	UserID    string    `json:"user_id,omitempty"`
	Since     time.Time `json:"since,omitempty"`
	Until     time.Time `json:"until,omitempty"`
	Limit     int       `json:"limit,omitempty"`
	Offset    int       `json:"offset,omitempty"`
}

// Store is the interface for audit log persistence.
type Store interface {
	Log(entry Entry) error
	Query(filter QueryFilter) ([]Entry, int, error)
}

// Logger wraps a Store with convenience methods for emitting audit events.
type Logger struct {
	store Store
}

// NewLogger creates a new audit logger.
func NewLogger(store Store) *Logger {
	return &Logger{store: store}
}

func (l *Logger) log(entry Entry) {
	if l == nil || l.store == nil {
		return
	}
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("audit_%d", time.Now().UnixNano())
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	if err := l.store.Log(entry); err != nil {
		log.Printf("[audit] failed to log: %v", err)
	}
}

// LogUserAction records a user-initiated action.
func (l *Logger) LogUserAction(userID, sessionID, traceID, action, detail string) {
	l.log(Entry{
		TraceID:   traceID,
		UserID:    userID,
		Type:      TypeUserAction,
		SessionID: sessionID,
		Action:    action,
		Detail:    detail,
	})
}

// LogAIResponse records an AI response event.
func (l *Logger) LogAIResponse(userID, sessionID, traceID, action, detail string, durationMs int64) {
	l.log(Entry{
		TraceID:    traceID,
		UserID:     userID,
		Type:       TypeAIResponse,
		SessionID:  sessionID,
		Action:     action,
		Detail:     detail,
		DurationMs: durationMs,
	})
}

// LogSkillExec records a skill execution event.
func (l *Logger) LogSkillExec(userID, sessionID, traceID, skillName string, durationMs int64, err error) {
	detail := "ok"
	if err != nil {
		detail = err.Error()
	}
	l.log(Entry{
		TraceID:    traceID,
		UserID:     userID,
		Type:       TypeSkillExec,
		SessionID:  sessionID,
		Action:     skillName,
		Detail:     detail,
		DurationMs: durationMs,
	})
}

// LogPipelineNode records a pipeline node execution event.
func (l *Logger) LogPipelineNode(traceID, workflowID, nodeName string, durationMs int64, err error) {
	detail := "ok"
	if err != nil {
		detail = err.Error()
	}
	l.log(Entry{
		TraceID:    traceID,
		Type:       TypePipelineNode,
		Action:     nodeName,
		Detail:     detail,
		DurationMs: durationMs,
		Meta:       map[string]any{"workflow_id": workflowID},
	})
}

// LogSystem records a system event.
func (l *Logger) LogSystem(action, detail string) {
	l.log(Entry{
		Type:   TypeSystem,
		Action: action,
		Detail: detail,
	})
}

// --- Trace ID propagation via context ---

type traceKey struct{}

// WithTraceID adds a trace ID to the context.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceKey{}, traceID)
}

// TraceIDFrom extracts the trace ID from the context.
func TraceIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(traceKey{}).(string); ok {
		return v
	}
	return ""
}
