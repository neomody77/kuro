package skill

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"time"
)

const (
	maxOutputSize   = 1 << 20 // 1 MB
	defaultTimeout  = 30 * time.Second
)

// ShellHandler executes a skill via a shell command.
// Input params are marshalled to JSON and piped to stdin.
// stdout is parsed as JSON; if parsing fails, raw output is returned.
type ShellHandler struct {
	Command string
}

func (h *ShellHandler) Execute(ctx context.Context, params map[string]any, _ map[string]string) (any, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	input, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("shell handler: marshal input: %w", err)
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", h.Command)
	cmd.Stdin = bytes.NewReader(input)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &limitedWriter{buf: &stdout, limit: maxOutputSize}
	cmd.Stderr = &limitedWriter{buf: &stderr, limit: maxOutputSize}

	if err := cmd.Run(); err != nil {
		exitCode := 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
		// Still try to parse stdout even on non-zero exit
		if stdout.Len() > 0 {
			var result map[string]any
			if json.Unmarshal(stdout.Bytes(), &result) == nil {
				result["exit_code"] = exitCode
				return result, nil
			}
		}
		return map[string]any{
			"stdout":    stdout.String(),
			"stderr":    stderr.String(),
			"exit_code": exitCode,
		}, nil
	}

	// Try to parse stdout as JSON
	raw := stdout.Bytes()
	if len(raw) == 0 {
		return map[string]any{"exit_code": 0}, nil
	}

	var result any
	if err := json.Unmarshal(raw, &result); err != nil {
		return map[string]any{
			"stdout":    string(raw),
			"exit_code": 0,
		}, nil
	}
	return result, nil
}

// HTTPHandler executes a skill by POSTing JSON to an HTTP endpoint.
type HTTPHandler struct {
	Endpoint string
}

func (h *HTTPHandler) Execute(ctx context.Context, params map[string]any, _ map[string]string) (any, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	body, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("http handler: marshal input: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("http handler: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http handler: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxOutputSize))
	if err != nil {
		return nil, fmt.Errorf("http handler: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http handler: status %d: %s", resp.StatusCode, string(respBody))
	}

	var result any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return map[string]any{"body": string(respBody)}, nil
	}
	return result, nil
}

// limitedWriter is a bytes.Buffer wrapper that stops writing after limit bytes.
type limitedWriter struct {
	buf   *bytes.Buffer
	limit int64
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	remaining := w.limit - int64(w.buf.Len())
	if remaining <= 0 {
		return len(p), nil // discard silently
	}
	if int64(len(p)) > remaining {
		p = p[:remaining]
	}
	return w.buf.Write(p)
}
