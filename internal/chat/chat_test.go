package chat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neomody77/kuro/internal/pipeline"
	"github.com/neomody77/kuro/internal/provider"
	"github.com/neomody77/kuro/internal/skill"
)

// mockProvider returns a configurable response.
type mockProvider struct {
	response  string
	lastMsgs  []provider.Message
	callCount int
}

func (p *mockProvider) Complete(_ context.Context, req *provider.CompletionRequest) (*provider.CompletionResponse, error) {
	p.lastMsgs = req.Messages
	p.callCount++
	return &provider.CompletionResponse{
		Content: p.response,
		Model:   "mock",
	}, nil
}

// echoHandler is a simple skill handler that echoes its inputs.
type echoHandler struct{}

func (h *echoHandler) Execute(_ context.Context, params map[string]any, _ map[string]string) (any, error) {
	return map[string]any{"echo": params}, nil
}

// failHandler always returns an error.
type failHandler struct{}

func (h *failHandler) Execute(_ context.Context, params map[string]any, _ map[string]string) (any, error) {
	return nil, fmt.Errorf("simulated failure")
}

var _ pipeline.ActionHandler = (*echoHandler)(nil)
var _ pipeline.ActionHandler = (*failHandler)(nil)

func setupTestService(response string) (*Service, *mockProvider, *skill.Registry) {
	prov := &mockProvider{response: response}
	reg := skill.NewRegistry(nil)
	reg.Register(&skill.Skill{
		Name:        "document",
		Description: "Manage documents. Actions: list, get, save, delete, search",
		Inputs:      []skill.SkillParam{{Name: "action", Required: true}},
		Handler:     &echoHandler{},
	})
	reg.Register(&skill.Skill{
		Name:        "shell",
		Description: "Run shell command",
		Inputs:      []skill.SkillParam{{Name: "command", Required: true}},
		Handler:     &echoHandler{},
	})
	reg.Register(&skill.Skill{
		Name:        "http",
		Description: "HTTP request",
		Inputs:      []skill.SkillParam{{Name: "url", Required: true}},
		Handler:     &echoHandler{},
	})
	svc := NewService(reg, prov, "mock-model")
	return svc, prov, reg
}

