package skill

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Store loads and saves skills from a user's repo/skills/ directory.
type Store struct {
	dir string
}

// NewStore creates a skill store rooted at the given directory.
func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

// List returns all skills found in the store directory.
func (s *Store) List() ([]*Skill, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("skill: list: %w", err)
	}

	var skills []*Skill
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !isSkillFile(name) {
			continue
		}
		sk, err := s.load(filepath.Join(s.dir, name))
		if err != nil {
			continue
		}
		skills = append(skills, sk)
	}
	return skills, nil
}

// Get loads a skill by name.
func (s *Store) Get(name string) (*Skill, error) {
	// Try YAML first, then JSON.
	for _, ext := range []string{".yaml", ".yml", ".json"} {
		path := filepath.Join(s.dir, name+ext)
		if _, err := os.Stat(path); err == nil {
			return s.load(path)
		}
	}
	return nil, fmt.Errorf("skill: not found: %q", name)
}

// Save writes a skill definition to disk as YAML.
func (s *Store) Save(sk *Skill) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("skill: create dir: %w", err)
	}

	data, err := yaml.Marshal(sk)
	if err != nil {
		return fmt.Errorf("skill: marshal: %w", err)
	}

	path := filepath.Join(s.dir, sk.Name+".yaml")
	return os.WriteFile(path, data, 0o644)
}

// Delete removes a skill file from disk.
func (s *Store) Delete(name string) error {
	for _, ext := range []string{".yaml", ".yml", ".json"} {
		path := filepath.Join(s.dir, name+ext)
		if err := os.Remove(path); err == nil {
			return nil
		}
	}
	return fmt.Errorf("skill: not found: %q", name)
}

// LoadAll loads all skills from the store into the given registry.
func (s *Store) LoadAll(reg *Registry) error {
	skills, err := s.List()
	if err != nil {
		return err
	}
	for _, sk := range skills {
		reg.Register(sk)
	}
	return nil
}

func (s *Store) load(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("skill: read %q: %w", path, err)
	}

	var sk Skill
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &sk); err != nil {
			return nil, fmt.Errorf("skill: parse yaml %q: %w", path, err)
		}
	case ".json":
		if err := json.Unmarshal(data, &sk); err != nil {
			return nil, fmt.Errorf("skill: parse json %q: %w", path, err)
		}
	default:
		return nil, fmt.Errorf("skill: unsupported format %q", ext)
	}

	// Default name from filename if not set.
	if sk.Name == "" {
		base := filepath.Base(path)
		sk.Name = strings.TrimSuffix(base, filepath.Ext(base))
	}

	return &sk, nil
}

func isSkillFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".yaml" || ext == ".yml" || ext == ".json"
}
