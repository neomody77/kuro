// Package provider defines AI provider adapters for OpenAI, Anthropic, and others.
package provider

import "context"

// Provider is the interface for AI completion providers.
type Provider interface {
	Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)
}

// CompletionRequest represents a request to an AI provider.
type CompletionRequest struct {
	Model    string   `json:"model"`
	Messages []Message `json:"messages"`
}

// Message is a single message in a completion request.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CompletionResponse represents a response from an AI provider.
type CompletionResponse struct {
	Content string `json:"content"`
	Model   string `json:"model"`
	Usage   Usage  `json:"usage"`
}

// Usage tracks token usage for a completion.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// Registry maps provider names to their implementations.
type Registry struct {
	providers map[string]Provider
}

// NewRegistry creates an empty provider registry.
func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]Provider)}
}

// Register adds a provider by name.
func (r *Registry) Register(name string, p Provider) {
	r.providers[name] = p
}

// Get returns a provider by name.
func (r *Registry) Get(name string) (Provider, bool) {
	p, ok := r.providers[name]
	return p, ok
}
