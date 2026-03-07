// Package credential handles AES-256-GCM encryption and credential CRUD operations.
package credential

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// GenerateID generates a short random credential ID like "cred_a1b2c3d4".
func GenerateID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return "cred_" + hex.EncodeToString(b)
}

// Credential represents a stored credential with encrypted data fields.
type Credential struct {
	ID   string            `json:"id"`
	Name string            `json:"name"`
	Type string            `json:"type"`
	Data map[string]string `json:"data"`
}

// CredentialType defines a schema for a credential type with required fields.
type CredentialType struct {
	Name   string
	Fields []string
}

// BuiltinTypes defines the built-in credential type schemas.
var BuiltinTypes = map[string]CredentialType{
	"email": {
		Name:   "email",
		Fields: []string{"imap_host", "imap_user", "imap_pass", "smtp_host", "smtp_user", "smtp_pass"},
	},
	"imap": {
		Name:   "imap",
		Fields: []string{"imap_host", "imap_user", "imap_pass"},
	},
	"smtp": {
		Name:   "smtp",
		Fields: []string{"smtp_host", "smtp_user", "smtp_pass"},
	},
	"http-basic": {
		Name:   "http-basic",
		Fields: []string{"username", "password"},
	},
	"http-bearer": {
		Name:   "http-bearer",
		Fields: []string{"token"},
	},
	"openai": {
		Name:   "openai",
		Fields: []string{"api_key"},
	},
	"anthropic": {
		Name:   "anthropic",
		Fields: []string{"api_key"},
	},
	"telegram-bot": {
		Name:   "telegram-bot",
		Fields: []string{"bot_token"},
	},
	"generic": {
		Name:   "generic",
		Fields: []string{},
	},
}

// GenerateMasterKey generates a random 32-byte master key and writes it to the given path.
func GenerateMasterKey(path string) ([]byte, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("credential: generate key: %w", err)
	}
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, fmt.Errorf("credential: write key: %w", err)
	}
	return key, nil
}

// LoadMasterKey reads a 32-byte master key from the given path.
func LoadMasterKey(path string) ([]byte, error) {
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("credential: read key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("credential: invalid key length %d, expected 32", len(key))
	}
	return key, nil
}

// Encrypt encrypts plaintext using AES-256-GCM and returns "ENC[AES256:base64...]".
func Encrypt(plaintext string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("credential: new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("credential: new gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("credential: generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	encoded := base64.StdEncoding.EncodeToString(ciphertext)
	return "ENC[AES256:" + encoded + "]", nil
}

// Decrypt decrypts an "ENC[AES256:base64...]" string using AES-256-GCM.
func Decrypt(ciphertext string, key []byte) (string, error) {
	if !strings.HasPrefix(ciphertext, "ENC[AES256:") || !strings.HasSuffix(ciphertext, "]") {
		return "", fmt.Errorf("credential: invalid encrypted format")
	}

	encoded := ciphertext[len("ENC[AES256:") : len(ciphertext)-1]
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("credential: base64 decode: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("credential: new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("credential: new gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("credential: ciphertext too short")
	}

	nonce, sealed := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", fmt.Errorf("credential: decrypt: %w", err)
	}

	return string(plaintext), nil
}

// ValidateCredential checks that a credential has a valid name, a known type,
// and all required fields for that type.
func ValidateCredential(cred Credential) error {
	if cred.Name == "" {
		return fmt.Errorf("credential: name is required")
	}
	ct, ok := BuiltinTypes[cred.Type]
	if !ok {
		return fmt.Errorf("credential: unknown type %q", cred.Type)
	}
	for _, field := range ct.Fields {
		if _, exists := cred.Data[field]; !exists {
			return fmt.Errorf("credential: type %q requires field %q", cred.Type, field)
		}
	}
	return nil
}

// IsEncrypted returns true if the value has the ENC[AES256:...] format.
func IsEncrypted(value string) bool {
	return strings.HasPrefix(value, "ENC[AES256:") && strings.HasSuffix(value, "]")
}
