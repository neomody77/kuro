package document

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neomody77/kuro/internal/gitstore"
)

// setup creates a temp git repo with a documents/ subdirectory and returns
// a document Store plus the repo root path.
func setup(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	gs, err := gitstore.Init(dir)
	if err != nil {
		t.Fatalf("gitstore.Init: %v", err)
	}
	docDir := filepath.Join(dir, "documents")
	if err := os.MkdirAll(docDir, 0o755); err != nil {
		t.Fatalf("mkdir documents: %v", err)
	}
	return NewStore(docDir, gs), dir
}

func TestList_NestedFolders(t *testing.T) {
	store, _ := setup(t)

	// Create nested structure: notes/work/meeting.md, notes/personal.md, readme.md
	if err := store.Put("readme.md", "# Root readme"); err != nil {
		t.Fatal(err)
	}
	if err := store.Put("notes/personal.md", "personal stuff"); err != nil {
		t.Fatal(err)
	}
	if err := store.Put("notes/work/meeting.md", "meeting notes"); err != nil {
		t.Fatal(err)
	}

	// List root
	docs, err := store.List("")
	if err != nil {
		t.Fatalf("List root: %v", err)
	}

	// Should have notes/ dir and readme.md
	if len(docs) != 2 {
		t.Fatalf("expected 2 entries at root, got %d: %+v", len(docs), docs)
	}

	var foundDir, foundFile bool
	for _, d := range docs {
		if d.Path == "notes" && d.IsDir {
			foundDir = true
		}
		if d.Path == "readme.md" && !d.IsDir {
			foundFile = true
		}
	}
	if !foundDir {
		t.Error("expected notes/ directory in listing")
	}
	if !foundFile {
		t.Error("expected readme.md in listing")
	}

	// List nested folder
	docs, err = store.List("notes")
	if err != nil {
		t.Fatalf("List notes: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 entries in notes/, got %d: %+v", len(docs), docs)
	}

	var foundWork, foundPersonal bool
	for _, d := range docs {
		if d.Path == "notes/work" && d.IsDir {
			foundWork = true
		}
		if d.Path == "notes/personal.md" && !d.IsDir {
			foundPersonal = true
		}
	}
	if !foundWork {
		t.Error("expected notes/work/ directory")
	}
	if !foundPersonal {
		t.Error("expected notes/personal.md")
	}
}

func TestList_EmptyDir(t *testing.T) {
	store, _ := setup(t)

	docs, err := store.List("")
	if err != nil {
		t.Fatalf("List empty: %v", err)
	}
	if len(docs) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(docs))
	}
}

func TestList_NonexistentDir(t *testing.T) {
	store, _ := setup(t)

	docs, err := store.List("nonexistent")
	if err != nil {
		t.Fatalf("List nonexistent: %v", err)
	}
	if docs != nil {
		t.Fatalf("expected nil, got %v", docs)
	}
}

func TestGet_ReadsContent(t *testing.T) {
	store, _ := setup(t)

	content := "# Hello World\n\nThis is a test document."
	if err := store.Put("test.md", content); err != nil {
		t.Fatal(err)
	}

	doc, err := store.Get("test.md")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if doc.Content != content {
		t.Errorf("content = %q, want %q", doc.Content, content)
	}
	if doc.Path != "test.md" {
		t.Errorf("path = %q, want %q", doc.Path, "test.md")
	}
	if doc.Size != int64(len(content)) {
		t.Errorf("size = %d, want %d", doc.Size, len(content))
	}
}

func TestGet_NestedFile(t *testing.T) {
	store, _ := setup(t)

	if err := store.Put("deep/nested/file.md", "deep content"); err != nil {
		t.Fatal(err)
	}

	doc, err := store.Get("deep/nested/file.md")
	if err != nil {
		t.Fatalf("Get nested: %v", err)
	}
	if doc.Content != "deep content" {
		t.Errorf("content = %q, want %q", doc.Content, "deep content")
	}
}

