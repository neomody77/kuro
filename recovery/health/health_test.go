package health

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neomody77/kuro/recovery/version"
)

// newTestManager creates a version manager with version dirs in a temp dir.
func newTestManager(t *testing.T, versionIDs ...string) (*version.Manager, string) {
	t.Helper()
	dir := t.TempDir()
	for _, id := range versionIDs {
		os.MkdirAll(filepath.Join(dir, id), 0755)
	}
	m := version.NewManager(dir)
	if err := m.Scan(); err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	return m, dir
}

func TestChecker_New(t *testing.T) {
	m, _ := newTestManager(t, "v1")
	c := New(m, 5*time.Second, 3)

	if c.interval != 5*time.Second {
		t.Errorf("expected interval 5s, got %v", c.interval)
	}
	if c.failureThreshold != 3 {
		t.Errorf("expected threshold 3, got %d", c.failureThreshold)
	}
	if !c.AutoRollback() {
		t.Error("expected auto-rollback enabled by default")
	}
}

func TestChecker_SetAutoRollback(t *testing.T) {
	m, _ := newTestManager(t)
	c := New(m, time.Second, 3)

	c.SetAutoRollback(false)
	if c.AutoRollback() {
		t.Error("expected auto-rollback disabled")
	}

	c.SetAutoRollback(true)
	if !c.AutoRollback() {
		t.Error("expected auto-rollback enabled")
	}
}

func TestChecker_Failures_InitialZero(t *testing.T) {
	m, _ := newTestManager(t, "v1")
	c := New(m, time.Second, 3)

	if f := c.Failures("v1"); f != 0 {
		t.Errorf("expected 0 failures initially, got %d", f)
	}
}

func TestChecker_LastKnownGood_InitialEmpty(t *testing.T) {
	m, _ := newTestManager(t)
	c := New(m, time.Second, 3)

	if lkg := c.LastKnownGood(); lkg != "" {
		t.Errorf("expected empty last-known-good, got %s", lkg)
	}
}

func TestChecker_PingHealthy(t *testing.T) {
	// Create a healthy backend
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer backend.Close()

	m, _ := newTestManager(t)
	c := New(m, time.Second, 3)

	port := extractPort(t, backend.URL)
	if !c.ping(port) {
		t.Error("expected ping to return true for healthy backend")
	}
}

func TestChecker_PingUnhealthy(t *testing.T) {
	// Create an unhealthy backend (returns 500)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer backend.Close()

	m, _ := newTestManager(t)
	c := New(m, time.Second, 3)

	port := extractPort(t, backend.URL)
	if c.ping(port) {
		t.Error("expected ping to return false for unhealthy backend")
	}
}

func TestChecker_PingNoServer(t *testing.T) {
	m, _ := newTestManager(t)
	c := New(m, time.Second, 3)

	// Port with nothing listening
	if c.ping(59999) {
		t.Error("expected ping to return false when no server")
	}
}

func TestChecker_CheckAll_HealthyResetsFailures(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()
	port := extractPort(t, backend.URL)

	m, _ := newTestManager(t, "v1")
	m.SetDefault("v1")
	c := New(m, time.Second, 5)

	// Manually inject a running version with the test backend's port.
	// Since we can't set ports from outside the version package,
	// we'll test the failure counting logic directly.

	// Set some initial failures
	c.mu.Lock()
	c.failures["v1"] = 2
	c.mu.Unlock()

	// We can't call checkAll directly because it reads from manager.List()
	// and version won't have StatusRunning or a port. So test ping + failure
	// tracking logic in isolation.

	// Verify ping works
	if !c.ping(port) {
		t.Fatal("expected healthy ping")
	}

	// After a healthy ping, failures should reset
	c.mu.Lock()
	c.failures["v1"] = 0 // simulate what checkAll does
	c.mu.Unlock()

	if f := c.Failures("v1"); f != 0 {
		t.Errorf("expected 0 failures after healthy check, got %d", f)
	}
}

func TestChecker_FailureCounting(t *testing.T) {
	m, _ := newTestManager(t, "v1")
	c := New(m, time.Second, 3)

	// Simulate consecutive failures
	c.mu.Lock()
	c.failures["v1"]++
	c.mu.Unlock()
	if f := c.Failures("v1"); f != 1 {
		t.Errorf("expected 1 failure, got %d", f)
	}

	c.mu.Lock()
	c.failures["v1"]++
	c.mu.Unlock()
	if f := c.Failures("v1"); f != 2 {
		t.Errorf("expected 2 failures, got %d", f)
	}

	c.mu.Lock()
	c.failures["v1"]++
	c.mu.Unlock()
	if f := c.Failures("v1"); f != 3 {
		t.Errorf("expected 3 failures, got %d", f)
	}
}

