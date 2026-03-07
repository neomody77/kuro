// Package gitstore manages Git operations for versioned configuration storage.
package gitstore

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Commit represents a single git commit.
type Commit struct {
	Hash    string
	Message string
	Time    time.Time
}

// Store wraps a git repository path and provides operations on it.
type Store struct {
	path string
}

// Init initializes a git repository at the given path if it doesn't already exist.
// Returns a Store for the repository.
func Init(path string) (*Store, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("gitstore: resolve path: %w", err)
	}

	s := &Store{path: absPath}

	// Check if already a git repo
	if _, err := os.Stat(filepath.Join(absPath, ".git")); err == nil {
		return s, nil
	}

	if err := os.MkdirAll(absPath, 0o755); err != nil {
		return nil, fmt.Errorf("gitstore: mkdir: %w", err)
	}

	if _, err := s.run("init"); err != nil {
		return nil, fmt.Errorf("gitstore: init: %w", err)
	}

	// Configure author for this repo
	if _, err := s.run("config", "user.name", "kuro"); err != nil {
		return nil, fmt.Errorf("gitstore: config user.name: %w", err)
	}
	if _, err := s.run("config", "user.email", "kuro@local"); err != nil {
		return nil, fmt.Errorf("gitstore: config user.email: %w", err)
	}

	return s, nil
}

// Open opens an existing git repository. Returns an error if the path is not a git repo.
func Open(path string) (*Store, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("gitstore: resolve path: %w", err)
	}

	if _, err := os.Stat(filepath.Join(absPath, ".git")); err != nil {
		return nil, fmt.Errorf("gitstore: not a git repository: %s", absPath)
	}

	return &Store{path: absPath}, nil
}

// Path returns the repository path.
func (s *Store) Path() string {
	return s.path
}

// Add stages the given files for commit.
func (s *Store) Add(files ...string) error {
	args := append([]string{"add", "--"}, files...)
	if _, err := s.run(args...); err != nil {
		return fmt.Errorf("gitstore: add: %w", err)
	}
	return nil
}

// Commit creates a new commit with the given message.
func (s *Store) Commit(message string) error {
	_, err := s.run("commit", "-m", message, "--author", "kuro <kuro@local>", "--allow-empty")
	if err != nil {
		return fmt.Errorf("gitstore: commit: %w", err)
	}
	return nil
}

// Log returns the last n commits.
func (s *Store) Log(n int) ([]Commit, error) {
	format := "%H%n%s%n%aI%n---"
	out, err := s.run("log", fmt.Sprintf("-%d", n), fmt.Sprintf("--format=%s", format))
	if err != nil {
		// No commits yet
		if strings.Contains(err.Error(), "does not have any commits") ||
			strings.Contains(err.Error(), "尚无任何提交") {
			return nil, nil
		}
		return nil, fmt.Errorf("gitstore: log: %w", err)
	}

	out = strings.TrimSpace(out)
	if out == "" {
		return nil, nil
	}

	blocks := strings.Split(out, "---")
	var commits []Commit
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		lines := strings.SplitN(block, "\n", 3)
		if len(lines) < 3 {
			continue
		}
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(lines[2]))
		if err != nil {
			t = time.Time{}
		}
		commits = append(commits, Commit{
			Hash:    strings.TrimSpace(lines[0]),
			Message: strings.TrimSpace(lines[1]),
			Time:    t,
		})
	}
	return commits, nil
}

// Revert reverts the commit with the given hash.
func (s *Store) Revert(hash string) error {
	_, err := s.run("revert", "--no-edit", hash)
	if err != nil {
		return fmt.Errorf("gitstore: revert %s: %w", hash, err)
	}
	return nil
}

// Diff returns the diff for a specific commit.
func (s *Store) Diff(hash string) (string, error) {
	out, err := s.run("diff-tree", "--root", "-p", hash)
	if err != nil {
		return "", fmt.Errorf("gitstore: diff %s: %w", hash, err)
	}
	return out, nil
}

// Status returns the short status of the working tree.
func (s *Store) Status() (string, error) {
	out, err := s.run("status", "--short")
	if err != nil {
		return "", fmt.Errorf("gitstore: status: %w", err)
	}
	return out, nil
}

// run executes a git command in the repository directory and returns stdout.
func (s *Store) run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = s.path

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
