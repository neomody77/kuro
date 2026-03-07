package credential

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neomody77/kuro/internal/gitstore"
)

// setupStore creates a temp git repo with a credential store for testing.
func setupStore(t *testing.T) (*Store, *gitstore.Store) {
	t.Helper()

	dir := t.TempDir()
	gs, err := gitstore.Init(dir)
	if err != nil {
		t.Fatalf("gitstore.Init: %v", err)
	}

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	credDir := filepath.Join(dir, "credentials")
	store := NewStore(credDir, key, gs)
	return store, gs
}

func TestSave_EncryptedInFileAndCommitExists(t *testing.T) {
	store, gs := setupStore(t)

	cred := Credential{
		Name: "work-email",
		Type: "email",
		Data: map[string]string{
			"imap_host": "imap.example.com",
			"imap_user": "user@example.com",
			"imap_pass": "secret-password",
			"smtp_host": "smtp.example.com",
			"smtp_user": "user@example.com",
			"smtp_pass": "secret-password",
		},
	}

	id, err := store.Save(cred)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify file exists (named by ID) and all data fields are encrypted
	filePath := filepath.Join(store.dir, id+".yaml")
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("file not found: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "ENC[AES256:") {
		t.Error("file should contain encrypted values with ENC[] format")
	}
	// Plaintext secrets must NOT appear in the file
	if strings.Contains(contentStr, "secret-password") {
		t.Error("plaintext secret found in file")
	}

	// Verify git commit exists
	commits, err := gs.Log(10)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(commits) == 0 {
		t.Fatal("expected at least 1 commit after save")
	}
	if !strings.Contains(commits[0].Message, "work-email") {
		t.Errorf("commit message = %q, should mention credential name", commits[0].Message)
	}
}

func TestSave_ThenLoad_Roundtrip(t *testing.T) {
	store, _ := setupStore(t)

	original := Credential{
		Name: "test-cred",
		Type: "generic",
		Data: map[string]string{
			"api_key": "sk-test-12345",
			"secret":  "super-secret",
		},
	}

	id, err := store.Save(original)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if loaded.Name != original.Name {
		t.Errorf("name = %q, want %q", loaded.Name, original.Name)
	}
	if loaded.Type != original.Type {
		t.Errorf("type = %q, want %q", loaded.Type, original.Type)
	}
	for k, v := range original.Data {
		if loaded.Data[k] != v {
			t.Errorf("data[%s] = %q, want %q", k, loaded.Data[k], v)
		}
	}
}

func TestList_ReturnsNamesAndTypes_NoSecrets(t *testing.T) {
	store, _ := setupStore(t)

	creds := []Credential{
		{Name: "alpha", Type: "generic", Data: map[string]string{"key": "secret-alpha"}},
		{Name: "beta", Type: "openai", Data: map[string]string{"api_key": "sk-secret-beta"}},
	}

	for _, c := range creds {
		if _, err := store.Save(c); err != nil {
			t.Fatalf("Save %s: %v", c.Name, err)
		}
	}

	list, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(list) != 2 {
		t.Fatalf("expected 2 credentials, got %d", len(list))
	}

	// Should be sorted
	if list[0].Name != "alpha" {
		t.Errorf("list[0].Name = %q, want %q", list[0].Name, "alpha")
	}
	if list[1].Name != "beta" {
		t.Errorf("list[1].Name = %q, want %q", list[1].Name, "beta")
	}

	// Types should be present
	if list[0].Type != "generic" {
		t.Errorf("list[0].Type = %q, want %q", list[0].Type, "generic")
	}
	if list[1].Type != "openai" {
		t.Errorf("list[1].Type = %q, want %q", list[1].Type, "openai")
	}

	// No secret data should be present
	for _, c := range list {
		if len(c.Data) != 0 {
			t.Errorf("credential %q has data (secrets should not be in list output)", c.Name)
		}
	}
}

func TestList_EmptyDir(t *testing.T) {
	store, _ := setupStore(t)

	list, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if list != nil && len(list) != 0 {
		t.Fatalf("expected empty list, got %d", len(list))
	}
}

func TestDelete_FileGoneAndCommitExists(t *testing.T) {
	store, gs := setupStore(t)

	cred := Credential{
		Name: "to-delete",
		Type: "generic",
		Data: map[string]string{"key": "value"},
	}

	id, err := store.Save(cred)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify file exists before delete (named by ID)
	filePath := filepath.Join(store.dir, id+".yaml")
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("file should exist before delete: %v", err)
	}

	if err := store.Delete(id); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// File should be gone
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("file should not exist after delete")
	}

	// Git commit should exist for deletion
	commits, err := gs.Log(10)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	found := false
	for _, c := range commits {
		if strings.Contains(c.Message, "delete") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected a git commit for the deletion")
	}

	// List should be empty now
	list, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected empty list after delete, got %d", len(list))
	}
}

