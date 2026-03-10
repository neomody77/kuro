package skill

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/neomody77/kuro/internal/events"
	"github.com/neomody77/kuro/internal/pipeline"
)

// countHandler counts how many times it's been called.
type countHandler struct {
	count atomic.Int32
}

func (h *countHandler) Execute(_ context.Context, _ map[string]any, _ map[string]string) (any, error) {
	h.count.Add(1)
	return map[string]any{"ok": true}, nil
}

var _ pipeline.ActionHandler = (*countHandler)(nil)

func TestHookDispatcherTriggersMatchingSkill(t *testing.T) {
	hub := events.NewHub()
	reg := NewRegistry(nil)
	handler := &countHandler{}

	reg.Register(&Skill{
		Name:    "on_fail",
		On:      []string{"pipeline.error"},
		Handler: handler,
		Source:  "global",
	})

	d := NewHookDispatcher(reg, hub)
	defer d.Stop()

	hub.Publish(events.Event{Type: "pipeline.error", Title: "fail", Message: "oops"})

	// Wait for async dispatch
	deadline := time.After(2 * time.Second)
	for {
		if handler.count.Load() > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for hook to fire")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestHookDispatcherIgnoresNonMatchingEvents(t *testing.T) {
	hub := events.NewHub()
	reg := NewRegistry(nil)
	handler := &countHandler{}

	reg.Register(&Skill{
		Name:    "on_fail",
		On:      []string{"pipeline.error"},
		Handler: handler,
		Source:  "global",
	})

	d := NewHookDispatcher(reg, hub)
	defer d.Stop()

	// Publish a non-matching event
	hub.Publish(events.Event{Type: "pipeline.success", Title: "ok", Message: "done"})

	time.Sleep(200 * time.Millisecond)
	if handler.count.Load() != 0 {
		t.Errorf("handler should not have been called, got count=%d", handler.count.Load())
	}
}

func TestListByEvent(t *testing.T) {
	reg := NewRegistry(nil)
	reg.Register(&Skill{Name: "a", On: []string{"pipeline.error"}})
	reg.Register(&Skill{Name: "b", On: []string{"pipeline.success"}})
	reg.Register(&Skill{Name: "c", On: []string{"pipeline.error", "pipeline.success"}})
	reg.Register(&Skill{Name: "d"}) // no event triggers

	matches := reg.ListByEvent("pipeline.error")
	names := map[string]bool{}
	for _, s := range matches {
		names[s.Name] = true
	}

	if !names["a"] {
		t.Error("expected skill 'a' to match pipeline.error")
	}
	if names["b"] {
		t.Error("skill 'b' should not match pipeline.error")
	}
	if !names["c"] {
		t.Error("expected skill 'c' to match pipeline.error")
	}
	if names["d"] {
		t.Error("skill 'd' should not match pipeline.error")
	}
}