func TestSendMessageReturnsResponse(t *testing.T) {
	svc, _, _ := setupTestService("Hello! I can help you with that.")

	resp, err := svc.SendMessage(context.Background(), "user1", "", "Hi there")
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if resp.Message.Role != "assistant" {
		t.Errorf("role = %q, want assistant", resp.Message.Role)
	}
	if resp.Message.Content != "Hello! I can help you with that." {
		t.Errorf("content = %q, want %q", resp.Message.Content, "Hello! I can help you with that.")
	}
	if resp.Message.ID == "" {
		t.Error("expected non-empty message ID")
	}
	if resp.Message.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestChatHistoryReturnsPastMessages(t *testing.T) {
	svc, _, _ := setupTestService("Response 1")

	ctx := context.Background()
	_, err := svc.SendMessage(ctx, "user1", "", "First message")
	if err != nil {
		t.Fatal(err)
	}

	history := svc.GetHistory("user1", "")
	if len(history) != 2 {
		t.Fatalf("expected 2 messages (user+assistant), got %d", len(history))
	}
	if history[0].Role != "user" {
		t.Errorf("first message role = %q, want user", history[0].Role)
	}
	if history[0].Content != "First message" {
		t.Errorf("first message content = %q, want %q", history[0].Content, "First message")
	}
	if history[1].Role != "assistant" {
		t.Errorf("second message role = %q, want assistant", history[1].Role)
	}
}

func TestChatHistoryEmptyForNewUser(t *testing.T) {
	svc, _, _ := setupTestService("ok")
	history := svc.GetHistory("nobody", "")
	if history == nil {
		t.Fatal("expected non-nil slice")
	}
	if len(history) != 0 {
		t.Errorf("expected empty history, got %d messages", len(history))
	}
}

func TestSkillDiscovery(t *testing.T) {
	svc, prov, _ := setupTestService("I see skills available!")

	_, err := svc.SendMessage(context.Background(), "user1", "", "What can you do?")
	if err != nil {
		t.Fatal(err)
	}

	// The system prompt sent to the provider should include skill names.
	if len(prov.lastMsgs) == 0 {
		t.Fatal("no messages sent to provider")
	}
	systemMsg := prov.lastMsgs[0]
	if systemMsg.Role != "system" {
		t.Errorf("first message role = %q, want system", systemMsg.Role)
	}
	if !strings.Contains(systemMsg.Content, "document") {
		t.Error("system prompt missing document skill")
	}
	if !strings.Contains(systemMsg.Content, "shell") {
		t.Error("system prompt missing shell skill")
	}
	if !strings.Contains(systemMsg.Content, "http") {
		t.Error("system prompt missing http skill")
	}
}

func TestSessionContextPreserved(t *testing.T) {
	prov := &mockProvider{response: "Got it."}
	reg := skill.NewRegistry(nil)
	svc := NewService(reg, prov, "mock")

	ctx := context.Background()

	// Send two messages and verify the second call includes history.
	svc.SendMessage(ctx, "user1", "", "My name is Alice")
	prov.response = "Hello Alice!"
	svc.SendMessage(ctx, "user1", "", "What is my name?")

	// The second AI call should have: system + user1 + assistant1 + user2 = 4 messages
	if len(prov.lastMsgs) != 4 {
		t.Fatalf("expected 4 messages in second call (system + user1 + assistant1 + user2), got %d", len(prov.lastMsgs))
	}
	// Check that user1 message is in history.
	found := false
	for _, m := range prov.lastMsgs {
		if m.Content == "My name is Alice" {
			found = true
		}
	}
	if !found {
		t.Error("previous user message not included in AI context")
	}
}

func TestSessionIsolation(t *testing.T) {
	svc, _, _ := setupTestService("Reply")

	ctx := context.Background()
	svc.SendMessage(ctx, "alice", "", "Hello from Alice")
	svc.SendMessage(ctx, "bob", "", "Hello from Bob")

	aliceHistory := svc.GetHistory("alice", "")
	bobHistory := svc.GetHistory("bob", "")

	if len(aliceHistory) != 2 {
		t.Errorf("alice history = %d, want 2", len(aliceHistory))
	}
	if len(bobHistory) != 2 {
		t.Errorf("bob history = %d, want 2", len(bobHistory))
	}
	if aliceHistory[0].Content != "Hello from Alice" {
		t.Errorf("alice message content = %q", aliceHistory[0].Content)
	}
	if bobHistory[0].Content != "Hello from Bob" {
		t.Errorf("bob message content = %q", bobHistory[0].Content)
	}
}

func TestNonDestructiveSkillExecutedImmediately(t *testing.T) {
	// AI returns a response with a skill call to document (non-destructive).
	response := "Let me list your documents.\n```skill\n{\"skill\": \"document\", \"inputs\": {\"action\": \"list\"}}\n```"
	svc, _, _ := setupTestService(response)

	resp, err := svc.SendMessage(context.Background(), "user1", "", "Show my docs")
	if err != nil {
		t.Fatal(err)
	}

	// Skill should have been executed (no confirmation needed).
	if resp.SkillCall != nil && resp.SkillCall.Confirm {
		t.Error("document.list should not require confirmation")
	}
	// Result should be embedded in the content.
	if !strings.Contains(resp.Message.Content, "Skill result:") {
		t.Errorf("expected skill result in content, got %q", resp.Message.Content)
	}
}

func TestDestructiveSkillRequiresConfirmation(t *testing.T) {
	// AI returns a skill call to shell (destructive).
	response := "I'll run that command.\n```skill\n{\"skill\": \"shell\", \"inputs\": {\"command\": \"rm -rf /\"}}\n```"
	svc, _, _ := setupTestService(response)

	resp, err := svc.SendMessage(context.Background(), "user1", "", "Delete everything")
	if err != nil {
		t.Fatal(err)
	}

	if resp.SkillCall == nil {
		t.Fatal("expected a skill call in response")
	}
	if !resp.SkillCall.Confirm {
		t.Error("shell should require confirmation")
	}
	if resp.SkillCall.Skill != "shell" {
		t.Errorf("skill = %q, want shell", resp.SkillCall.Skill)
	}
	// The skill should NOT have been executed yet.
	if strings.Contains(resp.Message.Content, "Skill result:") {
		t.Error("destructive skill should not be executed without confirmation")
	}
}

func TestConfirmActionApprove(t *testing.T) {
	response := "Running command.\n```skill\n{\"skill\": \"shell\", \"inputs\": {\"command\": \"echo hi\"}}\n```"
	svc, _, _ := setupTestService(response)

	ctx := context.Background()
	_, err := svc.SendMessage(ctx, "user1", "", "Run echo hi")
	if err != nil {
		t.Fatal(err)
	}

	// Approve the pending action.
	resp, err := svc.ConfirmAction(ctx, "user1", "", true)
	if err != nil {
		t.Fatalf("ConfirmAction: %v", err)
	}
	if !strings.Contains(resp.Message.Content, "Executed") {
		t.Errorf("expected Executed in content, got %q", resp.Message.Content)
	}
}

func TestConfirmActionDeny(t *testing.T) {
	response := "Running command.\n```skill\n{\"skill\": \"shell\", \"inputs\": {\"command\": \"echo hi\"}}\n```"
	svc, _, _ := setupTestService(response)

	ctx := context.Background()
	svc.SendMessage(ctx, "user1", "", "Run echo")

	resp, err := svc.ConfirmAction(ctx, "user1", "", false)
	if err != nil {
		t.Fatalf("ConfirmAction: %v", err)
	}
	if !strings.Contains(resp.Message.Content, "Cancelled") {
		t.Errorf("expected Cancelled in content, got %q", resp.Message.Content)
	}
}

func TestConfirmActionNoPending(t *testing.T) {
	svc, _, _ := setupTestService("ok")

	_, err := svc.ConfirmAction(context.Background(), "user1", "", true)
	if err == nil {
		t.Fatal("expected error when no pending action")
	}
}

func TestParseAIResponseNoSkill(t *testing.T) {
	text, sc := parseAIResponse("Just a plain text response.")
	if text != "Just a plain text response." {
		t.Errorf("text = %q", text)
	}
	if sc != nil {
		t.Error("expected no skill call")
	}
}

func TestParseAIResponseWithSkill(t *testing.T) {
	raw := "Here are your docs:\n```skill\n{\"skill\": \"document\", \"inputs\": {\"action\": \"list\"}}\n```\nDone."
	text, sc := parseAIResponse(raw)
	if sc == nil {
		t.Fatal("expected skill call")
	}
	if sc.Skill != "document" {
		t.Errorf("skill = %q", sc.Skill)
	}
	if !strings.Contains(text, "Here are your docs:") {
		t.Errorf("text should contain prefix, got %q", text)
	}
}

func TestBuildSystemPromptIncludesSkills(t *testing.T) {
	reg := skill.NewRegistry(nil)
	reg.Register(&skill.Skill{
		Name:        "test.skill",
		Description: "A test skill",
		Inputs:      []skill.SkillParam{{Name: "x", Required: true}},
	})
	prov := &mockProvider{response: "ok"}
	svc := NewService(reg, prov, "mock")

	prompt := svc.buildSystemPrompt()
	if !strings.Contains(prompt, "test.skill") {
		t.Error("system prompt should include test.skill")
	}
	if !strings.Contains(prompt, "A test skill") {
		t.Error("system prompt should include skill description")
	}
	if !strings.Contains(prompt, "x*") {
		t.Error("system prompt should mark required inputs with *")
	}
}

func TestMultipleTurnConversation(t *testing.T) {
	prov := &mockProvider{}
	reg := skill.NewRegistry(nil)
	svc := NewService(reg, prov, "mock")

	ctx := context.Background()

	// Turn 1
	prov.response = "I'm ready to help!"
	svc.SendMessage(ctx, "user1", "", "Hello")

	// Turn 2
	prov.response = "Sure, what do you need?"
	svc.SendMessage(ctx, "user1", "", "I need help")

	// Turn 3
	prov.response = "The project is at /home/user/project"
	svc.SendMessage(ctx, "user1", "", "Where is the project?")

	history := svc.GetHistory("user1", "")
	if len(history) != 6 {
		t.Fatalf("expected 6 messages (3 user + 3 assistant), got %d", len(history))
	}

	// Verify alternating roles.
	for i, m := range history {
		expectedRole := "user"
		if i%2 == 1 {
			expectedRole = "assistant"
		}
		if m.Role != expectedRole {
			t.Errorf("message %d: role = %q, want %q", i, m.Role, expectedRole)
		}
	}
}

func TestSkillCallWithInvalidJSON(t *testing.T) {
	// AI returns malformed skill JSON — should be treated as plain text.
	response := "Here:\n```skill\n{invalid json}\n```"
	svc, _, _ := setupTestService(response)

	resp, err := svc.SendMessage(context.Background(), "user1", "", "Do something")
	if err != nil {
		t.Fatal(err)
	}
	if resp.SkillCall != nil {
		t.Error("expected no skill call for invalid JSON")
	}
}

// --- Persistence tests ---

func setupPersistentService(t *testing.T, response string) (*Service, string) {
	t.Helper()
	dataDir := t.TempDir()
	prov := &mockProvider{response: response}
	reg := skill.NewRegistry(nil)
	svc := NewService(reg, prov, "mock", dataDir)
	return svc, dataDir
}

func TestPersistenceSessionSurvivesRestart(t *testing.T) {
	dataDir := t.TempDir()
	prov := &mockProvider{response: "Hello back!"}
	reg := skill.NewRegistry(nil)

	// Create service, send a message.
	svc1 := NewService(reg, prov, "mock", dataDir)
	ctx := context.Background()
	_, err := svc1.SendMessage(ctx, "alice", "s1", "Hi there")
	if err != nil {
		t.Fatal(err)
	}

	// Simulate restart: create a new service with same dataDir.
	svc2 := NewService(reg, prov, "mock", dataDir)
	history := svc2.GetHistory("alice", "s1")
	if len(history) != 2 {
		t.Fatalf("expected 2 messages after restart, got %d", len(history))
	}
	if history[0].Content != "Hi there" {
		t.Errorf("first message = %q, want %q", history[0].Content, "Hi there")
	}
	if history[1].Content != "Hello back!" {
		t.Errorf("second message = %q, want %q", history[1].Content, "Hello back!")
	}
}

func TestPersistenceListSessionsAfterRestart(t *testing.T) {
	dataDir := t.TempDir()
	prov := &mockProvider{response: "ok"}
	reg := skill.NewRegistry(nil)

	svc1 := NewService(reg, prov, "mock", dataDir)
	ctx := context.Background()
	svc1.SendMessage(ctx, "bob", "chat-1", "Hello")
	svc1.SendMessage(ctx, "bob", "chat-2", "World")

	// Restart.
	svc2 := NewService(reg, prov, "mock", dataDir)
	sessions := svc2.ListSessions("bob")
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions after restart, got %d", len(sessions))
	}
	ids := map[string]bool{}
	for _, s := range sessions {
		ids[s.ID] = true
	}
	if !ids["chat-1"] || !ids["chat-2"] {
		t.Errorf("missing sessions: got %v", ids)
	}
}

