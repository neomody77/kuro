package skill

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/neomody77/kuro/internal/credential"
	"github.com/neomody77/kuro/internal/document"
	"github.com/neomody77/kuro/internal/gitstore"
	"github.com/neomody77/kuro/internal/pipeline"
	"github.com/neomody77/kuro/internal/provider"
)

// setupTestEnv creates a temp dir with git repo, credential store, and document store.
func setupTestEnv(t *testing.T) (credStore *credential.Store, docStore *document.Store, workDir string) {
	t.Helper()
	tmpDir := t.TempDir()

	git, err := gitstore.Init(tmpDir)
	if err != nil {
		t.Fatalf("init git: %v", err)
	}

	// Generate master key
	keyPath := filepath.Join(tmpDir, ".master-key")
	key, err := credential.GenerateMasterKey(keyPath)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	credDir := filepath.Join(tmpDir, "credentials")
	os.MkdirAll(credDir, 0o755)
	credStore = credential.NewStore(credDir, key, git)

	docDir := filepath.Join(tmpDir, "documents")
	os.MkdirAll(docDir, 0o755)
	docStore = document.NewStore(docDir, git)

	workDir = filepath.Join(tmpDir, "workspace")
	os.MkdirAll(workDir, 0o755)

	return credStore, docStore, workDir
}

// TestRegistryBasics tests Register, Get, List, Delete.
func TestRegistryBasics(t *testing.T) {
	r := NewRegistry(nil)

	r.Register(&Skill{Name: "test.skill", Description: "A test skill"})
	r.Register(&Skill{Name: "test.other", Description: "Another"})

	s, ok := r.Get("test.skill")
	if !ok {
		t.Fatal("expected to find test.skill")
	}
	if s.Description != "A test skill" {
		t.Errorf("got description %q, want %q", s.Description, "A test skill")
	}

	list := r.List()
	if len(list) != 2 {
		t.Errorf("got %d skills, want 2", len(list))
	}

	r.Delete("test.other")
	list = r.List()
	if len(list) != 1 {
		t.Errorf("got %d skills after delete, want 1", len(list))
	}

	_, ok = r.Get("test.other")
	if ok {
		t.Fatal("expected test.other to be deleted")
	}
}

// TestRegisterDefaults checks all core skills are registered.
func TestRegisterDefaults(t *testing.T) {
	credStore, docStore, workDir := setupTestEnv(t)

	r := NewRegistry(nil)
	RegisterDefaults(r, CoreConfig{
		CredentialStore: credStore,
		DocumentStore:   docStore,
		WorkspaceDir:    workDir,
		DocumentsDir:    filepath.Dir(workDir),
	})

	expected := []string{
		"credential", "document",
		"shell", "email", "http", "file",
		"transform", "template", "ai",
	}

	for _, name := range expected {
		s, ok := r.Get(name)
		if !ok {
			t.Errorf("skill %q not registered", name)
			continue
		}
		if s.Handler == nil && s.Workflow == nil {
			t.Errorf("skill %q has no handler and no workflow", name)
		}
	}

	list := r.List()
	if len(list) < len(expected) {
		t.Errorf("got %d skills, want at least %d", len(list), len(expected))
	}
}

// TestAllCoreSkillsImplementActionHandler verifies all handler-based skills implement ActionHandler.
func TestAllCoreSkillsImplementActionHandler(t *testing.T) {
	credStore, docStore, workDir := setupTestEnv(t)

	r := NewRegistry(nil)
	RegisterDefaults(r, CoreConfig{
		CredentialStore: credStore,
		DocumentStore:   docStore,
		WorkspaceDir:    workDir,
	})

	for _, s := range r.List() {
		if s.Handler == nil {
			continue
		}
		// Verify it satisfies ActionHandler interface
		var _ pipeline.ActionHandler = s.Handler
	}
}

