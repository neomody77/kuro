// Package http implements HTTP request actions.
package http

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// RequestAction makes HTTP requests.
type RequestAction struct{}

func (a *RequestAction) Execute(ctx context.Context, params map[string]any, creds map[string]string) (any, error) {
	method, _ := params["method"].(string)
	if method == "" {
		method = "GET"
	}
	method = strings.ToUpper(method)

	url, _ := params["url"].(string)
	if url == "" {
		return nil, fmt.Errorf("http.request: 'url' parameter is required")
	}

	timeoutF, _ := params["timeout"].(float64)
	timeout := time.Duration(timeoutF) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	var bodyReader io.Reader
	if bodyStr, ok := params["body"].(string); ok && bodyStr != "" {
		bodyReader = strings.NewReader(bodyStr)
	} else if bodyBytes, ok := params["body"].([]byte); ok {
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("http.request: create request: %w", err)
	}

	// Set headers from params.
	if headers, ok := params["headers"].(map[string]any); ok {
		for k, v := range headers {
			req.Header.Set(k, fmt.Sprintf("%v", v))
		}
	}

	// Apply credential-based auth.
	if token := creds["token"]; token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	} else if user, pass := creds["username"], creds["password"]; user != "" {
		req.SetBasicAuth(user, pass)
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http.request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // 10MB limit
	if err != nil {
		return nil, fmt.Errorf("http.request: read body: %w", err)
	}

	respHeaders := make(map[string]string, len(resp.Header))
	for k := range resp.Header {
		respHeaders[k] = resp.Header.Get(k)
	}

	return map[string]any{
		"status":  resp.StatusCode,
		"headers": respHeaders,
		"body":    string(respBody),
	}, nil
}
