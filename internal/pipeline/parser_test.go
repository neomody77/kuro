package pipeline

import (
	"testing"
)

func TestParseWorkflow_Valid(t *testing.T) {
	data := []byte(`{
		"name": "test-workflow",
		"nodes": [
			{"name": "step1", "type": "http.request", "parameters": {"url": "http://example.com"}},
			{"name": "step2", "type": "transform.jq", "parameters": {"expr": ".data"}}
		],
		"connections": {
			"step1": {"main": [[{"node": "step2", "type": "main", "index": 0}]]}
		}
	}`)

	w, err := ParseWorkflow(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.Name != "test-workflow" {
		t.Errorf("name = %q, want %q", w.Name, "test-workflow")
	}
	if len(w.Nodes) != 2 {
		t.Fatalf("node count = %d, want 2", len(w.Nodes))
	}
	if w.Nodes[0].Name != "step1" {
		t.Errorf("node 0 name = %q, want %q", w.Nodes[0].Name, "step1")
	}
	if w.Nodes[1].Name != "step2" {
		t.Errorf("node 1 name = %q, want %q", w.Nodes[1].Name, "step2")
	}
	// Verify connections parsed correctly.
	conn, ok := w.Connections["step1"]
	if !ok {
		t.Fatal("missing connection for step1")
	}
	if len(conn.Main) != 1 || len(conn.Main[0]) != 1 || conn.Main[0][0].Node != "step2" {
		t.Errorf("unexpected connection: %+v", conn)
	}
}

func TestParseWorkflow_Invalid(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"empty", []byte("")},
		{"not json", []byte("not json at all")},
		{"truncated", []byte(`{"name": "test"`)},
		{"wrong type", []byte(`"just a string"`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseWorkflow(tc.data)
			if err == nil {
				t.Errorf("expected error for invalid input %q", tc.name)
			}
		})
	}
}

func TestValidateWorkflow_Valid(t *testing.T) {
	w := &Workflow{
		Name: "valid",
		Nodes: []Node{
			{Name: "a", Type: "http.request"},
			{Name: "b", Type: "transform.jq"},
		},
		Connections: map[string]NodeConnection{
			"a": {Main: [][]ConnectionTarget{{{Node: "b", Type: "main", Index: 0}}}},
		},
	}
	errs := ValidateWorkflow(w)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateWorkflow_MissingName(t *testing.T) {
	w := &Workflow{
		Nodes: []Node{{Name: "a", Type: "http.request"}},
	}
	errs := ValidateWorkflow(w)
	found := false
	for _, e := range errs {
		if e.Error() == "workflow name is required" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'workflow name is required' error, got %v", errs)
	}
}

func TestValidateWorkflow_NoNodes(t *testing.T) {
	w := &Workflow{Name: "empty", Nodes: []Node{}}
	errs := ValidateWorkflow(w)
	found := false
	for _, e := range errs {
		if e.Error() == "workflow must have at least one node" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'workflow must have at least one node' error, got %v", errs)
	}
}

func TestValidateWorkflow_MissingRef(t *testing.T) {
	w := &Workflow{
		Name:  "badref",
		Nodes: []Node{{Name: "a", Type: "http.request"}},
		Connections: map[string]NodeConnection{
			"a": {Main: [][]ConnectionTarget{{{Node: "nonexistent", Type: "main", Index: 0}}}},
		},
	}
	errs := ValidateWorkflow(w)
	if len(errs) == 0 {
		t.Error("expected validation errors for missing ref")
	}
}
