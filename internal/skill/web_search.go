package skill

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// webSearchSkill implements the web_search skill using the Tavily Search API.
// The API key is read from skill config (creds["api_key"]), not from settings.
type webSearchSkill struct{}

// tavilyRequest is the JSON body sent to the Tavily Search API.
type tavilyRequest struct {
	APIKey     string `json:"api_key"`
	Query      string `json:"query"`
	MaxResults int    `json:"max_results"`
}

// tavilyResponse is the JSON response from the Tavily Search API.
type tavilyResponse struct {
	Answer  string         `json:"answer"`
	Results []tavilyResult `json:"results"`
}

// tavilyResult represents a single search result from Tavily.
type tavilyResult struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

func (s *webSearchSkill) Execute(ctx context.Context, params map[string]any, creds map[string]string) (any, error) {
	action, _ := params["action"].(string)
	if action == "" {
		action = "search"
	}

	switch action {
	case "search":
		return s.search(ctx, params, creds)
	default:
		return nil, fmt.Errorf("web_search: unknown action %q (use: search)", action)
	}
}

func (s *webSearchSkill) search(ctx context.Context, params map[string]any, creds map[string]string) (any, error) {
	query, _ := params["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("web_search: 'query' is required")
	}

	maxResults := 5
	if mr, ok := params["max_results"].(float64); ok && mr > 0 {
		maxResults = int(mr)
	} else if mr, ok := params["max_results"].(int); ok && mr > 0 {
		maxResults = mr
	} else if mr, ok := params["max_results"].(string); ok && mr != "" {
		var n int
		if _, err := fmt.Sscanf(mr, "%d", &n); err == nil && n > 0 {
			maxResults = n
		}
	}

	if maxResults > 20 {
		maxResults = 20
	}

	// Read API key from skill config (credential store)
	apiKey := creds["api_key"]
	if apiKey == "" {
		return nil, fmt.Errorf("web_search: API key not configured. Go to Skills > web_search to add your Tavily API key")
	}

	reqBody := tavilyRequest{
		APIKey:     apiKey,
		Query:      query,
		MaxResults: maxResults,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("web_search: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.tavily.com/search", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("web_search: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("web_search: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("web_search: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("web_search: Tavily API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var tavilyResp tavilyResponse
	if err := json.Unmarshal(respBody, &tavilyResp); err != nil {
		return nil, fmt.Errorf("web_search: parse response: %w", err)
	}

	results := make([]map[string]any, 0, len(tavilyResp.Results))
	for _, r := range tavilyResp.Results {
		results = append(results, map[string]any{
			"title":   r.Title,
			"url":     r.URL,
			"content": r.Content,
			"score":   r.Score,
		})
	}

	out := map[string]any{
		"results": results,
		"count":   len(results),
	}
	if tavilyResp.Answer != "" {
		out["answer"] = tavilyResp.Answer
	}

	return out, nil
}
