// Package health implements health checking for engine processes.
package health

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/neomody77/kuro/recovery/version"
)

// Checker periodically pings running engines and triggers rollback on repeated failures.
type Checker struct {
	manager          *version.Manager
	interval         time.Duration
	failureThreshold int
	autoRollback     bool

	mu              sync.RWMutex
	failures        map[string]int // versionID → consecutive failure count
	lastKnownGood   string
	stopCh          chan struct{}
	client          *http.Client
}

// New creates a new HealthChecker.
func New(manager *version.Manager, interval time.Duration, failureThreshold int) *Checker {
	return &Checker{
		manager:          manager,
		interval:         interval,
		failureThreshold: failureThreshold,
		autoRollback:     true,
		failures:         make(map[string]int),
		stopCh:           make(chan struct{}),
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// SetAutoRollback enables or disables auto-rollback.
func (c *Checker) SetAutoRollback(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.autoRollback = enabled
}

// AutoRollback returns whether auto-rollback is enabled.
func (c *Checker) AutoRollback() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.autoRollback
}

// Failures returns the consecutive failure count for a version.
func (c *Checker) Failures(id string) int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.failures[id]
}

// LastKnownGood returns the last version ID that was healthy as current.
func (c *Checker) LastKnownGood() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastKnownGood
}

// Start begins the periodic health check loop.
func (c *Checker) Start() {
	go c.loop()
}

// Stop terminates the health check loop.
func (c *Checker) Stop() {
	close(c.stopCh)
}

func (c *Checker) loop() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.checkAll()
		}
	}
}

func (c *Checker) checkAll() {
	versions := c.manager.List()
	currentID := c.manager.Current()

	for _, v := range versions {
		if v.Status != version.StatusRunning {
			continue
		}
		healthy := c.ping(v.Port)

		c.mu.Lock()
		if healthy {
			c.failures[v.ID] = 0
			// Track last known good for current version
			if v.ID == currentID {
				c.lastKnownGood = v.ID
			}
		} else {
			c.failures[v.ID]++
			count := c.failures[v.ID]
			shouldRollback := c.autoRollback && v.ID == currentID && count >= c.failureThreshold
			c.mu.Unlock()

			if shouldRollback {
				c.rollback(v.ID)
			}
			continue
		}
		c.mu.Unlock()
	}
}

func (c *Checker) ping(port int) bool {
	resp, err := c.client.Get(fmt.Sprintf("http://127.0.0.1:%d/api/health", port))
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (c *Checker) rollback(failedID string) {
	c.mu.RLock()
	lkg := c.lastKnownGood
	c.mu.RUnlock()

	if lkg == "" || lkg == failedID {
		log.Printf("health: version %s failing but no last-known-good to rollback to", failedID)
		return
	}

	log.Printf("health: auto-rollback from %s to last-known-good %s", failedID, lkg)

	// Ensure last-known-good is running
	if p := c.manager.PortForVersion(lkg); p == 0 {
		if err := c.manager.Start(lkg); err != nil {
			log.Printf("health: failed to start last-known-good %s: %v", lkg, err)
			return
		}
	}

	if err := c.manager.SetDefault(lkg); err != nil {
		log.Printf("health: failed to set default to %s: %v", lkg, err)
	}
}
