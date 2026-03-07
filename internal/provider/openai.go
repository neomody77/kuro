package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// OpenAIProvider implements the Provider interface using the OpenAI-compatible API.
// Works with OpenAI, OpenRouter, and any compatible endpoint.
type OpenAIProvider struct {
	BaseURL string
	APIKey  string
	Client  *http.Client
}

func NewOpenAIProvider(baseURL, apiKey string) *OpenAIProvider {
	return &OpenAIProvider{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Client:  http.DefaultClient,
	}
}

type openaiRequest struct {
	Model    string        `json:"model"`
	Messages []openaiMsg   `json:"messages"`
}

type openaiMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (p *OpenAIProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	msgs := make([]openaiMsg, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = openaiMsg{Role: m.Role, Content: m.Content}
	}

	body, err := json.Marshal(openaiRequest{
		Model:    req.Model,
		Messages: msgs,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := p.Client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var oaiResp openaiResponse
	if err := json.Unmarshal(respBody, &oaiResp); err != nil {
		return nil, fmt.Errorf("parse response: %w (body: %s)", err, string(respBody[:min(len(respBody), 200)]))
	}
	if len(oaiResp.Choices) > 0 && oaiResp.Choices[0].Message.Content == "" {
		log.Printf("[provider] WARNING: empty content from %s (status=%d, body=%s)", req.Model, resp.StatusCode, string(respBody[:min(len(respBody), 500)]))
	}

	if oaiResp.Error != nil {
		return nil, fmt.Errorf("api error: %s", oaiResp.Error.Message)
	}

	if len(oaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return &CompletionResponse{
		Content: oaiResp.Choices[0].Message.Content,
		Model:   oaiResp.Model,
		Usage: Usage{
			PromptTokens:     oaiResp.Usage.PromptTokens,
			CompletionTokens: oaiResp.Usage.CompletionTokens,
		},
	}, nil
}
