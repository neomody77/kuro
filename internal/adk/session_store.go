package adk

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"google.golang.org/adk/session"
	"google.golang.org/adk/tool/toolconfirmation"
	"google.golang.org/genai"
)

// FileSessionService wraps an InMemoryService with file-based persistence.
// Sessions are stored as JSON files under {dataDir}/users/{userID}/adk-sessions/{sessionID}/.
type FileSessionService struct {
	inner   session.Service
	dataDir string
	mu      sync.Mutex
}

// NewFileSessionService creates a persistent session service.
// It loads existing sessions from disk and replays their events into the in-memory service.
func NewFileSessionService(dataDir string) (*FileSessionService, error) {
	inner := session.InMemoryService()
	svc := &FileSessionService{
		inner:   inner,
		dataDir: dataDir,
	}
	if err := svc.loadAll(); err != nil {
		log.Printf("[adk/session_store] warning: partial load: %v", err)
	}
	return svc, nil
}

func (s *FileSessionService) Create(ctx context.Context, req *session.CreateRequest) (*session.CreateResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resp, err := s.inner.Create(ctx, req)
	if err != nil {
		return nil, err
	}

	// Persist session metadata
	if err := s.persistSession(resp.Session); err != nil {
		log.Printf("[adk/session_store] warning: failed to persist new session: %v", err)
	}
	return resp, nil
}

func (s *FileSessionService) Get(ctx context.Context, req *session.GetRequest) (*session.GetResponse, error) {
	return s.inner.Get(ctx, req)
}

func (s *FileSessionService) List(ctx context.Context, req *session.ListRequest) (*session.ListResponse, error) {
	return s.inner.List(ctx, req)
}

func (s *FileSessionService) Delete(ctx context.Context, req *session.DeleteRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.inner.Delete(ctx, req); err != nil {
		return err
	}

	// Remove session directory from disk
	dir := s.sessionDir(req.UserID, req.SessionID)
	if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
		log.Printf("[adk/session_store] warning: failed to remove session dir: %v", err)
	}
	return nil
}

func (s *FileSessionService) AppendEvent(ctx context.Context, sess session.Session, event *session.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.inner.AppendEvent(ctx, sess, event); err != nil {
		return err
	}

	// Only persist non-partial events (matching InMemoryService behavior)
	if event.Partial {
		return nil
	}

	if err := s.appendEventToDisk(sess.UserID(), sess.ID(), event); err != nil {
		log.Printf("[adk/session_store] warning: failed to persist event: %v", err)
	}
	return nil
}

// --- persistence helpers ---

