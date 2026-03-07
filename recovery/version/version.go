// Package version manages multiple engine versions and their lifecycle.
package version

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"
)

type Status string

const (
	StatusRunning Status = "running"
	StatusStopped Status = "stopped"
)

type Version struct {
	ID      string     `json:"id"`
	Path    string     `json:"path"`
	Port    int        `json:"port"`
	Status  Status     `json:"status"`
	Process *os.Process `json:"-"`
}

type Manager struct {
	mu       sync.RWMutex
	baseDir  string // ~/.kuro/versions
	versions map[string]*Version
	nextPort int
}

func NewManager(baseDir string) *Manager {
	return &Manager{
		baseDir:  baseDir,
		versions: make(map[string]*Version),
		nextPort: 9001,
	}
}

// Scan discovers existing versions from the versions/ directory.
func (m *Manager) Scan() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := os.MkdirAll(m.baseDir, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(m.baseDir)
	if err != nil {
		return err
	}

	for _, e := range entries {
		name := e.Name()
		if name == "current" {
			continue
		}
		if !e.IsDir() {
			continue
		}
		if _, exists := m.versions[name]; exists {
			continue
		}
		m.versions[name] = &Version{
			ID:     name,
			Path:   filepath.Join(m.baseDir, name),
			Status: StatusStopped,
		}
	}
	return nil
}

// List returns all known versions, sorted by ID.
func (m *Manager) List() []Version {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Version, 0, len(m.versions))
	for _, v := range m.versions {
		result = append(result, *v)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID > result[j].ID // newest first
	})
	return result
}

// Get returns a single version by ID.
func (m *Manager) Get(id string) (*Version, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.versions[id]
	if !ok {
		return nil, false
	}
	cp := *v
	return &cp, true
}

// allocatePort finds the next available port.
func (m *Manager) allocatePort() int {
	used := make(map[int]bool)
	for _, v := range m.versions {
		if v.Port > 0 {
			used[v.Port] = true
		}
	}
	port := 9001
	for used[port] {
		port++
	}
	return port
}

// Start launches an engine binary for the given version.
func (m *Manager) Start(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	v, ok := m.versions[id]
	if !ok {
		return fmt.Errorf("version %s not found", id)
	}
	if v.Status == StatusRunning {
		return fmt.Errorf("version %s is already running", id)
	}

	binaryPath := filepath.Join(v.Path, "kuro")
	if _, err := os.Stat(binaryPath); err != nil {
		return fmt.Errorf("engine binary not found at %s", binaryPath)
	}

	port := m.allocatePort()

	cmd := exec.Command(binaryPath)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", port))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start version %s: %w", id, err)
	}

	v.Port = port
	v.Process = cmd.Process
	v.Status = StatusRunning

	// Monitor the process in background
	go func() {
		_ = cmd.Wait()
		m.mu.Lock()
		v.Status = StatusStopped
		v.Process = nil
		v.Port = 0
		m.mu.Unlock()
	}()

	return nil
}

// Stop gracefully stops an engine (SIGTERM, then SIGKILL after 10s).
func (m *Manager) Stop(id string) error {
	m.mu.Lock()
	v, ok := m.versions[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("version %s not found", id)
	}
	if v.Status != StatusRunning || v.Process == nil {
		m.mu.Unlock()
		return fmt.Errorf("version %s is not running", id)
	}
	proc := v.Process
	m.mu.Unlock()

	// Send SIGTERM
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM: %w", err)
	}

	// Wait up to 10 seconds for graceful exit
	done := make(chan struct{})
	go func() {
		proc.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Exited gracefully
	case <-time.After(10 * time.Second):
		proc.Signal(syscall.SIGKILL)
		<-done
	}

	m.mu.Lock()
	v.Status = StatusStopped
	v.Process = nil
	v.Port = 0
	m.mu.Unlock()

	return nil
}

// Current returns the version ID that the "current" symlink points to.
func (m *Manager) Current() string {
	link := filepath.Join(m.baseDir, "current")
	target, err := os.Readlink(link)
	if err != nil {
		return ""
	}
	return filepath.Base(target)
}

// SetDefault updates the "current" symlink to point to the given version.
func (m *Manager) SetDefault(id string) error {
	m.mu.RLock()
	_, ok := m.versions[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("version %s not found", id)
	}

	link := filepath.Join(m.baseDir, "current")
	// Remove existing symlink
	os.Remove(link)
	return os.Symlink(id, link)
}

// PortForVersion returns the port a running version is on, or 0 if not running.
func (m *Manager) PortForVersion(id string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.versions[id]
	if !ok || v.Status != StatusRunning {
		return 0
	}
	return v.Port
}

// DefaultPort returns the port for the current/default version.
func (m *Manager) DefaultPort() int {
	cur := m.Current()
	if cur == "" {
		return 0
	}
	return m.PortForVersion(cur)
}

// Delete stops a version if running and removes its directory.
func (m *Manager) Delete(id string) error {
	m.mu.RLock()
	v, ok := m.versions[id]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("version %s not found", id)
	}

	if v.Status == StatusRunning {
		if err := m.Stop(id); err != nil {
			return err
		}
	}

	// Don't allow deleting current version
	if m.Current() == id {
		return fmt.Errorf("cannot delete current version %s; set a different default first", id)
	}

	m.mu.Lock()
	delete(m.versions, id)
	m.mu.Unlock()

	return os.RemoveAll(filepath.Join(m.baseDir, id))
}

// Install copies engine binary and UI assets into versions/{id}/.
func (m *Manager) Install(binaryPath, uiPath, id string) error {
	destDir := filepath.Join(m.baseDir, id)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	// Copy binary
	destBinary := filepath.Join(destDir, "kuro")
	if err := copyFile(binaryPath, destBinary); err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}
	if err := os.Chmod(destBinary, 0755); err != nil {
		return err
	}

	// Copy UI directory/file if provided
	if uiPath != "" {
		destUI := filepath.Join(destDir, "ui")
		if err := os.MkdirAll(destUI, 0755); err != nil {
			return err
		}
		if err := copyFile(uiPath, filepath.Join(destUI, filepath.Base(uiPath))); err != nil {
			return fmt.Errorf("failed to copy UI assets: %w", err)
		}
	}

	m.mu.Lock()
	m.versions[id] = &Version{
		ID:     id,
		Path:   destDir,
		Status: StatusStopped,
	}
	m.mu.Unlock()

	return nil
}

// StopAll stops all running versions.
func (m *Manager) StopAll() {
	m.mu.RLock()
	var running []string
	for id, v := range m.versions {
		if v.Status == StatusRunning {
			running = append(running, id)
		}
	}
	m.mu.RUnlock()

	for _, id := range running {
		m.Stop(id)
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
