// Package action provides the action registry and built-in action implementations.
package action

import (
	"sync"

	"github.com/neomody77/kuro/internal/action/ai"
	"github.com/neomody77/kuro/internal/action/email"
	fileaction "github.com/neomody77/kuro/internal/action/file"
	httpaction "github.com/neomody77/kuro/internal/action/http"
	"github.com/neomody77/kuro/internal/action/shell"
	"github.com/neomody77/kuro/internal/action/template"
	"github.com/neomody77/kuro/internal/action/transform"
	"github.com/neomody77/kuro/internal/pipeline"
	"github.com/neomody77/kuro/internal/provider"
)

// Registry maps action names to their handlers.
type Registry struct {
	mu       sync.RWMutex
	handlers map[string]pipeline.ActionHandler
}

// NewRegistry creates an empty action registry.
func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]pipeline.ActionHandler)}
}

// Register adds a handler for the given action name.
func (r *Registry) Register(name string, handler pipeline.ActionHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[name] = handler
}

// Get returns the handler for the given action name.
func (r *Registry) Get(name string) (pipeline.ActionHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[name]
	return h, ok
}

// All returns a copy of all registered action handlers.
func (r *Registry) All() map[string]pipeline.ActionHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]pipeline.ActionHandler, len(r.handlers))
	for k, v := range r.handlers {
		out[k] = v
	}
	return out
}

// Config holds configuration for creating a default registry.
type Config struct {
	WorkspaceDir string
	DocumentsDir string
	Providers    *provider.Registry
}

// NewDefaultRegistry creates a registry with all built-in actions pre-registered.
func NewDefaultRegistry(cfg Config) *Registry {
	r := NewRegistry()

	// Email actions.
	r.Register("email.fetch", &email.FetchAction{})
	r.Register("email.send", &email.SendAction{})

	// HTTP action.
	r.Register("http.request", &httpaction.RequestAction{})

	// File actions.
	r.Register("file.read", &fileaction.ReadAction{WorkspaceDir: cfg.WorkspaceDir})
	r.Register("file.write", &fileaction.WriteAction{WorkspaceDir: cfg.WorkspaceDir})
	r.Register("file.list", &fileaction.ListAction{WorkspaceDir: cfg.WorkspaceDir})

	// Shell action.
	r.Register("shell.exec", &shell.ExecAction{})

	// Transform action.
	r.Register("transform.jq", &transform.JqAction{})

	// Template action.
	r.Register("transform.template", &template.RenderAction{DocumentsDir: cfg.DocumentsDir})

	// AI action.
	r.Register("ai.complete", &ai.CompleteAction{Providers: cfg.Providers})

	return r
}
