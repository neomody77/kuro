// Package chat implements the chat service with AI integration and skill invocation.
package chat

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/neomody77/kuro/internal/provider"
	"github.com/neomody77/kuro/internal/skill"
)

//go:embed system_prompt.txt
var systemPromptTemplate string

// DestructiveSkills lists skills that require user confirmation before execution.
// For unified skills with an "action" param, use "skill:action" format.
var DestructiveSkills = map[string]bool{
	"shell":             true,
	"credential:delete": true,
	"document:delete":   true,
	"file:write":        true,
}

// Message represents a single chat message.
type Message struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"` // "user" or "assistant"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// SkillCall represents an AI-initiated skill invocation.
type SkillCall struct {
	Skill   string         `json:"skill"`
	Inputs  map[string]any `json:"inputs"`
	Confirm bool           `json:"confirm"` // true if confirmation is needed before execution
}

// Response is the result of processing a chat message.
type Response struct {
	Message   Message    `json:"message"`
	SkillCall *SkillCall `json:"skillCall,omitempty"`
}

// PendingAction tracks a skill call awaiting user confirmation.
type PendingAction struct {
	ID        string         `json:"id"`
	Skill     string         `json:"skill"`
	Inputs    map[string]any `json:"inputs"`
	CreatedAt time.Time      `json:"created_at"`
}

// Session holds conversation state for a single conversation.
type Session struct {
	mu       sync.Mutex
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Messages []Message `json:"messages,omitempty"`
	Pending  *PendingAction
	Created  time.Time `json:"created"`
}

// SessionInfo is a summary of a session (no messages).
type SessionInfo struct {
	ID      string    `json:"id"`
	Title   string    `json:"title"`
	Created time.Time `json:"created"`
}

// Service is the chat service handling conversations and AI integration.
type Service struct {
	mu        sync.RWMutex
	sessions  map[string]*Session // key: "userID:sessionID"
	registry  *skill.Registry
	provider  provider.Provider
	model     string
	idCounter int
	dataDir   string // if non-empty, sessions are persisted to disk
}

// NewService creates a new chat service.
// If dataDir is non-empty, sessions are persisted as JSONL files under dataDir/users/{userID}/chat/.
func NewService(registry *skill.Registry, prov provider.Provider, model string, dataDir ...string) *Service {
	s := &Service{
		sessions: make(map[string]*Session),
		registry: registry,
		provider: prov,
		model:    model,
	}
	if len(dataDir) > 0 {
		s.dataDir = dataDir[0]
	}
	return s
}

// --- Persistence (JSONL) ---
//
// Layout:
//   {dataDir}/users/{userID}/chat/sessions.jsonl   — session index (one SessionInfo per line)
//   {dataDir}/users/{userID}/chat/{sessionID}.jsonl — append-only message log

func (s *Service) chatDir(userID string) string {
	return filepath.Join(s.dataDir, "users", userID, "chat")
}

func (s *Service) sessionsIndexPath(userID string) string {
	return filepath.Join(s.chatDir(userID), "sessions.jsonl")
}

func (s *Service) messagesPath(userID, sessionID string) string {
	return filepath.Join(s.chatDir(userID), sessionID+".jsonl")
}