func TestPersistenceDeleteSessionRemovesFile(t *testing.T) {
	dataDir := t.TempDir()
	prov := &mockProvider{response: "ok"}
	reg := skill.NewRegistry(nil)

	svc := NewService(reg, prov, "mock", dataDir)
	ctx := context.Background()
	svc.SendMessage(ctx, "carol", "s1", "Hello")

	// Verify JSONL message file exists.
	fp := filepath.Join(dataDir, "users", "carol", "chat", "s1.jsonl")
	if _, err := os.Stat(fp); err != nil {
		t.Fatalf("session message file should exist: %v", err)
	}

	svc.DeleteSession("carol", "s1")

	// Message file should be gone.
	if _, err := os.Stat(fp); !os.IsNotExist(err) {
		t.Error("session message file should be deleted after DeleteSession")
	}

	// Restart — session should not reappear.
	svc2 := NewService(reg, prov, "mock", dataDir)
	sessions := svc2.ListSessions("carol")
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions after delete+restart, got %d", len(sessions))
	}
}

func TestPersistenceTitleAutoSetFromFirstMessage(t *testing.T) {
	dataDir := t.TempDir()
	prov := &mockProvider{response: "ok"}
	reg := skill.NewRegistry(nil)

	svc1 := NewService(reg, prov, "mock", dataDir)
	svc1.SendMessage(context.Background(), "dave", "s1", "Check the weather in Tokyo")

	// Restart and verify title was persisted.
	svc2 := NewService(reg, prov, "mock", dataDir)
	sessions := svc2.ListSessions("dave")
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Title != "Check the weather in Tokyo" {
		t.Errorf("title = %q, want %q", sessions[0].Title, "Check the weather in Tokyo")
	}
}

