package gitstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func initTestRepo(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Init(dir)
	if err != nil {
		t.Fatalf("Init(%s): %v", dir, err)
	}
	return s
}

// writeFile is a helper to create a file in the repo.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	full := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestInit_CreatesGitDir(t *testing.T) {
	dir := t.TempDir()
	s, err := Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	gitDir := filepath.Join(s.Path(), ".git")
	info, err := os.Stat(gitDir)
	if err != nil {
		t.Fatalf(".git dir not found: %v", err)
	}
	if !info.IsDir() {
		t.Fatal(".git is not a directory")
	}
}

func TestInit_IdempotentOnExistingRepo(t *testing.T) {
	dir := t.TempDir()
	s1, err := Init(dir)
	if err != nil {
		t.Fatalf("first Init: %v", err)
	}
	s2, err := Init(dir)
	if err != nil {
		t.Fatalf("second Init: %v", err)
	}
	if s1.Path() != s2.Path() {
		t.Fatalf("paths differ: %s vs %s", s1.Path(), s2.Path())
	}
}

func TestAddCommit_AppearsInLog(t *testing.T) {
	s := initTestRepo(t)

	writeFile(t, s.Path(), "hello.txt", "hello world")
	if err := s.Add("hello.txt"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := s.Commit("add hello"); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	commits, err := s.Log(10)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}
	if commits[0].Message != "add hello" {
		t.Errorf("message = %q, want %q", commits[0].Message, "add hello")
	}
}

func TestLog_ReturnsHashMessageTime(t *testing.T) {
	s := initTestRepo(t)

	writeFile(t, s.Path(), "a.txt", "aaa")
	if err := s.Add("a.txt"); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit("first"); err != nil {
		t.Fatal(err)
	}

	writeFile(t, s.Path(), "b.txt", "bbb")
	if err := s.Add("b.txt"); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit("second"); err != nil {
		t.Fatal(err)
	}

	commits, err := s.Log(10)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits))
	}

	// Most recent first
	if commits[0].Message != "second" {
		t.Errorf("commits[0].Message = %q, want %q", commits[0].Message, "second")
	}
	if commits[1].Message != "first" {
		t.Errorf("commits[1].Message = %q, want %q", commits[1].Message, "first")
	}

	for i, c := range commits {
		if c.Hash == "" {
			t.Errorf("commits[%d].Hash is empty", i)
		}
		if len(c.Hash) != 40 {
			t.Errorf("commits[%d].Hash length = %d, want 40", i, len(c.Hash))
		}
		if c.Time.IsZero() {
			t.Errorf("commits[%d].Time is zero", i)
		}
	}
}

func TestLog_LimitsResults(t *testing.T) {
	s := initTestRepo(t)

	for i := 0; i < 5; i++ {
		writeFile(t, s.Path(), "f.txt", strings.Repeat("x", i+1))
		if err := s.Add("f.txt"); err != nil {
			t.Fatal(err)
		}
		if err := s.Commit("commit"); err != nil {
			t.Fatal(err)
		}
	}

	commits, err := s.Log(3)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(commits) != 3 {
		t.Fatalf("expected 3 commits, got %d", len(commits))
	}
}

func TestLog_EmptyRepo(t *testing.T) {
	s := initTestRepo(t)

	commits, err := s.Log(10)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(commits) != 0 {
		t.Fatalf("expected 0 commits, got %d", len(commits))
	}
}

func TestRevert_UndoesCommit(t *testing.T) {
	s := initTestRepo(t)

	// Create initial file
	writeFile(t, s.Path(), "data.txt", "original")
	if err := s.Add("data.txt"); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit("initial"); err != nil {
		t.Fatal(err)
	}

	// Modify the file
	writeFile(t, s.Path(), "data.txt", "modified")
	if err := s.Add("data.txt"); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit("modify data"); err != nil {
		t.Fatal(err)
	}

	// Get the hash of the modify commit
	commits, err := s.Log(2)
	if err != nil {
		t.Fatal(err)
	}
	modifyHash := commits[0].Hash

	// Revert the modify commit
	if err := s.Revert(modifyHash); err != nil {
		t.Fatalf("Revert: %v", err)
	}

	// File should be restored to original content
	content, err := os.ReadFile(filepath.Join(s.Path(), "data.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "original" {
		t.Errorf("content = %q, want %q", string(content), "original")
	}

	// There should now be 3 commits (initial, modify, revert)
	commits, err = s.Log(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 3 {
		t.Fatalf("expected 3 commits after revert, got %d", len(commits))
	}
}

func TestDiff_ReturnsPatch(t *testing.T) {
	s := initTestRepo(t)

	writeFile(t, s.Path(), "file.txt", "line1\n")
	if err := s.Add("file.txt"); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit("add file"); err != nil {
		t.Fatal(err)
	}

	commits, err := s.Log(1)
	if err != nil {
		t.Fatal(err)
	}

	diff, err := s.Diff(commits[0].Hash)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}

	if !strings.Contains(diff, "file.txt") {
		t.Errorf("diff does not mention file.txt:\n%s", diff)
	}
	if !strings.Contains(diff, "+line1") {
		t.Errorf("diff does not contain added line:\n%s", diff)
	}
}

func TestStatus_DetectsModified(t *testing.T) {
	s := initTestRepo(t)

	// Create and commit a file
	writeFile(t, s.Path(), "tracked.txt", "original")
	if err := s.Add("tracked.txt"); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit("add tracked"); err != nil {
		t.Fatal(err)
	}

	// Modify the file
	writeFile(t, s.Path(), "tracked.txt", "changed")

	status, err := s.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !strings.Contains(status, "tracked.txt") {
		t.Errorf("status does not show tracked.txt as modified:\n%q", status)
	}
	if !strings.Contains(status, "M") {
		t.Errorf("status does not show M flag:\n%q", status)
	}
}

func TestStatus_DetectsUntracked(t *testing.T) {
	s := initTestRepo(t)

	// Need at least one commit for status to work properly
	writeFile(t, s.Path(), "initial.txt", "init")
	if err := s.Add("initial.txt"); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit("init"); err != nil {
		t.Fatal(err)
	}

	// Create an untracked file
	writeFile(t, s.Path(), "untracked.txt", "new file")

	status, err := s.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !strings.Contains(status, "untracked.txt") {
		t.Errorf("status does not show untracked.txt:\n%q", status)
	}
	if !strings.Contains(status, "??") {
		t.Errorf("status does not show ?? flag:\n%q", status)
	}
}

func TestStatus_CleanRepo(t *testing.T) {
	s := initTestRepo(t)

	writeFile(t, s.Path(), "clean.txt", "data")
	if err := s.Add("clean.txt"); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit("add clean"); err != nil {
		t.Fatal(err)
	}

	status, err := s.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if strings.TrimSpace(status) != "" {
		t.Errorf("expected empty status for clean repo, got %q", status)
	}
}

func TestOpen_ExistingRepo(t *testing.T) {
	s := initTestRepo(t)

	s2, err := Open(s.Path())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if s2.Path() != s.Path() {
		t.Errorf("paths differ: %s vs %s", s2.Path(), s.Path())
	}
}

func TestOpen_NonRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := Open(dir)
	if err == nil {
		t.Fatal("Open on non-repo should return error")
	}
}
