package credential

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/neomody77/kuro/internal/gitstore"
)

// Store manages credentials on disk with encryption and git versioning.
// Files are named by ID: {id}.yaml
type Store struct {
	dir string
	key []byte
	git *gitstore.Store
}

// NewStore creates a credential store. dir is the credentials directory inside the git repo.
// key is the 32-byte master key. git is the git store for the repo containing dir.
func NewStore(dir string, key []byte, git *gitstore.Store) *Store {
	return &Store{dir: dir, key: key, git: git}
}

// List returns all credentials with id, name and type only (no secret data).
func (s *Store) List() ([]Credential, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("credential: list: %w", err)
	}

	var creds []Credential
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".yaml")
		cred, err := s.readFile(id)
		if err != nil {
			continue
		}
		// Return only id, name, and type — no data
		creds = append(creds, Credential{ID: cred.ID, Name: cred.Name, Type: cred.Type})
	}
	sort.Slice(creds, func(i, j int) bool { return creds[i].Name < creds[j].Name })
	return creds, nil
}

// Get returns a credential by ID with all data fields decrypted.
func (s *Store) Get(id string) (*Credential, error) {
	cred, err := s.readFile(id)
	if err != nil {
		return nil, err
	}

	// Decrypt all encrypted fields
	for k, v := range cred.Data {
		if IsEncrypted(v) {
			plain, decErr := Decrypt(v, s.key)
			if decErr != nil {
				return nil, fmt.Errorf("credential: decrypt field %s: %w", k, decErr)
			}
			cred.Data[k] = plain
		}
	}

	return cred, nil
}

// Save encrypts credential data fields, writes the YAML file, and commits to git.
// If ID is empty, generates one. Returns the credential ID.
func (s *Store) Save(cred Credential) (string, error) {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return "", fmt.Errorf("credential: mkdir: %w", err)
	}

	if cred.ID == "" {
		cred.ID = GenerateID()
	}

	// Encrypt all data fields
	encData := make(map[string]string, len(cred.Data))
	for k, v := range cred.Data {
		if IsEncrypted(v) {
			encData[k] = v
		} else {
			enc, err := Encrypt(v, s.key)
			if err != nil {
				return "", fmt.Errorf("credential: encrypt field %s: %w", k, err)
			}
			encData[k] = enc
		}
	}

	content := marshalCredentialYAML(cred.ID, cred.Name, cred.Type, encData)
	filename := cred.ID + ".yaml"
	path := filepath.Join(s.dir, filename)

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("credential: write: %w", err)
	}

	// Git add and commit
	relPath, err := filepath.Rel(s.git.Path(), path)
	if err != nil {
		relPath = path
	}
	if err := s.git.Add(relPath); err != nil {
		return "", fmt.Errorf("credential: git add: %w", err)
	}
	if err := s.git.Commit(fmt.Sprintf("credential: save %s (%s)", cred.Name, cred.ID)); err != nil {
		return "", fmt.Errorf("credential: git commit: %w", err)
	}

	return cred.ID, nil
}

// Delete removes a credential file by ID and commits the deletion.
func (s *Store) Delete(id string) error {
	path := filepath.Join(s.dir, id+".yaml")
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("credential: not found: %s", id)
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("credential: delete: %w", err)
	}

	relPath, err := filepath.Rel(s.git.Path(), path)
	if err != nil {
		relPath = path
	}
	if err := s.git.Add(relPath); err != nil {
		return fmt.Errorf("credential: git add: %w", err)
	}
	if err := s.git.Commit(fmt.Sprintf("credential: delete %s", id)); err != nil {
		return fmt.Errorf("credential: git commit: %w", err)
	}

	return nil
}

// readFile reads and parses a credential YAML file by stem name (without .yaml).
func (s *Store) readFile(stem string) (*Credential, error) {
	path := filepath.Join(s.dir, stem+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("credential: read %s: %w", stem, err)
	}
	return parseCredentialYAML(data)
}

// marshalCredentialYAML produces a YAML representation of a credential.
//
//	id: cred_a1b2c3d4
//	name: work-email
//	type: email
//	data:
//	  key: ENC[AES256:base64...]
func marshalCredentialYAML(id, name, typ string, data map[string]string) string {
	var b strings.Builder
	b.WriteString("id: " + id + "\n")
	b.WriteString("name: " + name + "\n")
	b.WriteString("type: " + typ + "\n")
	b.WriteString("data:\n")

	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		b.WriteString("  " + k + ": " + data[k] + "\n")
	}
	return b.String()
}

// parseCredentialYAML parses the simple YAML format used for credentials.
func parseCredentialYAML(data []byte) (*Credential, error) {
	cred := &Credential{Data: make(map[string]string)}
	inData := false

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}

		if inData {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
				inData = false
			} else {
				idx := strings.Index(trimmed, ": ")
				if idx < 0 {
					if strings.HasSuffix(trimmed, ":") {
						cred.Data[strings.TrimSuffix(trimmed, ":")] = ""
					}
					continue
				}
				key := trimmed[:idx]
				value := trimmed[idx+2:]
				cred.Data[key] = value
				continue
			}
		}

		if strings.HasPrefix(line, "id: ") {
			cred.ID = strings.TrimPrefix(line, "id: ")
		} else if strings.HasPrefix(line, "name: ") {
			cred.Name = strings.TrimPrefix(line, "name: ")
		} else if strings.HasPrefix(line, "type: ") {
			cred.Type = strings.TrimPrefix(line, "type: ")
		} else if strings.TrimSpace(line) == "data:" {
			inData = true
		}
	}

	if cred.Name == "" && cred.ID == "" {
		return nil, fmt.Errorf("credential: missing name and id fields")
	}

	return cred, nil
}