func TestDelete_NotFound(t *testing.T) {
	store, _ := setupStore(t)

	err := store.Delete("nonexistent")
	if err == nil {
		t.Fatal("expected error deleting nonexistent credential")
	}
}

func TestGet_NotFound(t *testing.T) {
	store, _ := setupStore(t)

	// Ensure the credentials directory exists
	os.MkdirAll(store.dir, 0o755)

	_, err := store.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error getting nonexistent credential")
	}
}

func TestSave_UpdateExisting(t *testing.T) {
	store, _ := setupStore(t)

	cred1 := Credential{
		Name: "updatable",
		Type: "generic",
		Data: map[string]string{"key": "original-value"},
	}
	id, err := store.Save(cred1)
	if err != nil {
		t.Fatalf("Save original: %v", err)
	}

	// Update using the same ID
	cred2 := Credential{
		ID:   id,
		Name: "updatable",
		Type: "generic",
		Data: map[string]string{"key": "updated-value"},
	}
	if _, err := store.Save(cred2); err != nil {
		t.Fatalf("Save updated: %v", err)
	}

	loaded, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if loaded.Data["key"] != "updated-value" {
		t.Errorf("data[key] = %q, want %q", loaded.Data["key"], "updated-value")
	}
}

func TestSave_AlreadyEncryptedKeptAsIs(t *testing.T) {
	store, _ := setupStore(t)

	// First save to get an encrypted value
	cred := Credential{
		Name: "test",
		Type: "generic",
		Data: map[string]string{"key": "plain-value"},
	}
	id, err := store.Save(cred)
	if err != nil {
		t.Fatal(err)
	}

	// Read the raw file to get the encrypted value
	raw, err := os.ReadFile(filepath.Join(store.dir, id+".yaml"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parseCredentialYAML(raw)
	if err != nil {
		t.Fatal(err)
	}
	encValue := parsed.Data["key"]
	if !IsEncrypted(encValue) {
		t.Fatal("expected encrypted value in file")
	}

	// Save again with the already-encrypted value (same ID)
	cred2 := Credential{
		ID:   id,
		Name: "test",
		Type: "generic",
		Data: map[string]string{"key": encValue},
	}
	if _, err := store.Save(cred2); err != nil {
		t.Fatal(err)
	}

	// Should still decrypt to the original value
	loaded, err := store.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Data["key"] != "plain-value" {
		t.Errorf("data[key] = %q, want %q", loaded.Data["key"], "plain-value")
	}
}

func TestMultipleCredentials(t *testing.T) {
	store, _ := setupStore(t)

	names := []string{"cred-a", "cred-b", "cred-c"}
	ids := make(map[string]string) // name -> id
	for _, name := range names {
		cred := Credential{
			Name: name,
			Type: "generic",
			Data: map[string]string{"secret": "value-" + name},
		}
		id, err := store.Save(cred)
		if err != nil {
			t.Fatalf("Save %s: %v", name, err)
		}
		ids[name] = id
	}

	list, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 credentials, got %d", len(list))
	}

	// Each credential should load correctly by ID
	for _, name := range names {
		loaded, err := store.Get(ids[name])
		if err != nil {
			t.Fatalf("Get %s: %v", name, err)
		}
		expected := "value-" + name
		if loaded.Data["secret"] != expected {
			t.Errorf("%s: data[secret] = %q, want %q", name, loaded.Data["secret"], expected)
		}
	}
}