func TestPersistenceUserIsolation(t *testing.T) {
	dataDir := t.TempDir()
	prov := &mockProvider{response: "ok"}
	reg := skill.NewRegistry(nil)

	svc := NewService(reg, prov, "mock", dataDir)
	ctx := context.Background()
	svc.SendMessage(ctx, "eve", "s1", "Eve's message")
	svc.SendMessage(ctx, "frank", "s1", "Frank's message")

	// Restart.
	svc2 := NewService(reg, prov, "mock", dataDir)
	eveHistory := svc2.GetHistory("eve", "s1")
	frankHistory := svc2.GetHistory("frank", "s1")

	if len(eveHistory) != 2 {
		t.Fatalf("eve history = %d, want 2", len(eveHistory))
	}
	if len(frankHistory) != 2 {
		t.Fatalf("frank history = %d, want 2", len(frankHistory))
	}
	if eveHistory[0].Content != "Eve's message" {
		t.Errorf("eve content = %q", eveHistory[0].Content)
	}
	if frankHistory[0].Content != "Frank's message" {
		t.Errorf("frank content = %q", frankHistory[0].Content)
	}
}

func TestNoPersistenceWithoutDataDir(t *testing.T) {
	// Without dataDir, sessions are memory-only (no files created).
	prov := &mockProvider{response: "ok"}
	reg := skill.NewRegistry(nil)
	svc := NewService(reg, prov, "mock")

	svc.SendMessage(context.Background(), "user1", "s1", "Hello")
	history := svc.GetHistory("user1", "s1")
	if len(history) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(history))
	}
	// No crash, no file writes — just works in memory.
}
