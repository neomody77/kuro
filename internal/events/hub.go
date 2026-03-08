// Package events provides a publish-subscribe event hub for real-time SSE streaming.
package events

import (
	"sync"
	"time"
)

// Event is a server-sent event pushed to all connected clients.
type Event struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`      // "pipeline.completed", "pipeline.failed", "cron.triggered", "credential.changed", etc.
	Title     string         `json:"title"`      // short summary
	Message   string         `json:"message"`    // detail text
	Severity  string         `json:"severity"`   // "info", "success", "error", "warning"
	Timestamp time.Time      `json:"timestamp"`
	Meta      map[string]any `json:"meta,omitempty"`
}

// subscriber is a channel that receives events.
type subscriber struct {
	ch     chan Event
	closed bool
}

// Hub fans out events to all connected SSE clients.
type Hub struct {
	mu          sync.RWMutex
	subscribers map[*subscriber]struct{}
	recent      []Event // ring buffer of recent events for new clients
	maxRecent   int
}

// NewHub creates a new event hub.
func NewHub() *Hub {
	return &Hub{
		subscribers: make(map[*subscriber]struct{}),
		maxRecent:   50,
	}
}

// Publish sends an event to all subscribers.
func (h *Hub) Publish(ev Event) {
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now()
	}
	if ev.ID == "" {
		ev.ID = time.Now().Format("20060102150405.000000")
	}

	h.mu.Lock()
	// Add to recent ring buffer
	h.recent = append(h.recent, ev)
	if len(h.recent) > h.maxRecent {
		h.recent = h.recent[len(h.recent)-h.maxRecent:]
	}
	// Copy subscriber list under lock
	subs := make([]*subscriber, 0, len(h.subscribers))
	for s := range h.subscribers {
		subs = append(subs, s)
	}
	h.mu.Unlock()

	for _, s := range subs {
		select {
		case s.ch <- ev:
		default:
			// slow consumer — drop event
		}
	}
}

// Subscribe returns a channel of events and a cancel function.
// The cancel function must be called when done to avoid leaks.
func (h *Hub) Subscribe() (<-chan Event, func()) {
	s := &subscriber{
		ch: make(chan Event, 32),
	}

	h.mu.Lock()
	h.subscribers[s] = struct{}{}
	// Send recent events to catch up
	recent := make([]Event, len(h.recent))
	copy(recent, h.recent)
	h.mu.Unlock()

	// Enqueue recent events without blocking
	go func() {
		for _, ev := range recent {
			select {
			case s.ch <- ev:
			default:
			}
		}
	}()

	cancel := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if !s.closed {
			s.closed = true
			delete(h.subscribers, s)
			close(s.ch)
		}
	}

	return s.ch, cancel
}

// Recent returns the most recent events (up to maxRecent).
func (h *Hub) Recent() []Event {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]Event, len(h.recent))
	copy(out, h.recent)
	return out
}