// appendMessage appends a single message to the session's JSONL file.
func (s *Service) appendMessage(userID, sessionID string, msg Message) {
	if s.dataDir == "" {
		return
	}
	dir := s.chatDir(userID)
	os.MkdirAll(dir, 0o755)
	f, err := os.OpenFile(s.messagesPath(userID, sessionID), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	line, err := json.Marshal(msg)
	if err != nil {
		return
	}
	f.Write(line)
	f.Write([]byte("\n"))
}

// rewriteSessionIndex rewrites the sessions.jsonl index for a user.
func (s *Service) rewriteSessionIndex(userID string) {
	if s.dataDir == "" {
		return
	}
	dir := s.chatDir(userID)
	os.MkdirAll(dir, 0o755)

	prefix := userID + ":"
	var infos []SessionInfo
	for k, sess := range s.sessions {
		if strings.HasPrefix(k, prefix) {
			infos = append(infos, SessionInfo{ID: sess.ID, Title: sess.Title, Created: sess.Created})
		}
	}

	f, err := os.Create(s.sessionsIndexPath(userID))
	if err != nil {
		return
	}
	defer f.Close()
	for _, info := range infos {
		line, _ := json.Marshal(info)
		f.Write(line)
		f.Write([]byte("\n"))
	}
}

// updateSessionTitle updates the title in the sessions.jsonl index.
func (s *Service) updateSessionTitle(userID string) {
	if s.dataDir == "" {
		return
	}
	s.rewriteSessionIndex(userID)
}

// deleteSessionFiles removes the message file and updates the index.
func (s *Service) deleteSessionFiles(userID, sessionID string) {
	if s.dataDir == "" {
		return
	}
	os.Remove(s.messagesPath(userID, sessionID))
	s.rewriteSessionIndex(userID)
}

// loadSessionsFromDisk loads all sessions for a user from the JSONL index + message files.
func (s *Service) loadSessionsFromDisk(userID string) {
	if s.dataDir == "" {
		return
	}
	indexPath := s.sessionsIndexPath(userID)
	f, err := os.Open(indexPath)
	if err != nil {
		return
	}
	defer f.Close()

	prefix := userID + ":"
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var info SessionInfo
		if json.Unmarshal(scanner.Bytes(), &info) != nil || info.ID == "" {
			continue
		}
		key := prefix + info.ID
		if _, exists := s.sessions[key]; exists {
			continue
		}
		// Load messages from the message file.
		msgs := s.loadMessages(userID, info.ID)
		s.sessions[key] = &Session{
			ID:       info.ID,
			Title:    info.Title,
			Messages: msgs,
			Created:  info.Created,
		}
	}
}

// loadMessages reads all messages from a session's JSONL file.
func (s *Service) loadMessages(userID, sessionID string) []Message {
	f, err := os.Open(s.messagesPath(userID, sessionID))
	if err != nil {
		return nil
	}
	defer f.Close()

	var msgs []Message
	scanner := bufio.NewScanner(f)
	// Allow large messages (default 64KB may be too small for long conversations).
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var msg Message
		if json.Unmarshal(scanner.Bytes(), &msg) == nil {
			msgs = append(msgs, msg)
		}
	}
	return msgs
}

// --- End persistence ---

// SetProvider replaces the active AI provider and model at runtime.
func (s *Service) SetProvider(prov provider.Provider, model string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.provider = prov
	s.model = model
}

func (s *Service) nextID() string {
	s.idCounter++
	return fmt.Sprintf("msg_%d", s.idCounter)
}

func sessionKey(userID, sessionID string) string {
	return userID + ":" + sessionID
}

func (s *Service) getSession(userID, sessionID string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := sessionKey(userID, sessionID)
	sess, ok := s.sessions[key]
	if !ok {
		// Try loading from disk.
		if s.dataDir != "" {
			msgs := s.loadMessages(userID, sessionID)
			if msgs != nil {
				// Read title from index.
				title := s.loadSessionTitle(userID, sessionID)
				sess = &Session{ID: sessionID, Title: title, Messages: msgs, Created: time.Now()}
				s.sessions[key] = sess
				return sess
			}
		}
		sess = &Session{ID: sessionID, Title: "New Chat", Created: time.Now()}
		s.sessions[key] = sess
	}
	return sess
}

// loadSessionTitle reads the title for a session from the index file.
func (s *Service) loadSessionTitle(userID, sessionID string) string {
	f, err := os.Open(s.sessionsIndexPath(userID))
	if err != nil {
		return "New Chat"
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var info SessionInfo
		if json.Unmarshal(scanner.Bytes(), &info) == nil && info.ID == sessionID {
			return info.Title
		}
	}
	return "New Chat"
}

// CreateSession creates a new chat session and returns its info.
func (s *Service) CreateSession(userID string) *SessionInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := fmt.Sprintf("s_%d", time.Now().UnixMilli())
	sess := &Session{ID: id, Title: "New Chat", Created: time.Now()}
	s.sessions[sessionKey(userID, id)] = sess
	s.rewriteSessionIndex(userID)
	return &SessionInfo{ID: id, Title: sess.Title, Created: sess.Created}
}

