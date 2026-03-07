// Package ai implements AI completion actions.
package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/neomody77/kuro/internal/provider"
)

// CompleteAction calls an AI provider for text completion.
type CompleteAction struct {
	Providers *provider.Registry
}

func (a *CompleteAction) Execute(ctx context.Context, params map[string]any, creds map[string]string) (any, error) {
	providerStr, _ := params["provider"].(string)
	prompt, _ := params["prompt"].(string)
	input, _ := params["input"].(string)
	fallback, _ := params["fallback"].(string)

	if providerStr == "" {
		providerStr = "openai"
	}

	// Parse "provider/model" format.
	providerName, model := parseProvider(providerStr)

	if prompt == "" && input == "" {
		return nil, fmt.Errorf("ai.complete: 'prompt' or 'input' parameter is required")
	}

	// Build messages.
	var messages []provider.Message
	if prompt != "" {
		messages = append(messages, provider.Message{Role: "system", Content: prompt})
	}
	if input != "" {
		messages = append(messages, provider.Message{Role: "user", Content: input})
	}

	if a.Providers == nil {
		return a.handleFallback(fallback, input, fmt.Errorf("ai.complete: no provider registry configured"))
	}

	p, ok := a.Providers.Get(providerName)
	if !ok {
		return a.handleFallback(fallback, input, fmt.Errorf("ai.complete: unknown provider %q", providerName))
	}

	resp, err := p.Complete(ctx, &provider.CompletionRequest{
		Model:    model,
		Messages: messages,
	})
	if err != nil {
		return a.handleFallback(fallback, input, err)
	}

	return map[string]any{
		"output": resp.Content,
		"model":  resp.Model,
		"usage": map[string]any{
			"prompt_tokens":     resp.Usage.PromptTokens,
			"completion_tokens": resp.Usage.CompletionTokens,
		},
	}, nil
}

func (a *CompleteAction) handleFallback(policy, input string, err error) (any, error) {
	switch policy {
	case "skip":
		return map[string]any{"output": "", "skipped": true}, nil
	case "raw":
		return map[string]any{"output": input, "raw": true}, nil
	case "error", "":
		return nil, err
	default:
		return nil, err
	}
}

func parseProvider(s string) (name, model string) {
	parts := strings.SplitN(s, "/", 2)
	name = parts[0]
	if len(parts) == 2 {
		model = parts[1]
	}
	return
}
