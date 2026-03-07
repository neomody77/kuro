// Package settings manages persistent application settings stored in YAML.
package settings

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/neomody77/kuro/internal/provider"
	"gopkg.in/yaml.v3"
)

// ProviderConfig holds the configuration for an AI provider endpoint.
type ProviderConfig struct {
	ID      string   `json:"id" yaml:"id"`
	Name    string   `json:"name" yaml:"name"`
	Type    string   `json:"type" yaml:"type"`
	BaseURL string   `json:"base_url" yaml:"base_url"`
	APIKey  string   `json:"api_key" yaml:"api_key"`
	Models  []string `json:"models" yaml:"models"`
}

// ActiveModel identifies the currently selected provider and model.
type ActiveModel struct {
	ProviderID string `json:"provider_id" yaml:"provider_id"`
	Model      string `json:"model" yaml:"model"`
}

// Settings holds all persistent application settings.
type Settings struct {
	Providers   []ProviderConfig `json:"providers" yaml:"providers"`
	ActiveModel ActiveModel      `json:"active_model" yaml:"active_model"`
}

// Store manages loading and saving settings from disk.
type Store struct {
	mu   sync.RWMutex
	path string
	data Settings
}

// NewStore creates a new settings store and loads existing settings from disk.
func NewStore(path string) *Store {
	s := &Store{path: path}
	_ = s.Load()
	return s
}

// Load reads settings from disk. If the file does not exist, empty defaults are used.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.data = Settings{}
			return nil
		}
		return fmt.Errorf("settings: read %s: %w", s.path, err)
	}

	var settings Settings
	if err := yaml.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("settings: parse %s: %w", s.path, err)
	}
	s.data = settings
	return nil
}

// Save writes the current settings to disk, creating the directory if needed.
func (s *Store) Save() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("settings: create dir %s: %w", dir, err)
	}

	out, err := yaml.Marshal(&s.data)
	if err != nil {
		return fmt.Errorf("settings: marshal: %w", err)
	}

	if err := os.WriteFile(s.path, out, 0o600); err != nil {
		return fmt.Errorf("settings: write %s: %w", s.path, err)
	}
	return nil
}

// maskKey masks an API key, showing only the last 4 characters.
func maskKey(key string) string {
	if len(key) <= 4 {
		return "***"
	}
	return "***..." + key[len(key)-4:]
}

// isMaskedKey returns true if the key looks like a masked value.
func isMaskedKey(key string) bool {
	return strings.HasPrefix(key, "***")
}

// Get returns the current settings with API keys masked for safe display.
func (s *Store) Get() Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := Settings{
		ActiveModel: s.data.ActiveModel,
		Providers:   make([]ProviderConfig, len(s.data.Providers)),
	}
	for i, p := range s.data.Providers {
		result.Providers[i] = p
		if p.APIKey != "" {
			result.Providers[i].APIKey = maskKey(p.APIKey)
		}
	}
	return result
}

// GetFull returns the current settings with full (unmasked) API keys.
func (s *Store) GetFull() Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := Settings{
		ActiveModel: s.data.ActiveModel,
		Providers:   make([]ProviderConfig, len(s.data.Providers)),
	}
	copy(result.Providers, s.data.Providers)
	return result
}

// SetActiveModel validates and sets the active provider and model.
func (s *Store) SetActiveModel(providerID, model string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	found := false
	for _, p := range s.data.Providers {
		if p.ID == providerID {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("settings: provider %q not found", providerID)
	}

	s.data.ActiveModel = ActiveModel{ProviderID: providerID, Model: model}
	return s.Save()
}

// AddProvider adds a new provider or updates an existing one by ID.
// If the incoming key is masked, the existing key is preserved.
func (s *Store) AddProvider(p ProviderConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, existing := range s.data.Providers {
		if existing.ID == p.ID {
			// If the incoming key is masked, keep the existing key.
			if isMaskedKey(p.APIKey) {
				p.APIKey = existing.APIKey
			}
			s.data.Providers[i] = p
			return s.Save()
		}
	}

	s.data.Providers = append(s.data.Providers, p)
	return s.Save()
}

// DeleteProvider removes a provider by ID. If the active model references
// this provider, the active model is cleared.
func (s *Store) DeleteProvider(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	found := false
	providers := make([]ProviderConfig, 0, len(s.data.Providers))
	for _, p := range s.data.Providers {
		if p.ID == id {
			found = true
			continue
		}
		providers = append(providers, p)
	}
	if !found {
		return fmt.Errorf("settings: provider %q not found", id)
	}

	s.data.Providers = providers
	if s.data.ActiveModel.ProviderID == id {
		s.data.ActiveModel = ActiveModel{}
	}
	return s.Save()
}

// GetProvider returns a provider by ID with the full (unmasked) API key.
func (s *Store) GetProvider(id string) (ProviderConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, p := range s.data.Providers {
		if p.ID == id {
			return p, true
		}
	}
	return ProviderConfig{}, false
}

// TestProvider attempts a minimal completion to verify that the provider works.
func (s *Store) TestProvider(ctx context.Context, p ProviderConfig) error {
	// If the key is masked, try to fill it from an existing provider.
	if isMaskedKey(p.APIKey) {
		existing, ok := s.GetProvider(p.ID)
		if ok {
			p.APIKey = existing.APIKey
		}
	}

	if p.BaseURL == "" {
		return fmt.Errorf("settings: base_url is required")
	}
	if p.APIKey == "" {
		return fmt.Errorf("settings: api_key is required")
	}

	prov := provider.NewOpenAIProvider(p.BaseURL, p.APIKey)

	model := "gpt-3.5-turbo"
	if len(p.Models) > 0 {
		model = p.Models[0]
	}

	_, err := prov.Complete(ctx, &provider.CompletionRequest{
		Model: model,
		Messages: []provider.Message{
			{Role: "user", Content: "ping"},
		},
	})
	if err != nil {
		return fmt.Errorf("settings: provider test failed: %w", err)
	}
	return nil
}
