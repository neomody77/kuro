package pipeline

import (
	"strings"
	"testing"
	"time"
)

func TestResolveExpressions_NodeOutput(t *testing.T) {
	results := map[string]*NodeResult{
		"fetch": {
			NodeID: "fetch",
			Status: NodeCompleted,
			Output: "hello world",
		},
	}

	got := ResolveExpressions("result is {{ nodes.fetch.output }}", results)
	if got != "result is hello world" {
		t.Errorf("got %q, want %q", got, "result is hello world")
	}
}

func TestResolveExpressions_NodeStatus(t *testing.T) {
	results := map[string]*NodeResult{
		"step1": {
			NodeID: "step1",
			Status: NodeCompleted,
		},
	}

	got := ResolveExpressions("{{ nodes.step1.status }}", results)
	if got != "completed" {
		t.Errorf("got %q, want %q", got, "completed")
	}
}

func TestResolveExpressions_NodeError(t *testing.T) {
	results := map[string]*NodeResult{
		"fail": {
			NodeID: "fail",
			Status: NodeFailed,
			Error:  "connection refused",
		},
	}

	got := ResolveExpressions("{{ nodes.fail.error }}", results)
	if got != "connection refused" {
		t.Errorf("got %q, want %q", got, "connection refused")
	}
}

func TestResolveExpressions_MapField(t *testing.T) {
	results := map[string]*NodeResult{
		"fetch": {
			NodeID: "fetch",
			Status: NodeCompleted,
			Output: map[string]any{"subject": "test email"},
		},
	}

	got := ResolveExpressions("{{ nodes.fetch.subject }}", results)
	if got != "test email" {
		t.Errorf("got %q, want %q", got, "test email")
	}
}

func TestResolveExpressions_MissingNode(t *testing.T) {
	results := map[string]*NodeResult{}

	got := ResolveExpressions("{{ nodes.missing.output }}", results)
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

func TestResolveExpressions_Now(t *testing.T) {
	got := ResolveExpressions("{{ now }}", nil)
	_, err := time.Parse(time.RFC3339, got)
	if err != nil {
		t.Errorf("expected RFC3339 timestamp, got %q: %v", got, err)
	}
}

func TestResolveExpressions_NowDateFilter(t *testing.T) {
	got := ResolveExpressions("{{ now | date('YYYY-MM-DD') }}", nil)
	expected := time.Now().Format("2006-01-02")
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestResolveExpressions_LengthFilter(t *testing.T) {
	results := map[string]*NodeResult{
		"x": {NodeID: "x", Status: NodeCompleted, Output: "abcde"},
	}

	got := ResolveExpressions("{{ nodes.x.output | length }}", results)
	if got != "5" {
		t.Errorf("got %q, want %q", got, "5")
	}
}

func TestResolveExpressions_ContainsFilter(t *testing.T) {
	results := map[string]*NodeResult{
		"x": {NodeID: "x", Status: NodeCompleted, Output: "hello world"},
	}

	got := ResolveExpressions("{{ nodes.x.output | contains('world') }}", results)
	if got != "true" {
		t.Errorf("got %q, want %q", got, "true")
	}

	got = ResolveExpressions("{{ nodes.x.output | contains('xyz') }}", results)
	if got != "false" {
		t.Errorf("got %q, want %q", got, "false")
	}
}

func TestResolveExpressions_UpperFilter(t *testing.T) {
	results := map[string]*NodeResult{
		"x": {NodeID: "x", Status: NodeCompleted, Output: "hello"},
	}

	got := ResolveExpressions("{{ nodes.x.output | upper }}", results)
	if got != "HELLO" {
		t.Errorf("got %q, want %q", got, "HELLO")
	}
}

func TestResolveExpressions_LowerFilter(t *testing.T) {
	results := map[string]*NodeResult{
		"x": {NodeID: "x", Status: NodeCompleted, Output: "HELLO"},
	}

	got := ResolveExpressions("{{ nodes.x.output | lower }}", results)
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestResolveExpressions_MultipleExpressions(t *testing.T) {
	results := map[string]*NodeResult{
		"a": {NodeID: "a", Status: NodeCompleted, Output: "foo"},
		"b": {NodeID: "b", Status: NodeCompleted, Output: "bar"},
	}

	got := ResolveExpressions("{{ nodes.a.output }} and {{ nodes.b.output }}", results)
	if got != "foo and bar" {
		t.Errorf("got %q, want %q", got, "foo and bar")
	}
}

func TestResolveExpressions_NoExpressions(t *testing.T) {
	got := ResolveExpressions("plain text", nil)
	if got != "plain text" {
		t.Errorf("got %q, want %q", got, "plain text")
	}
}

func TestResolveParams(t *testing.T) {
	results := map[string]*NodeResult{
		"prev": {NodeID: "prev", Status: NodeCompleted, Output: "data"},
	}
	params := map[string]any{
		"url":   "http://example.com/{{ nodes.prev.output }}",
		"count": 42,
	}

	resolved := ResolveParams(params, results)
	if resolved["url"] != "http://example.com/data" {
		t.Errorf("url = %q, want %q", resolved["url"], "http://example.com/data")
	}
	if resolved["count"] != 42 {
		t.Errorf("count = %v, want 42", resolved["count"])
	}
}

func TestResolveExpressions_Duration(t *testing.T) {
	results := map[string]*NodeResult{
		"x": {NodeID: "x", Status: NodeCompleted, Duration: 5 * time.Second},
	}

	got := ResolveExpressions("{{ nodes.x.duration }}", results)
	if !strings.Contains(got, "5s") {
		t.Errorf("got %q, want string containing 5s", got)
	}
}