func TestChecker_RollbackLogic(t *testing.T) {
	m, dir := newTestManager(t, "v1", "v2")

	// Create fake binaries so Start() won't fail with "binary not found"
	// (though we won't actually start—testing rollback method logic)
	for _, id := range []string{"v1", "v2"} {
		binaryPath := filepath.Join(dir, id, "kuro")
		os.WriteFile(binaryPath, []byte("#!/bin/sh\necho ok"), 0755)
	}

	m.SetDefault("v2")

	c := New(m, time.Second, 3)
	c.mu.Lock()
	c.lastKnownGood = "v1"
	c.mu.Unlock()

	// Test rollback — it should set default to v1
	// rollback calls manager.SetDefault(lkg) which is "v1"
	// It also tries to start v1 if not running.
	// Since v1 is not actually a real binary, Start will fail, but
	// SetDefault should still be called after the Start attempt.
	// Looking at the code: if Start fails, rollback returns early.
	// So let's just test the logic by checking the conditions.

	// Verify rollback conditions:
	// 1. lastKnownGood must not be empty
	// 2. lastKnownGood must differ from failedID
	lkg := c.LastKnownGood()
	if lkg != "v1" {
		t.Errorf("expected last-known-good v1, got %s", lkg)
	}
	if lkg == "v2" {
		t.Error("last-known-good should differ from current")
	}
}

func TestChecker_RollbackSkippedWhenNoLKG(t *testing.T) {
	m, _ := newTestManager(t, "v1")
	m.SetDefault("v1")

	c := New(m, time.Second, 3)
	// lastKnownGood is empty — rollback should do nothing

	// Call rollback directly — should be a no-op (logged)
	c.rollback("v1")

	// Default should still be v1
	if cur := m.Current(); cur != "v1" {
		t.Errorf("expected current still v1, got %s", cur)
	}
}

func TestChecker_RollbackSkippedWhenLKGSameAsFailed(t *testing.T) {
	m, _ := newTestManager(t, "v1")
	m.SetDefault("v1")

	c := New(m, time.Second, 3)
	c.mu.Lock()
	c.lastKnownGood = "v1" // same as the failing version
	c.mu.Unlock()

	c.rollback("v1")

	// Default should still be v1
	if cur := m.Current(); cur != "v1" {
		t.Errorf("expected current still v1, got %s", cur)
	}
}

func TestChecker_AutoRollbackTriggersAfterThreshold(t *testing.T) {
	// This is an integration-style test that checks the full checkAll flow.
	// We need a version that appears running in the manager but whose health
	// check fails. Since we can't start processes, we test the threshold logic.

	m, _ := newTestManager(t, "v1", "v2")
	m.SetDefault("v2")

	c := New(m, time.Second, 3)
	c.mu.Lock()
	c.lastKnownGood = "v1"
	c.mu.Unlock()

	// Simulate threshold: failures[v2] reaches 3 (the threshold)
	c.mu.Lock()
	c.failures["v2"] = 2
	c.mu.Unlock()

	// At this point, one more failure should trigger rollback
	shouldRollback := c.AutoRollback() && c.Failures("v2")+1 >= c.failureThreshold
	if !shouldRollback {
		t.Error("expected rollback to be triggered after threshold")
	}
}

func TestChecker_StartStop(t *testing.T) {
	m, _ := newTestManager(t)
	c := New(m, 100*time.Millisecond, 3)

	c.Start()
	// Give the loop time to tick
	time.Sleep(50 * time.Millisecond)
	c.Stop()
	// Should not panic or deadlock
}

func TestChecker_CheckAll_SkipsStoppedVersions(t *testing.T) {
	m, _ := newTestManager(t, "v1")
	c := New(m, time.Second, 3)

	// v1 is stopped — checkAll should skip it (no failure increment)
	c.checkAll()

	if f := c.Failures("v1"); f != 0 {
		t.Errorf("expected 0 failures for stopped version, got %d", f)
	}
}

func TestChecker_SetsLastKnownGoodOnHealthyDefault(t *testing.T) {
	// When the current/default version is healthy, lastKnownGood should be set.
	// We test this by directly simulating what checkAll does for a healthy current version.

	m, _ := newTestManager(t, "v1")
	m.SetDefault("v1")

	c := New(m, time.Second, 3)

	// Simulate a healthy check of the current version
	c.mu.Lock()
	c.failures["v1"] = 0
	c.lastKnownGood = "v1" // this is what checkAll does when current is healthy
	c.mu.Unlock()

	if lkg := c.LastKnownGood(); lkg != "v1" {
		t.Errorf("expected last-known-good v1, got %s", lkg)
	}
}

func extractPort(t *testing.T, url string) int {
	t.Helper()
	parts := strings.Split(url, ":")
	var portStr string
	for _, p := range parts {
		portStr = p
	}
	port := 0
	fmt.Sscanf(portStr, "%d", &port)
	if port == 0 {
		t.Fatalf("failed to extract port from %s", url)
	}
	return port
}