type storedSession struct {
	ID        string         `json:"id"`
	AppName   string         `json:"app_name"`
	UserID    string         `json:"user_id"`
	State     map[string]any `json:"state,omitempty"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type storedEvent struct {
	ID               string              `json:"id"`
	InvocationID     string              `json:"invocation_id,omitempty"`
	Timestamp        time.Time           `json:"timestamp"`
	Author           string              `json:"author,omitempty"`
	Branch           string              `json:"branch,omitempty"`
	Content          *genai.Content      `json:"content,omitempty"`
	TurnComplete     bool                `json:"turn_complete,omitempty"`
	FinishReason     genai.FinishReason  `json:"finish_reason,omitempty"`
	Actions          storedEventActions  `json:"actions,omitempty"`
	LongRunningTools []string            `json:"long_running_tool_ids,omitempty"`
}

type storedEventActions struct {
	StateDelta                 map[string]any                                `json:"state_delta,omitempty"`
	ArtifactDelta              map[string]int64                              `json:"artifact_delta,omitempty"`
	RequestedToolConfirmations map[string]toolconfirmation.ToolConfirmation  `json:"requested_tool_confirmations,omitempty"`
	SkipSummarization          bool                                         `json:"skip_summarization,omitempty"`
	TransferToAgent            string                                       `json:"transfer_to_agent,omitempty"`
	Escalate                   bool                                         `json:"escalate,omitempty"`
}

func (s *FileSessionService) sessionDir(userID, sessionID string) string {
	return filepath.Join(s.dataDir, "users", userID, "adk-sessions", sessionID)
}

func (s *FileSessionService) persistSession(sess session.Session) error {
	dir := s.sessionDir(sess.UserID(), sess.ID())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	// Collect state
	state := map[string]any{}
	for k, v := range sess.State().All() {
		state[k] = v
	}

	data, err := json.Marshal(storedSession{
		ID:        sess.ID(),
		AppName:   sess.AppName(),
		UserID:    sess.UserID(),
		State:     state,
		UpdatedAt: sess.LastUpdateTime(),
	})
	if err != nil {
		return err
	}

	// Write atomically
	tmp := filepath.Join(dir, "session.json.tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, "session.json"))
}

func (s *FileSessionService) appendEventToDisk(userID, sessionID string, event *session.Event) error {
	dir := s.sessionDir(userID, sessionID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	se := storedEvent{
		ID:               event.ID,
		InvocationID:     event.InvocationID,
		Timestamp:        event.Timestamp,
		Author:           event.Author,
		Branch:           event.Branch,
		Content:          event.Content,
		TurnComplete:     event.TurnComplete,
		FinishReason:     event.FinishReason,
		LongRunningTools: event.LongRunningToolIDs,
		Actions: storedEventActions{
			StateDelta:                 event.Actions.StateDelta,
			ArtifactDelta:              event.Actions.ArtifactDelta,
			RequestedToolConfirmations: event.Actions.RequestedToolConfirmations,
			SkipSummarization:          event.Actions.SkipSummarization,
			TransferToAgent:            event.Actions.TransferToAgent,
			Escalate:                   event.Actions.Escalate,
		},
	}

	line, err := json.Marshal(se)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(filepath.Join(dir, "events.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s\n", line)
	return err
}

// loadAll scans disk for persisted sessions and replays them.
func (s *FileSessionService) loadAll() error {
	usersDir := filepath.Join(s.dataDir, "users")
	userEntries, err := os.ReadDir(usersDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	ctx := context.Background()
	loaded := 0

	for _, userEntry := range userEntries {
		if !userEntry.IsDir() {
			continue
		}
		userID := userEntry.Name()
		sessionsDir := filepath.Join(usersDir, userID, "adk-sessions")
		sessionEntries, err := os.ReadDir(sessionsDir)
		if err != nil {
			continue
		}

		for _, sessEntry := range sessionEntries {
			if !sessEntry.IsDir() {
				continue
			}
			sessionID := sessEntry.Name()
			if err := s.loadSession(ctx, userID, sessionID); err != nil {
				log.Printf("[adk/session_store] skip session %s/%s: %v", userID, sessionID, err)
				continue
			}
			loaded++
		}
	}

	if loaded > 0 {
		log.Printf("[adk/session_store] restored %d session(s)", loaded)
	}
	return nil
}

func (s *FileSessionService) loadSession(ctx context.Context, userID, sessionID string) error {
	dir := s.sessionDir(userID, sessionID)

	// Read session metadata
	metaData, err := os.ReadFile(filepath.Join(dir, "session.json"))
	if err != nil {
		return fmt.Errorf("read metadata: %w", err)
	}

	var meta storedSession
	if err := json.Unmarshal(metaData, &meta); err != nil {
		return fmt.Errorf("parse metadata: %w", err)
	}

	// Create session in inner service
	resp, err := s.inner.Create(ctx, &session.CreateRequest{
		AppName:   meta.AppName,
		UserID:    meta.UserID,
		SessionID: meta.ID,
		State:     meta.State,
	})
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	// Replay events
	eventsData, err := os.ReadFile(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No events yet
		}
		return fmt.Errorf("read events: %w", err)
	}

	for _, line := range strings.Split(string(eventsData), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var se storedEvent
		if err := json.Unmarshal([]byte(line), &se); err != nil {
			log.Printf("[adk/session_store] skip malformed event in %s/%s: %v", userID, sessionID, err)
			continue
		}

		event := &session.Event{
			ID:                 se.ID,
			InvocationID:       se.InvocationID,
			Timestamp:          se.Timestamp,
			Author:             se.Author,
			Branch:             se.Branch,
			LongRunningToolIDs: se.LongRunningTools,
			Actions: session.EventActions{
				StateDelta:                 se.Actions.StateDelta,
				ArtifactDelta:              se.Actions.ArtifactDelta,
				RequestedToolConfirmations: se.Actions.RequestedToolConfirmations,
				SkipSummarization:          se.Actions.SkipSummarization,
				TransferToAgent:            se.Actions.TransferToAgent,
				Escalate:                   se.Actions.Escalate,
			},
		}
		event.Content = se.Content
		event.TurnComplete = se.TurnComplete
		event.FinishReason = se.FinishReason

		if err := s.inner.AppendEvent(ctx, resp.Session, event); err != nil {
			log.Printf("[adk/session_store] skip event replay in %s/%s: %v", userID, sessionID, err)
		}
	}

	return nil
}
