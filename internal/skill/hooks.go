package skill

import (
	"context"
	"time"

	"github.com/neomody77/kuro/internal/events"
)

const hookTimeout = 30 * time.Second

// HookDispatcher listens to EventHub and executes skills whose On field
// matches the event type. Each matching skill is executed asynchronously.
type HookDispatcher struct {
	registry *Registry
	cancel   func()
}

// NewHookDispatcher creates a dispatcher that subscribes to the event hub
// and triggers matching skills for each event.
func NewHookDispatcher(registry *Registry, hub *events.Hub) *HookDispatcher {
	ch, unsub := hub.Subscribe()

	ctx, cancel := context.WithCancel(context.Background())

	d := &HookDispatcher{
		registry: registry,
		cancel: func() {
			cancel()
			unsub()
		},
	}

	go d.loop(ctx, ch)
	return d
}

// Stop shuts down the dispatcher and unsubscribes from the event hub.
func (d *HookDispatcher) Stop() {
	d.cancel()
}

func (d *HookDispatcher) loop(ctx context.Context, ch <-chan events.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			d.dispatch(ev)
		}
	}
}

func (d *HookDispatcher) dispatch(ev events.Event) {
	params := map[string]any{
		"event":   ev.Type,
		"title":   ev.Title,
		"message": ev.Message,
	}
	if ev.Meta != nil {
		params["meta"] = ev.Meta
	}

	ctx, cancel := context.WithTimeout(context.Background(), hookTimeout)
	go func() {
		defer cancel()
		d.registry.ExecuteEvent(ctx, ev.Type, params)
	}()
}
