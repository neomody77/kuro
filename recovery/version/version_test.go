package version

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScan_DiscoversVersionDirs(t *testing.T) {
	dir := t.TempDir()

	// Create version directories
	os.MkdirAll(filepath.Join(dir, "0.1.0"), 0755)
	os.MkdirAll(filepath.Join(dir, "0.2.0"), 0755)
	os.MkdirAll(filepath.Join(dir, "0.3.0"), 0755)

	// Create "current" symlink — should be skipped
	os.Symlink("0.2.0", filepath.Join(dir, "current"))

	// Create a regular file — should be skipped
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hi"), 0644)

	m := NewManager(dir)
	if err := m.Scan(); err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	versions := m.List()
	if len(versions) != 3 {
		t.Fatalf("expected 3 versions, got %d", len(versions))
	}

	// List returns newest first (descending sort)
	if versions[0].ID != "0.3.0" {
		t.Errorf("expected first version 0.3.0, got %s", versions[0].ID)
	}
	if versions[2].ID != "0.1.0" {
		t.Errorf("expected last version 0.1.0, got %s", versions[2].ID)
	}

	// All should be stopped
	for _, v := range versions {
		if v.Status != StatusStopped {
			t.Errorf("version %s should be stopped, got %s", v.ID, v.Status)
		}
	}
}

func TestScan_CreatesBaseDirIfNotExists(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nonexistent", "versions")

	m := NewManager(dir)
	if err := m.Scan(); err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Fatal("expected base dir to be created")
	}

	if len(m.List()) != 0 {
		t.Error("expected 0 versions in empty dir")
	}
}

func TestScan_IdempotentDoesNotDuplicate(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "0.1.0"), 0755)

	m := NewManager(dir)
	m.Scan()
	m.Scan() // second scan should not duplicate

	if len(m.List()) != 1 {
		t.Fatalf("expected 1 version after double scan, got %d", len(m.List()))
	}
}

func TestGet(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "1.0.0"), 0755)

	m := NewManager(dir)
	m.Scan()

	v, ok := m.Get("1.0.0")
	if !ok {
		t.Fatal("expected to find version 1.0.0")
	}
	if v.ID != "1.0.0" {
		t.Errorf("expected ID 1.0.0, got %s", v.ID)
	}

	_, ok = m.Get("nope")
	if ok {
		t.Error("expected not to find nonexistent version")
	}
}

func TestSetDefault_CreatesSymlink(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "0.1.0"), 0755)
	os.MkdirAll(filepath.Join(dir, "0.2.0"), 0755)

	m := NewManager(dir)
	m.Scan()

	// No default initially
	if cur := m.Current(); cur != "" {
		t.Errorf("expected no current, got %s", cur)
	}

	// Set default
	if err := m.SetDefault("0.1.0"); err != nil {
		t.Fatalf("SetDefault() error: %v", err)
	}
	if cur := m.Current(); cur != "0.1.0" {
		t.Errorf("expected current 0.1.0, got %s", cur)
	}

	// Change default
	if err := m.SetDefault("0.2.0"); err != nil {
		t.Fatalf("SetDefault() error: %v", err)
	}
	if cur := m.Current(); cur != "0.2.0" {
		t.Errorf("expected current 0.2.0, got %s", cur)
	}

	// Set default to nonexistent version
	if err := m.SetDefault("nope"); err == nil {
		t.Error("expected error for nonexistent version")
	}
}

func TestInstall(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	// Create a fake binary to install
	srcDir := t.TempDir()
	binaryPath := filepath.Join(srcDir, "kuro")
	os.WriteFile(binaryPath, []byte("#!/bin/sh\necho ok"), 0755)

	// Install without UI assets
	if err := m.Install(binaryPath, "", "1.0.0"); err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	// Check version was registered
	v, ok := m.Get("1.0.0")
	if !ok {
		t.Fatal("expected version 1.0.0 after install")
	}
	if v.Status != StatusStopped {
		t.Errorf("expected stopped, got %s", v.Status)
	}

	// Check binary was copied
	destBinary := filepath.Join(dir, "1.0.0", "kuro")
	info, err := os.Stat(destBinary)
	if err != nil {
		t.Fatalf("binary not found at %s", destBinary)
	}
	if info.Mode()&0111 == 0 {
		t.Error("binary should be executable")
	}
}