// ListSessions returns all sessions for a user.
func (s *Service) ListSessions(userID string) []SessionInfo {
	s.mu.Lock()
	s.loadSessionsFromDisk(userID)
	s.mu.Unlock()

	s.mu.RLock()
	defer s.mu.RUnlock()
	prefix := userID + ":"
	var result []SessionInfo
	for k, sess := range s.sessions {
		if strings.HasPrefix(k, prefix) {
			result = append(result, SessionInfo{ID: sess.ID, Title: sess.Title, Created: sess.Created})
		}
	}
	return result
}

// DeleteSession removes a session.
func (s *Service) DeleteSession(userID, sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionKey(userID, sessionID))
	s.deleteSessionFiles(userID, sessionID)
}

// SendMessage processes a user message and returns an AI response.
func (s *Service) SendMessage(ctx context.Context, userID, sessionID, content string) (*Response, error) {
	if sessionID == "" {
		sessionID = "default"
	}
	sess := s.getSession(userID, sessionID)
	// Auto-set title from first user message.
	titleChanged := false
	if len(sess.Messages) == 0 {
		title := content
		if len(title) > 40 {
			title = title[:40] + "..."
		}
		sess.Title = title
		titleChanged = true
	}
	sess.mu.Lock()
	defer sess.mu.Unlock()

	// Record user message.
	userMsg := Message{
		ID:        s.nextID(),
		Role:      "user",
		Content:   content,
		Timestamp: time.Now(),
	}
	sess.Messages = append(sess.Messages, userMsg)
	s.appendMessage(userID, sessionID, userMsg)

	// Build system prompt with available skills.
	systemPrompt := s.buildSystemPrompt()

	// Build message history for AI.
	messages := []provider.Message{
		{Role: "system", Content: systemPrompt},
	}
	for _, m := range sess.Messages {
		messages = append(messages, provider.Message{Role: m.Role, Content: m.Content})
	}

	// Call AI provider.
	resp, err := s.provider.Complete(ctx, &provider.CompletionRequest{
		Model:    s.model,
		Messages: messages,
	})
	if err != nil {
		return nil, fmt.Errorf("chat: ai completion failed: %w", err)
	}
	if resp.Content == "" {
		log.Printf("[chat] WARNING: AI returned empty content (model=%s, msgs=%d)", s.model, len(messages))
	}

	// Parse AI response for skill invocations.
	assistantContent, skillCall := parseAIResponse(resp.Content)

	// Check if skill requires confirmation.
	// Check both direct skill name and "skill:action" format for unified skills.
	if skillCall != nil && isDestructive(skillCall) {
		skillCall.Confirm = true
		sess.Pending = &PendingAction{
			ID:        s.nextID(),
			Skill:     skillCall.Skill,
			Inputs:    skillCall.Inputs,
			CreatedAt: time.Now(),
		}
	} else if skillCall != nil {
		// Execute non-destructive skill immediately.
		result, err := s.registry.Execute(ctx, skillCall.Skill, skillCall.Inputs)
		if err != nil {
			assistantContent += fmt.Sprintf("\n\n[Skill error: %v]", err)
		} else {
			resultJSON, _ := json.Marshal(result)
			assistantContent += fmt.Sprintf("\n\n[Skill result: %s]", string(resultJSON))
		}
	}

	assistantMsg := Message{
		ID:        s.nextID(),
		Role:      "assistant",
		Content:   assistantContent,
		Timestamp: time.Now(),
	}
	sess.Messages = append(sess.Messages, assistantMsg)
	s.appendMessage(userID, sessionID, assistantMsg)

	// Update session index if title changed or this is a new session.
	if titleChanged {
		s.updateSessionTitle(userID)
	}

	return &Response{
		Message:   assistantMsg,
		SkillCall: skillCall,
	}, nil
}