// TestExecuteByName verifies skills can be invoked by name through the registry.
func TestExecuteByName(t *testing.T) {
	credStore, docStore, workDir := setupTestEnv(t)

	r := NewRegistry(nil)
	RegisterDefaults(r, CoreConfig{
		CredentialStore: credStore,
		DocumentStore:   docStore,
		WorkspaceDir:    workDir,
	})

	ctx := context.Background()

	// Invoke shell by name
	result, err := r.Execute(ctx, "shell", map[string]any{
		"command": "echo hello",
	})
	if err != nil {
		t.Fatalf("Execute shell: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if m["stdout"] != "hello\n" {
		t.Errorf("stdout = %q, want %q", m["stdout"], "hello\n")
	}
}

// TestExecuteUnknownSkill returns error.
func TestExecuteUnknownSkill(t *testing.T) {
	r := NewRegistry(nil)
	_, err := r.Execute(context.Background(), "no.such.skill", nil)
	if err == nil {
		t.Fatal("expected error for unknown skill")
	}
}

// --- Credential skill tests ---

func TestCredentialListSkill(t *testing.T) {
	credStore, _, _ := setupTestEnv(t)
	ctx := context.Background()

	credStore.Save(credential.Credential{
		Name: "test-cred",
		Type: "generic",
		Data: map[string]string{"key": "value"},
	})

	handler := &credentialSkill{store: credStore}
	result, err := handler.Execute(ctx, map[string]any{"action": "list"}, nil)
	if err != nil {
		t.Fatalf("credential list: %v", err)
	}
	m := result.(map[string]any)
	items := m["credentials"].([]map[string]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 credential, got %d", len(items))
	}
	if items[0]["name"] != "test-cred" {
		t.Errorf("name = %v, want test-cred", items[0]["name"])
	}
	if items[0]["id"] == nil || items[0]["id"] == "" {
		t.Error("credential list should include id")
	}
	// Verify no secret data in list output
	if _, hasData := items[0]["data"]; hasData {
		t.Error("credential list should not expose data")
	}
}

func TestCredentialGetSkill(t *testing.T) {
	credStore, _, _ := setupTestEnv(t)
	ctx := context.Background()

	id, _ := credStore.Save(credential.Credential{
		Name: "my-key",
		Type: "generic",
		Data: map[string]string{"secret": "super-secret-value"},
	})

	handler := &credentialSkill{store: credStore}
	result, err := handler.Execute(ctx, map[string]any{"action": "get", "id": id}, nil)
	if err != nil {
		t.Fatalf("credential get: %v", err)
	}
	m := result.(map[string]any)
	data := m["data"].(map[string]string)
	if data["secret"] != "super-secret-value" {
		t.Errorf("decrypted value = %q, want %q", data["secret"], "super-secret-value")
	}
}

func TestCredentialSaveSkill(t *testing.T) {
	credStore, _, _ := setupTestEnv(t)
	ctx := context.Background()

	handler := &credentialSkill{store: credStore}
	result, err := handler.Execute(ctx, map[string]any{
		"action": "save",
		"name":   "new-cred",
		"type":   "generic",
		"data":   map[string]string{"token": "abc123"},
	}, nil)
	if err != nil {
		t.Fatalf("credential save: %v", err)
	}
	m := result.(map[string]any)
	if m["ok"] != true {
		t.Error("expected ok=true")
	}
	id := m["id"].(string)

	cred, err := credStore.Get(id)
	if err != nil {
		t.Fatalf("get after save: %v", err)
	}
	if cred.Data["token"] != "abc123" {
		t.Errorf("token = %q, want abc123", cred.Data["token"])
	}
}

func TestCredentialDeleteSkill(t *testing.T) {
	credStore, _, _ := setupTestEnv(t)
	ctx := context.Background()

	id, _ := credStore.Save(credential.Credential{
		Name: "to-delete",
		Type: "generic",
		Data: map[string]string{"a": "b"},
	})

	handler := &credentialSkill{store: credStore}
	_, err := handler.Execute(ctx, map[string]any{"action": "delete", "id": id}, nil)
	if err != nil {
		t.Fatalf("credential delete: %v", err)
	}

	_, err = credStore.Get(id)
	if err == nil {
		t.Error("expected error after delete")
	}
}

// --- Document skill tests ---

func TestDocumentListSkill(t *testing.T) {
	_, docStore, _ := setupTestEnv(t)
	ctx := context.Background()

	docStore.Put("notes.md", "some notes")
	docStore.Put("sub/readme.md", "nested doc")

	handler := &documentSkill{store: docStore}
	result, err := handler.Execute(ctx, map[string]any{"action": "list"}, nil)
	if err != nil {
		t.Fatalf("document list: %v", err)
	}
	m := result.(map[string]any)
	count := m["count"].(int)
	if count < 1 {
		t.Errorf("expected at least 1 entry, got %d", count)
	}
}

func TestDocumentGetSkill(t *testing.T) {
	_, docStore, _ := setupTestEnv(t)
	ctx := context.Background()

	docStore.Put("hello.md", "# Hello World")

	handler := &documentSkill{store: docStore}
	result, err := handler.Execute(ctx, map[string]any{"action": "get", "path": "hello.md"}, nil)
	if err != nil {
		t.Fatalf("document get: %v", err)
	}
	m := result.(map[string]any)
	if m["content"] != "# Hello World" {
		t.Errorf("content = %q, want %q", m["content"], "# Hello World")
	}
}

func TestDocumentSaveSkill(t *testing.T) {
	_, docStore, _ := setupTestEnv(t)
	ctx := context.Background()

	handler := &documentSkill{store: docStore}
	result, err := handler.Execute(ctx, map[string]any{
		"action":  "save",
		"path":    "new-doc.md",
		"content": "new content",
	}, nil)
	if err != nil {
		t.Fatalf("document save: %v", err)
	}
	m := result.(map[string]any)
	if m["ok"] != true {
		t.Error("expected ok=true")
	}

	doc, err := docStore.Get("new-doc.md")
	if err != nil {
		t.Fatalf("get after save: %v", err)
	}
	if doc.Content != "new content" {
		t.Errorf("content = %q, want %q", doc.Content, "new content")
	}
}

func TestDocumentDeleteSkill(t *testing.T) {
	_, docStore, _ := setupTestEnv(t)
	ctx := context.Background()

	docStore.Put("delete-me.md", "bye")

	handler := &documentSkill{store: docStore}
	_, err := handler.Execute(ctx, map[string]any{"action": "delete", "path": "delete-me.md"}, nil)
	if err != nil {
		t.Fatalf("document delete: %v", err)
	}

	_, err = docStore.Get("delete-me.md")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestDocumentSearchSkill(t *testing.T) {
	_, docStore, _ := setupTestEnv(t)
	ctx := context.Background()

	docStore.Put("alpha.md", "this contains the keyword banana")
	docStore.Put("beta.md", "no match here")

	handler := &documentSkill{store: docStore}
	result, err := handler.Execute(ctx, map[string]any{"action": "search", "query": "banana"}, nil)
	if err != nil {
		t.Fatalf("document search: %v", err)
	}
	m := result.(map[string]any)
	count := m["count"].(int)
	if count != 1 {
		t.Errorf("expected 1 match, got %d", count)
	}
	items := m["results"].([]map[string]any)
	if items[0]["path"] != "alpha.md" {
		t.Errorf("path = %v, want alpha.md", items[0]["path"])
	}
}

// --- Shell skill test ---

func TestShellExecSkill(t *testing.T) {
	r := NewRegistry(nil)
	RegisterDefaults(r, CoreConfig{})

	ctx := context.Background()
	result, err := r.Execute(ctx, "shell", map[string]any{
		"command": "echo 'hello world'",
	})
	if err != nil {
		t.Fatalf("shell: %v", err)
	}
	m := result.(map[string]any)
	if m["stdout"] != "hello world\n" {
		t.Errorf("stdout = %q, want %q", m["stdout"], "hello world\n")
	}
	if m["exit_code"] != 0 {
		t.Errorf("exit_code = %v, want 0", m["exit_code"])
	}
}

func TestShellExecSkillCapturesStderr(t *testing.T) {
	r := NewRegistry(nil)
	RegisterDefaults(r, CoreConfig{})

	ctx := context.Background()
	result, err := r.Execute(ctx, "shell", map[string]any{
		"command": "echo err >&2; exit 1",
	})
	if err != nil {
		t.Fatalf("shell: %v", err)
	}
	m := result.(map[string]any)
	if m["stderr"] != "err\n" {
		t.Errorf("stderr = %q, want %q", m["stderr"], "err\n")
	}
	if m["exit_code"] != 1 {
		t.Errorf("exit_code = %v, want 1", m["exit_code"])
	}
}

// --- File skill tests ---

func TestFileReadWriteSkill(t *testing.T) {
	workDir := t.TempDir()

	r := NewRegistry(nil)
	RegisterDefaults(r, CoreConfig{WorkspaceDir: workDir})

	ctx := context.Background()

	// Write a file
	result, err := r.Execute(ctx, "file", map[string]any{
		"action": "write",
		"path":   "test.txt",
		"data":   "hello file",
	})
	if err != nil {
		t.Fatalf("file write: %v", err)
	}
	m := result.(map[string]any)
	if m["written"].(int) != 10 {
		t.Errorf("written = %v, want 10", m["written"])
	}

	// Read it back
	result, err = r.Execute(ctx, "file", map[string]any{
		"action": "read",
		"path":   "test.txt",
	})
	if err != nil {
		t.Fatalf("file read: %v", err)
	}
	m = result.(map[string]any)
	if m["content"] != "hello file" {
		t.Errorf("content = %q, want %q", m["content"], "hello file")
	}
}

func TestFileListSkill(t *testing.T) {
	workDir := t.TempDir()
	os.WriteFile(filepath.Join(workDir, "a.txt"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(workDir, "b.txt"), []byte("b"), 0o644)
	os.MkdirAll(filepath.Join(workDir, "subdir"), 0o755)

	r := NewRegistry(nil)
	RegisterDefaults(r, CoreConfig{WorkspaceDir: workDir})

	ctx := context.Background()
	result, err := r.Execute(ctx, "file", map[string]any{
		"action": "list",
		"path":   ".",
	})
	if err != nil {
		t.Fatalf("file list: %v", err)
	}
	m := result.(map[string]any)
	count := m["count"].(int)
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

// --- Transform skill tests ---

func TestTransformJqFilter(t *testing.T) {
	r := NewRegistry(nil)
	RegisterDefaults(r, CoreConfig{})

	ctx := context.Background()

	input := `[{"name":"alice","age":30},{"name":"bob","age":25}]`
	result, err := r.Execute(ctx, "transform", map[string]any{
		"input": input,
		"expr":  "[0] | .name",
	})
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	m := result.(map[string]any)
	if m["output"] != "alice" {
		t.Errorf("output = %v, want alice", m["output"])
	}
}

func TestTransformJqMap(t *testing.T) {
	r := NewRegistry(nil)
	RegisterDefaults(r, CoreConfig{})

	ctx := context.Background()

	input := `[{"name":"alice"},{"name":"bob"}]`
	result, err := r.Execute(ctx, "transform", map[string]any{
		"input": input,
		"expr":  "map(.name)",
	})
	if err != nil {
		t.Fatalf("transform map: %v", err)
	}
	m := result.(map[string]any)
	output, ok := m["output"].([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", m["output"])
	}
	if len(output) != 2 || output[0] != "alice" || output[1] != "bob" {
		t.Errorf("output = %v, want [alice bob]", output)
	}
}

func TestTransformJqSelect(t *testing.T) {
	r := NewRegistry(nil)
	RegisterDefaults(r, CoreConfig{})

	ctx := context.Background()

	input := `[{"name":"alice","age":30},{"name":"bob","age":25}]`
	result, err := r.Execute(ctx, "transform", map[string]any{
		"input": input,
		"expr":  `select(.age > 28)`,
	})
	if err != nil {
		t.Fatalf("transform select: %v", err)
	}
	m := result.(map[string]any)
	output, ok := m["output"].([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", m["output"])
	}
	if len(output) != 1 {
		t.Errorf("expected 1 match, got %d", len(output))
	}
}

func TestTransformJqLength(t *testing.T) {
	r := NewRegistry(nil)
	RegisterDefaults(r, CoreConfig{})

	ctx := context.Background()

	input := `[1,2,3,4,5]`
	result, err := r.Execute(ctx, "transform", map[string]any{
		"input": input,
		"expr":  "length",
	})
	if err != nil {
		t.Fatalf("transform length: %v", err)
	}
	m := result.(map[string]any)
	if m["output"] != 5 {
		t.Errorf("output = %v, want 5", m["output"])
	}
}

// --- Template skill test ---

func TestTemplateRenderSkill(t *testing.T) {
	r := NewRegistry(nil)
	RegisterDefaults(r, CoreConfig{})

	ctx := context.Background()
	result, err := r.Execute(ctx, "template", map[string]any{
		"template": "Hello, {{.name}}! You have {{.count}} items.",
		"vars": map[string]any{
			"name":  "Alice",
			"count": 5,
		},
	})
	if err != nil {
		t.Fatalf("template: %v", err)
	}
	m := result.(map[string]any)
	if m["output"] != "Hello, Alice! You have 5 items." {
		t.Errorf("output = %q", m["output"])
	}
}

// --- HTTP skill test ---

func TestHTTPRequestSkill(t *testing.T) {
	r := NewRegistry(nil)
	RegisterDefaults(r, CoreConfig{})

	s, ok := r.Get("http")
	if !ok {
		t.Fatal("http not registered")
	}
	if s.Handler == nil {
		t.Fatal("http has no handler")
	}
}

// --- AI skill test ---

type mockProvider struct {
	response string
}

func (p *mockProvider) Complete(ctx context.Context, req *provider.CompletionRequest) (*provider.CompletionResponse, error) {
	return &provider.CompletionResponse{
		Content: p.response,
		Model:   "mock",
		Usage:   provider.Usage{PromptTokens: 10, CompletionTokens: 5},
	}, nil
}

func TestAICompleteSkill(t *testing.T) {
	providers := provider.NewRegistry()
	providers.Register("mock", &mockProvider{response: "The answer is 42"})

	r := NewRegistry(nil)
	RegisterDefaults(r, CoreConfig{Providers: providers})

	ctx := context.Background()
	result, err := r.Execute(ctx, "ai", map[string]any{
		"provider": "mock",
		"input":    "What is the meaning of life?",
	})
	if err != nil {
		t.Fatalf("ai: %v", err)
	}
	m := result.(map[string]any)
	if m["output"] != "The answer is 42" {
		t.Errorf("output = %q, want %q", m["output"], "The answer is 42")
	}
}

// --- Registry skill listing test ---

func TestRegistryListsAllSkills(t *testing.T) {
	credStore, docStore, workDir := setupTestEnv(t)

	r := NewRegistry(nil)
	RegisterDefaults(r, CoreConfig{
		CredentialStore: credStore,
		DocumentStore:   docStore,
		WorkspaceDir:    workDir,
	})

	list := r.List()
	names := make(map[string]bool)
	for _, s := range list {
		names[s.Name] = true
	}

	required := []string{
		"credential", "document",
		"shell", "email", "http", "file",
		"transform", "template", "ai",
	}

	for _, name := range required {
		if !names[name] {
			t.Errorf("missing required skill: %s", name)
		}
	}
}

// --- Credential skill validation tests ---

func TestCredentialSaveWithMapStringAny(t *testing.T) {
	credStore, _, _ := setupTestEnv(t)
	ctx := context.Background()

	handler := &credentialSkill{store: credStore}
	result, err := handler.Execute(ctx, map[string]any{
		"action": "save",
		"name":   "test",
		"type":   "generic",
		"data":   map[string]any{"token": "xyz"},
	}, nil)
	if err != nil {
		t.Fatalf("credential save with map[string]any: %v", err)
	}
	id := result.(map[string]any)["id"].(string)

	cred, _ := credStore.Get(id)
	if cred.Data["token"] != "xyz" {
		t.Errorf("token = %q, want xyz", cred.Data["token"])
	}
}

func TestCredentialGetMissingID(t *testing.T) {
	credStore, _, _ := setupTestEnv(t)
	handler := &credentialSkill{store: credStore}
	_, err := handler.Execute(context.Background(), map[string]any{"action": "get"}, nil)
	if err == nil {
		t.Error("expected error for missing id")
	}
}

func TestDocumentGetMissingPath(t *testing.T) {
	_, docStore, _ := setupTestEnv(t)
	handler := &documentSkill{store: docStore}
	_, err := handler.Execute(context.Background(), map[string]any{"action": "get"}, nil)
	if err == nil {
		t.Error("expected error for missing path")
	}
}

// --- JSON roundtrip test for Skill struct ---

func TestSkillJSON(t *testing.T) {
	s := &Skill{
		Name:        "test.skill",
		Description: "A test",
		Inputs:      []SkillParam{{Name: "x", Required: true}},
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var s2 Skill
	if err := json.Unmarshal(data, &s2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s2.Name != "test.skill" {
		t.Errorf("name = %q", s2.Name)
	}
	if len(s2.Inputs) != 1 || s2.Inputs[0].Name != "x" {
		t.Errorf("inputs = %v", s2.Inputs)
	}
}