func TestInstall_WithUI(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	srcDir := t.TempDir()
	binaryPath := filepath.Join(srcDir, "kuro")
	os.WriteFile(binaryPath, []byte("bin"), 0755)
	uiPath := filepath.Join(srcDir, "index.html")
	os.WriteFile(uiPath, []byte("<html>"), 0644)

	if err := m.Install(binaryPath, uiPath, "2.0.0"); err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	// Check UI file was copied
	destUI := filepath.Join(dir, "2.0.0", "ui", "index.html")
	if _, err := os.Stat(destUI); err != nil {
		t.Fatalf("UI asset not found at %s", destUI)
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "0.1.0"), 0755)
	os.MkdirAll(filepath.Join(dir, "0.2.0"), 0755)

	m := NewManager(dir)
	m.Scan()

	// Delete a non-current version
	if err := m.Delete("0.1.0"); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	// Version should be gone from list
	if _, ok := m.Get("0.1.0"); ok {
		t.Error("expected version 0.1.0 to be removed")
	}

	// Directory should be removed
	if _, err := os.Stat(filepath.Join(dir, "0.1.0")); !os.IsNotExist(err) {
		t.Error("expected directory to be removed")
	}
}

func TestDelete_CurrentVersionBlocked(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "0.1.0"), 0755)

	m := NewManager(dir)
	m.Scan()
	m.SetDefault("0.1.0")

	if err := m.Delete("0.1.0"); err == nil {
		t.Error("expected error deleting current version")
	}
}

func TestDelete_NonexistentVersion(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	if err := m.Delete("nope"); err == nil {
		t.Error("expected error deleting nonexistent version")
	}
}

func TestStart_NoVersionError(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	if err := m.Start("nope"); err == nil {
		t.Error("expected error starting nonexistent version")
	}
}

func TestStart_NoBinaryError(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "0.1.0"), 0755)
	// No binary inside

	m := NewManager(dir)
	m.Scan()

	if err := m.Start("0.1.0"); err == nil {
		t.Error("expected error when binary not found")
	}
}

func TestStop_NotRunningError(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "0.1.0"), 0755)

	m := NewManager(dir)
	m.Scan()

	if err := m.Stop("0.1.0"); err == nil {
		t.Error("expected error stopping non-running version")
	}
}

func TestStop_NonexistentError(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	if err := m.Stop("nope"); err == nil {
		t.Error("expected error stopping nonexistent version")
	}
}

func TestPortForVersion(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "0.1.0"), 0755)

	m := NewManager(dir)
	m.Scan()

	// Not running — should return 0
	if port := m.PortForVersion("0.1.0"); port != 0 {
		t.Errorf("expected port 0 for stopped version, got %d", port)
	}

	// Nonexistent — should return 0
	if port := m.PortForVersion("nope"); port != 0 {
		t.Errorf("expected port 0 for nonexistent version, got %d", port)
	}
}

func TestDefaultPort_NoDefault(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	if port := m.DefaultPort(); port != 0 {
		t.Errorf("expected port 0 when no default set, got %d", port)
	}
}

func TestAllocatePort(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	// With no versions, should allocate 9001
	port := m.allocatePort()
	if port != 9001 {
		t.Errorf("expected port 9001, got %d", port)
	}
}

func TestList_SortOrder(t *testing.T) {
	dir := t.TempDir()
	for _, v := range []string{"a", "c", "b"} {
		os.MkdirAll(filepath.Join(dir, v), 0755)
	}

	m := NewManager(dir)
	m.Scan()

	versions := m.List()
	if len(versions) != 3 {
		t.Fatalf("expected 3 versions, got %d", len(versions))
	}
	// Descending order
	if versions[0].ID != "c" || versions[1].ID != "b" || versions[2].ID != "a" {
		t.Errorf("unexpected order: %s, %s, %s", versions[0].ID, versions[1].ID, versions[2].ID)
	}
}
