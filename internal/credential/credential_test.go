package credential

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateMasterKey_Is32Bytes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "master.key")

	key, err := GenerateMasterKey(path)
	if err != nil {
		t.Fatalf("GenerateMasterKey: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("key length = %d, want 32", len(key))
	}

	// File should exist with 0600 permissions
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("key file not found: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("key file perm = %o, want 600", info.Mode().Perm())
	}
}

func TestLoadMasterKey_MatchesSaved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "master.key")

	generated, err := GenerateMasterKey(path)
	if err != nil {
		t.Fatalf("GenerateMasterKey: %v", err)
	}

	loaded, err := LoadMasterKey(path)
	if err != nil {
		t.Fatalf("LoadMasterKey: %v", err)
	}

	if len(loaded) != 32 {
		t.Fatalf("loaded key length = %d, want 32", len(loaded))
	}
	for i := range generated {
		if generated[i] != loaded[i] {
			t.Fatalf("key mismatch at byte %d", i)
		}
	}
}

func TestLoadMasterKey_InvalidLength(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.key")
	if err := os.WriteFile(path, []byte("tooshort"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadMasterKey(path)
	if err == nil {
		t.Fatal("expected error for invalid key length")
	}
}

func TestLoadMasterKey_FileNotFound(t *testing.T) {
	_, err := LoadMasterKey("/nonexistent/path/key")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestEncrypt_CiphertextDiffersFromPlaintext(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	plaintext := "my-secret-password"
	enc, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if enc == plaintext {
		t.Fatal("ciphertext should differ from plaintext")
	}
	if !IsEncrypted(enc) {
		t.Fatalf("encrypted value should have ENC[AES256:...] format, got %q", enc)
	}
}

func TestDecrypt_Roundtrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	plaintext := "super-secret-value-123"
	enc, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	dec, err := Decrypt(enc, key)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if dec != plaintext {
		t.Fatalf("decrypted = %q, want %q", dec, plaintext)
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	for i := range key1 {
		key1[i] = byte(i)
		key2[i] = byte(i + 1)
	}

	enc, err := Encrypt("secret", key1)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Decrypt(enc, key2)
	if err == nil {
		t.Fatal("expected error decrypting with wrong key")
	}
}

func TestDecrypt_InvalidFormat(t *testing.T) {
	key := make([]byte, 32)

	_, err := Decrypt("not-encrypted", key)
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
}

func TestIsEncrypted(t *testing.T) {
	if IsEncrypted("plaintext") {
		t.Error("plaintext should not be encrypted")
	}
	if !IsEncrypted("ENC[AES256:abc123==]") {
		t.Error("ENC[AES256:...] should be encrypted")
	}
	if IsEncrypted("ENC[AES256:missing-bracket") {
		t.Error("missing closing bracket should not be encrypted")
	}
}

func TestEncrypt_DifferentNonces(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	enc1, err := Encrypt("same-value", key)
	if err != nil {
		t.Fatal(err)
	}
	enc2, err := Encrypt("same-value", key)
	if err != nil {
		t.Fatal(err)
	}

	if enc1 == enc2 {
		t.Error("encrypting same value should produce different ciphertexts (different nonces)")
	}

	// Both should decrypt to same value
	dec1, _ := Decrypt(enc1, key)
	dec2, _ := Decrypt(enc2, key)
	if dec1 != dec2 {
		t.Error("both ciphertexts should decrypt to same value")
	}
}

func TestValidateCredential_EmailRequiresFields(t *testing.T) {
	// Valid email credential
	valid := Credential{
		Name: "work-email",
		Type: "email",
		Data: map[string]string{
			"imap_host": "imap.example.com",
			"imap_user": "user@example.com",
			"imap_pass": "pass123",
			"smtp_host": "smtp.example.com",
			"smtp_user": "user@example.com",
			"smtp_pass": "pass123",
		},
	}
	if err := ValidateCredential(valid); err != nil {
		t.Fatalf("valid email should pass: %v", err)
	}

	// Missing required field
	invalid := Credential{
		Name: "bad-email",
		Type: "email",
		Data: map[string]string{
			"imap_host": "imap.example.com",
			// missing all other fields
		},
	}
	if err := ValidateCredential(invalid); err == nil {
		t.Fatal("email missing required fields should fail")
	}
}

func TestValidateCredential_GenericAcceptsAnything(t *testing.T) {
	cred := Credential{
		Name: "my-generic",
		Type: "generic",
		Data: map[string]string{
			"foo": "bar",
		},
	}
	if err := ValidateCredential(cred); err != nil {
		t.Fatalf("generic should accept any data: %v", err)
	}
}

func TestValidateCredential_UnknownType(t *testing.T) {
	cred := Credential{
		Name: "test",
		Type: "unknown-type",
		Data: map[string]string{"x": "y"},
	}
	if err := ValidateCredential(cred); err == nil {
		t.Fatal("unknown type should fail validation")
	}
}

func TestValidateCredential_EmptyName(t *testing.T) {
	cred := Credential{
		Name: "",
		Type: "generic",
		Data: map[string]string{},
	}
	if err := ValidateCredential(cred); err == nil {
		t.Fatal("empty name should fail validation")
	}
}

func TestValidateCredential_AllBuiltinTypes(t *testing.T) {
	tests := []struct {
		typ    string
		data   map[string]string
		wantOK bool
	}{
		{"http-basic", map[string]string{"username": "u", "password": "p"}, true},
		{"http-basic", map[string]string{"username": "u"}, false},
		{"http-bearer", map[string]string{"token": "t"}, true},
		{"http-bearer", map[string]string{}, false},
		{"openai", map[string]string{"api_key": "sk-xxx"}, true},
		{"anthropic", map[string]string{"api_key": "sk-xxx"}, true},
		{"telegram-bot", map[string]string{"bot_token": "123:ABC"}, true},
		{"telegram-bot", map[string]string{}, false},
	}

	for _, tt := range tests {
		cred := Credential{Name: "test", Type: tt.typ, Data: tt.data}
		err := ValidateCredential(cred)
		if tt.wantOK && err != nil {
			t.Errorf("type=%s data=%v: unexpected error: %v", tt.typ, tt.data, err)
		}
		if !tt.wantOK && err == nil {
			t.Errorf("type=%s data=%v: expected error but got nil", tt.typ, tt.data)
		}
	}
}

func TestMarshalParseRoundtrip(t *testing.T) {
	id := "cred_test1"
	name := "test-cred"
	typ := "generic"
	data := map[string]string{
		"key1": "ENC[AES256:abc123==]",
		"key2": "ENC[AES256:def456==]",
	}

	yaml := marshalCredentialYAML(id, name, typ, data)
	cred, err := parseCredentialYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if cred.ID != id {
		t.Errorf("id = %q, want %q", cred.ID, id)
	}
	if cred.Name != name {
		t.Errorf("name = %q, want %q", cred.Name, name)
	}
	if cred.Type != typ {
		t.Errorf("type = %q, want %q", cred.Type, typ)
	}
	if len(cred.Data) != len(data) {
		t.Fatalf("data length = %d, want %d", len(cred.Data), len(data))
	}
	for k, v := range data {
		if cred.Data[k] != v {
			t.Errorf("data[%s] = %q, want %q", k, cred.Data[k], v)
		}
	}
}