func TestGet_NonexistentFile(t *testing.T) {
	store, _ := setup(t)

	_, err := store.Get("nonexistent.md")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestGet_Directory(t *testing.T) {
	store, _ := setup(t)

	if err := store.Put("dir/file.md", "content"); err != nil {
		t.Fatal(err)
	}

	_, err := store.Get("dir")
	if err == nil {
		t.Fatal("expected error when getting a directory")
	}
}

func TestPut_WritesFileAndCommits(t *testing.T) {
	store, repoDir := setup(t)

	if err := store.Put("new.md", "new content"); err != nil {
		t.Fatalf("Put: %v", err)
	}

	// Verify file exists on disk
	data, err := os.ReadFile(filepath.Join(repoDir, "documents", "new.md"))
	if err != nil {
		t.Fatalf("file not on disk: %v", err)
	}
	if string(data) != "new content" {
		t.Errorf("file content = %q, want %q", string(data), "new content")
	}

	// Verify git commit exists
	gs, err := gitstore.Open(repoDir)
	if err != nil {
		t.Fatalf("Open git: %v", err)
	}
	commits, err := gs.Log(10)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(commits) == 0 {
		t.Fatal("expected at least 1 commit after Put")
	}
	if !strings.Contains(commits[0].Message, "save") {
		t.Errorf("commit message = %q, want it to contain 'save'", commits[0].Message)
	}
}

func TestPut_CreatesNestedDirectories(t *testing.T) {
	store, repoDir := setup(t)

	if err := store.Put("a/b/c/deep.md", "deep"); err != nil {
		t.Fatalf("Put nested: %v", err)
	}

	// Verify the nested directory structure was created
	info, err := os.Stat(filepath.Join(repoDir, "documents", "a", "b", "c"))
	if err != nil {
		t.Fatalf("nested dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected directory")
	}
}

func TestPut_OverwritesExisting(t *testing.T) {
	store, _ := setup(t)

	if err := store.Put("file.md", "v1"); err != nil {
		t.Fatal(err)
	}
	if err := store.Put("file.md", "v2"); err != nil {
		t.Fatal(err)
	}

	doc, err := store.Get("file.md")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Content != "v2" {
		t.Errorf("content = %q, want %q", doc.Content, "v2")
	}
}

func TestDelete_RemovesFileAndCommits(t *testing.T) {
	store, repoDir := setup(t)

	if err := store.Put("todelete.md", "will be deleted"); err != nil {
		t.Fatal(err)
	}

	if err := store.Delete("todelete.md"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify file is gone
	_, err := os.Stat(filepath.Join(repoDir, "documents", "todelete.md"))
	if !os.IsNotExist(err) {
		t.Fatal("expected file to be removed")
	}

	// Verify commit exists for deletion
	gs, err := gitstore.Open(repoDir)
	if err != nil {
		t.Fatalf("Open git: %v", err)
	}
	commits, err := gs.Log(10)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(commits) < 2 {
		t.Fatal("expected at least 2 commits (put + delete)")
	}
	if !strings.Contains(commits[0].Message, "delete") {
		t.Errorf("commit message = %q, want it to contain 'delete'", commits[0].Message)
	}
}

func TestDelete_NonexistentFileNoError(t *testing.T) {
	store, _ := setup(t)

	// Deleting a non-existent file should not error (os.Remove returns
	// IsNotExist which is swallowed).
	err := store.Delete("ghost.md")
	if err != nil {
		t.Fatalf("Delete nonexistent: %v", err)
	}
}

func TestSearch_FindsByKeyword(t *testing.T) {
	store, _ := setup(t)

	if err := store.Put("alpha.md", "the quick brown fox"); err != nil {
		t.Fatal(err)
	}
	if err := store.Put("beta.md", "lazy dog jumped"); err != nil {
		t.Fatal(err)
	}
	if err := store.Put("gamma.md", "the fox is quick"); err != nil {
		t.Fatal(err)
	}

	results, err := store.Search("fox")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d: %+v", len(results), results)
	}

	paths := map[string]bool{}
	for _, r := range results {
		paths[r.Path] = true
	}
	if !paths["alpha.md"] {
		t.Error("expected alpha.md in results")
	}
	if !paths["gamma.md"] {
		t.Error("expected gamma.md in results")
	}
}

func TestSearch_NoResults(t *testing.T) {
	store, _ := setup(t)

	if err := store.Put("doc.md", "nothing special here"); err != nil {
		t.Fatal(err)
	}

	results, err := store.Search("nonexistent-keyword-xyz")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestSearch_NestedFiles(t *testing.T) {
	store, _ := setup(t)

	if err := store.Put("top.md", "findme here"); err != nil {
		t.Fatal(err)
	}
	if err := store.Put("sub/deep.md", "also findme"); err != nil {
		t.Fatal(err)
	}

	results, err := store.Search("findme")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestPathTraversal_Rejected(t *testing.T) {
	store, _ := setup(t)

	traversalPaths := []string{
		"../etc/passwd",
		"../../etc/shadow",
		"foo/../../etc/passwd",
		"../../../tmp/evil",
	}

	for _, p := range traversalPaths {
		t.Run("Put_"+p, func(t *testing.T) {
			err := store.Put(p, "evil")
			if err == nil {
				t.Errorf("Put(%q) should be rejected", p)
			}
		})

		t.Run("Get_"+p, func(t *testing.T) {
			_, err := store.Get(p)
			if err == nil {
				t.Errorf("Get(%q) should be rejected", p)
			}
		})

		t.Run("Delete_"+p, func(t *testing.T) {
			err := store.Delete(p)
			if err == nil {
				t.Errorf("Delete(%q) should be rejected", p)
			}
		})

		t.Run("List_"+p, func(t *testing.T) {
			_, err := store.List(p)
			if err == nil {
				t.Errorf("List(%q) should be rejected", p)
			}
		})
	}
}

func TestPathTraversal_ValidPathsAllowed(t *testing.T) {
	store, _ := setup(t)

	validPaths := []string{
		"normal.md",
		"sub/file.md",
		"a/b/c.md",
	}

	for _, p := range validPaths {
		if err := store.Put(p, "ok"); err != nil {
			t.Errorf("Put(%q) should succeed: %v", p, err)
		}
	}
}

func TestPut_WithoutGit(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir, nil)

	if err := store.Put("nogit.md", "content"); err != nil {
		t.Fatalf("Put without git: %v", err)
	}

	doc, err := store.Get("nogit.md")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if doc.Content != "content" {
		t.Errorf("content = %q, want %q", doc.Content, "content")
	}
}
