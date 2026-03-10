package skill

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestShellHandlerEcho(t *testing.T) {
	h := &ShellHandler{Command: `echo '{"greeting":"hello"}'`}
	result, err := h.Execute(context.Background(), map[string]any{"name": "world"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if m["greeting"] != "hello" {
		t.Errorf("greeting = %v, want hello", m["greeting"])
	}
}

func TestShellHandlerReadStdin(t *testing.T) {
	// The command reads stdin JSON and echoes it back
	h := &ShellHandler{Command: "cat"}
	result, err := h.Execute(context.Background(), map[string]any{"key": "value"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if m["key"] != "value" {
		t.Errorf("key = %v, want value", m["key"])
	}
}

func TestShellHandlerNonJSON(t *testing.T) {
	h := &ShellHandler{Command: "echo 'not json'"}
	result, err := h.Execute(context.Background(), map[string]any{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if m["stdout"] != "not json\n" {
		t.Errorf("stdout = %q, want %q", m["stdout"], "not json\n")
	}
	if m["exit_code"] != 0 {
		t.Errorf("exit_code = %v, want 0", m["exit_code"])
	}
}

func TestShellHandlerFailure(t *testing.T) {
	h := &ShellHandler{Command: "exit 42"}
	result, err := h.Execute(context.Background(), map[string]any{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if m["exit_code"] != 42 {
		t.Errorf("exit_code = %v, want 42", m["exit_code"])
	}
}

func TestHTTPHandlerSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content-type = %s, want application/json", r.Header.Get("Content-Type"))
		}
		var params map[string]any
		json.NewDecoder(r.Body).Decode(&params)
		json.NewEncoder(w).Encode(map[string]any{
			"result": "ok",
			"echo":   params["input"],
		})
	}))
	defer server.Close()

	h := &HTTPHandler{Endpoint: server.URL}
	result, err := h.Execute(context.Background(), map[string]any{"input": "test"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if m["result"] != "ok" {
		t.Errorf("result = %v, want ok", m["result"])
	}
	if m["echo"] != "test" {
		t.Errorf("echo = %v, want test", m["echo"])
	}
}

func TestHTTPHandlerErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	h := &HTTPHandler{Endpoint: server.URL}
	_, err := h.Execute(context.Background(), map[string]any{}, nil)
	if err == nil {
		t.Fatal("expected error for 500 status")
	}
	if !containsStr(err.Error(), "500") {
		t.Errorf("error = %q, want mention of 500", err.Error())
	}
}

func TestHTTPHandlerNonJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("plain text response"))
	}))
	defer server.Close()

	h := &HTTPHandler{Endpoint: server.URL}
	result, err := h.Execute(context.Background(), map[string]any{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if m["body"] != "plain text response" {
		t.Errorf("body = %v, want 'plain text response'", m["body"])
	}
}