// ConfirmAction executes or cancels a pending destructive action.
func (s *Service) ConfirmAction(ctx context.Context, userID, sessionID string, approve bool) (*Response, error) {
	if sessionID == "" {
		sessionID = "default"
	}
	sess := s.getSession(userID, sessionID)
	sess.mu.Lock()
	defer sess.mu.Unlock()

	if sess.Pending == nil {
		return nil, fmt.Errorf("chat: no pending action")
	}

	pending := sess.Pending
	sess.Pending = nil

	var content string
	if approve {
		result, err := s.registry.Execute(ctx, pending.Skill, pending.Inputs)
		if err != nil {
			content = fmt.Sprintf("Skill %q failed: %v", pending.Skill, err)
		} else {
			resultJSON, _ := json.Marshal(result)
			content = fmt.Sprintf("Executed %q: %s", pending.Skill, string(resultJSON))
		}
	} else {
		content = fmt.Sprintf("Cancelled %q.", pending.Skill)
	}

	msg := Message{
		ID:        s.nextID(),
		Role:      "assistant",
		Content:   content,
		Timestamp: time.Now(),
	}
	sess.Messages = append(sess.Messages, msg)
	s.appendMessage(userID, sessionID, msg)

	return &Response{Message: msg}, nil
}

// GetHistory returns the conversation history for a session.
func (s *Service) GetHistory(userID, sessionID string) []Message {
	if sessionID == "" {
		sessionID = "default"
	}
	sess := s.getSession(userID, sessionID)
	sess.mu.Lock()
	defer sess.mu.Unlock()

	if sess.Messages == nil {
		return []Message{}
	}
	out := make([]Message, len(sess.Messages))
	copy(out, sess.Messages)
	return out
}

// buildSystemPrompt constructs the AI system prompt including available skills.
func (s *Service) buildSystemPrompt() string {
	var sb strings.Builder
	sb.WriteString("Available skills:\n")
	if s.registry != nil {
		for _, sk := range s.registry.List() {
			sb.WriteString(fmt.Sprintf("- %s: %s", sk.Name, sk.Description))
			if len(sk.Inputs) > 0 {
				inputNames := make([]string, len(sk.Inputs))
				for i, inp := range sk.Inputs {
					inputNames[i] = inp.Name
					if inp.Required {
						inputNames[i] += "*"
					}
				}
				sb.WriteString(fmt.Sprintf(" (inputs: %s)", strings.Join(inputNames, ", ")))
			}
			sb.WriteString("\n")
		}
	}
	return strings.Replace(systemPromptTemplate, "{{SKILLS}}", sb.String(), 1)
}

// isDestructive checks if a skill call requires user confirmation.
func isDestructive(sc *SkillCall) bool {
	// Direct match (e.g., "shell")
	if DestructiveSkills[sc.Skill] {
		return true
	}
	// Check "skill:action" format for unified skills
	if action, ok := sc.Inputs["action"].(string); ok {
		return DestructiveSkills[sc.Skill+":"+action]
	}
	return false
}

// parseAIResponse extracts the text content and any embedded skill call from the AI response.
func parseAIResponse(raw string) (string, *SkillCall) {
	// Look for ```skill ... ``` block in the response.
	const startTag = "```skill\n"
	const endTag = "\n```"

	idx := strings.Index(raw, startTag)
	if idx < 0 {
		// Also try without newline after opening.
		const altStart = "```skill "
		idx = strings.Index(raw, altStart)
		if idx >= 0 {
			// Find end of the line and parse JSON from there.
			after := raw[idx+len(altStart):]
			endIdx := strings.Index(after, endTag)
			if endIdx < 0 {
				endIdx = len(after)
			}
			jsonStr := strings.TrimSpace(after[:endIdx])
			sc := parseSkillJSON(jsonStr)
			text := strings.TrimSpace(raw[:idx] + raw[idx+len(altStart)+endIdx:])
			if len(text) > len(endTag) {
				text = strings.TrimSuffix(text, "```")
			}
			return strings.TrimSpace(text), sc
		}
		return raw, nil
	}

	after := raw[idx+len(startTag):]
	endIdx := strings.Index(after, endTag)
	if endIdx < 0 {
		endIdx = len(after)
	}
	jsonStr := strings.TrimSpace(after[:endIdx])
	sc := parseSkillJSON(jsonStr)

	// Remove the skill block from the text.
	before := raw[:idx]
	remaining := ""
	if endIdx+len(endTag) < len(after) {
		remaining = after[endIdx+len(endTag):]
	}
	text := strings.TrimSpace(before + remaining)

	return text, sc
}

func parseSkillJSON(s string) *SkillCall {
	var sc SkillCall
	if err := json.Unmarshal([]byte(s), &sc); err != nil {
		return nil
	}
	if sc.Skill == "" {
		return nil
	}
	return &sc
}
