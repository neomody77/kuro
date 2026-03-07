package pipeline

import (
	"encoding/json"
	"fmt"
	"time"
)

// ParseWorkflow parses a JSON workflow definition into a Workflow struct.
func ParseWorkflow(data []byte) (*Workflow, error) {
	var w Workflow
	if err := json.Unmarshal(data, &w); err != nil {
		return nil, fmt.Errorf("parse workflow: %w", err)
	}
	if w.ID == "" {
		w.ID = fmt.Sprintf("wf_%d", time.Now().UnixMilli())
	}
	if w.Connections == nil {
		w.Connections = make(map[string]NodeConnection)
	}
	return &w, nil
}

// ParsePipeline is a legacy alias for ParseWorkflow.
func ParsePipeline(data []byte) (*Workflow, error) {
	return ParseWorkflow(data)
}

// ValidateWorkflow checks a workflow definition for structural errors.
func ValidateWorkflow(w *Workflow) []error {
	var errs []error

	if w.Name == "" {
		errs = append(errs, fmt.Errorf("workflow name is required"))
	}
	if len(w.Nodes) == 0 {
		errs = append(errs, fmt.Errorf("workflow must have at least one node"))
	}

	// Build a set of node names for reference checking.
	nodeNames := make(map[string]bool, len(w.Nodes))
	for _, node := range w.Nodes {
		nodeNames[node.Name] = true
	}

	// Check that all connection targets reference existing nodes.
	for srcName, conn := range w.Connections {
		if !nodeNames[srcName] {
			errs = append(errs, fmt.Errorf("connection source %q is not a valid node", srcName))
		}
		for _, outputs := range conn.Main {
			for _, target := range outputs {
				if !nodeNames[target.Node] {
					errs = append(errs, fmt.Errorf("connection from %q references unknown node %q", srcName, target.Node))
				}
			}
		}
	}

	return errs
}

// ValidatePipeline is a legacy alias for ValidateWorkflow.
func ValidatePipeline(w *Workflow) []error {
	return ValidateWorkflow(w)
}
