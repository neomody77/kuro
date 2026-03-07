// Package document implements the document store for notes, templates, and knowledge base.
package document

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neomody77/kuro/internal/gitstore"
)

// Doc represents a document with metadata.
type Doc struct {
	Path       string    `json:"path"`
	Content    string    `json:"content,omitempty"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modified_at"`
	IsDir      bool      `json:"is_dir"`
}

// Store manages markdown documents under a user's repo/documents/ directory.
type Store struct {
	dir string
	git *gitstore.Store
}

// NewStore creates a document store rooted at the given directory.
// git may be nil if git versioning is not needed.
func NewStore(dir string, git *gitstore.Store) *Store {
	return &Store{dir: dir, git: git}
}

// List returns all documents and subdirectories under the given relative path.
func (s *Store) List(relPath string) ([]Doc, error) {
	absPath, err := s.resolve(relPath)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("document: list %q: %w", relPath, err)
	}

	var docs []Doc
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		entryPath := relPath
		if entryPath != "" {
			entryPath += "/"
		}
		entryPath += entry.Name()

		docs = append(docs, Doc{
			Path:       entryPath,
			Size:       info.Size(),
			ModifiedAt: info.ModTime(),
			IsDir:      entry.IsDir(),
		})
	}
	return docs, nil
}

// Get reads a document at the given relative path.
func (s *Store) Get(relPath string) (*Doc, error) {
	absPath, err := s.resolve(relPath)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("document: get %q: %w", relPath, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("document: %q is a directory", relPath)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("document: read %q: %w", relPath, err)
	}

	return &Doc{
		Path:       relPath,
		Content:    string(data),
		Size:       info.Size(),
		ModifiedAt: info.ModTime(),
	}, nil
}

// Put creates or updates a document at the given relative path, then commits to git.
func (s *Store) Put(relPath string, content string) error {
	absPath, err := s.resolve(relPath)
	if err != nil {
		return err
	}

	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("document: mkdir %q: %w", dir, err)
	}

	if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("document: write %q: %w", relPath, err)
	}

	if s.git != nil {
		gitRel, err := filepath.Rel(s.git.Path(), absPath)
		if err != nil {
			gitRel = absPath
		}
		if err := s.git.Add(gitRel); err != nil {
			return fmt.Errorf("document: git add: %w", err)
		}
		if err := s.git.Commit(fmt.Sprintf("document: save %s", relPath)); err != nil {
			return fmt.Errorf("document: git commit: %w", err)
		}
	}
	return nil
}

// Delete removes a document at the given relative path and commits to git.
func (s *Store) Delete(relPath string) error {
	absPath, err := s.resolve(relPath)
	if err != nil {
		return err
	}

	err = os.Remove(absPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("document: delete %q: %w", relPath, err)
	}
	removed := err == nil

	if removed && s.git != nil {
		gitRel, err := filepath.Rel(s.git.Path(), absPath)
		if err != nil {
			gitRel = absPath
		}
		if err := s.git.Add(gitRel); err != nil {
			return fmt.Errorf("document: git add: %w", err)
		}
		if err := s.git.Commit(fmt.Sprintf("document: delete %s", relPath)); err != nil {
			return fmt.Errorf("document: git commit: %w", err)
		}
	}
	return nil
}

// Rename moves a document from oldPath to newPath.
func (s *Store) Rename(oldPath, newPath string) error {
	absOld, err := s.resolve(oldPath)
	if err != nil {
		return err
	}
	absNew, err := s.resolve(newPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(absNew), 0o755); err != nil {
		return fmt.Errorf("document: rename mkdir: %w", err)
	}
	if err := os.Rename(absOld, absNew); err != nil {
		return fmt.Errorf("document: rename: %w", err)
	}
	if s.git != nil {
		for _, abs := range []string{absOld, absNew} {
			gitRel, err := filepath.Rel(s.git.Path(), abs)
			if err != nil {
				gitRel = abs
			}
			_ = s.git.Add(gitRel)
		}
		_ = s.git.Commit(fmt.Sprintf("document: rename %s -> %s", oldPath, newPath))
	}
	return nil
}

// Search returns documents whose content contains the query string.
func (s *Store) Search(query string) ([]Doc, error) {
	var results []Doc
	err := filepath.WalkDir(s.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		if strings.Contains(string(data), query) {
			info, _ := d.Info()
			rel, _ := filepath.Rel(s.dir, path)
			results = append(results, Doc{
				Path:       rel,
				Size:       info.Size(),
				ModifiedAt: info.ModTime(),
			})
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("document: search: %w", err)
	}
	return results, nil
}

// resolve converts a relative document path to an absolute path,
// ensuring it stays within the store directory.
func (s *Store) resolve(relPath string) (string, error) {
	cleaned := filepath.Clean(relPath)
	abs := filepath.Join(s.dir, cleaned)
	// Ensure the resolved path is still under the store directory.
	// filepath.Rel will produce a path starting with ".." if abs escapes.
	rel, err := filepath.Rel(s.dir, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("document: path %q escapes store directory", relPath)
	}
	return abs, nil
}
