package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// Executor runs workflows by walking the DAG and invoking node handlers with data flow.
type Executor struct {
	mu           sync.RWMutex
	actions      map[string]ActionHandler
	nodeHandlers map[string]NodeHandler
	execStore    ExecutionStore
	credResolver CredentialResolver
	onComplete   func(exec *Execution, wf *Workflow) // optional callback after execution finishes
}

// NewExecutor creates a new workflow executor.
func NewExecutor(store ExecutionStore) *Executor {
	return &Executor{
		actions:      make(map[string]ActionHandler),
		nodeHandlers: make(map[string]NodeHandler),
		execStore:    store,
	}
}

// SetCredentialResolver sets the credential resolver for node credential lookups.
func (e *Executor) SetCredentialResolver(cr CredentialResolver) {
	e.credResolver = cr
}

// SetOnComplete sets a callback invoked after each execution finishes.
func (e *Executor) SetOnComplete(fn func(exec *Execution, wf *Workflow)) {
	e.onComplete = fn
}

// RegisterAction registers a legacy action handler by name.
func (e *Executor) RegisterAction(name string, handler ActionHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.actions[name] = handler
}

// RegisterNodeHandler registers an n8n-compatible node handler by node type.
func (e *Executor) RegisterNodeHandler(nodeType string, handler NodeHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.nodeHandlers[nodeType] = handler
}

func (e *Executor) getNodeHandler(nodeType string) (NodeHandler, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	h, ok := e.nodeHandlers[nodeType]
	return h, ok
}

func (e *Executor) getAction(name string) (ActionHandler, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	h, ok := e.actions[name]
	return h, ok
}

// Execute runs a workflow to completion with data flow between nodes.
func (e *Executor) Execute(ctx context.Context, w *Workflow) (*Execution, error) {
	dag := BuildDAG(w)
	sorted, err := dag.TopologicalSort()
	if err != nil {
		return nil, fmt.Errorf("build execution plan: %w", err)
	}

	nodeByName := make(map[string]*Node, len(w.Nodes))
	for i := range w.Nodes {
		nodeByName[w.Nodes[i].Name] = &w.Nodes[i]
	}

	now := time.Now()
	exec := &Execution{
		ID:         fmt.Sprintf("exec_%d", now.UnixNano()),
		WorkflowID: w.ID,
		Status:     ExecRunning,
		Mode:       ModeManual,
		StartedAt:  now,
		Data: &ExecutionData{
			ResultData: ResultData{
				RunData: make(map[string][]NodeRunData),
			},
		},
	}

	// Track node outputs for data flow
	nodeOutputs := make(map[string]*NodeOutput)

	for _, nodeName := range sorted {
		if ctx.Err() != nil {
			exec.Status = ExecCanceled
			break
		}

		node := nodeByName[nodeName]
		if node == nil || node.Disabled {
			continue
		}

		// Gather input from predecessor nodes
		input := buildInputForNode(nodeName, w, nodeOutputs)

		// Determine if this is a root node (no predecessors in DAG)
		isRoot := dag.InDegree[nodeName] == 0
		isTrigger := isTriggerNode(node)

		// Skip non-root, non-trigger nodes that have no input items
		if !isRoot && !isTrigger && len(input) == 0 {
			continue
		}

		start := time.Now()
		runData := NodeRunData{StartTime: start.UnixMilli()}

		// Resolve credentials for this node
		creds := e.resolveNodeCredentials(node)

		// Try NodeHandler first, then fall back to ActionHandler
		if handler, ok := e.getNodeHandler(node.Type); ok {
			// Inject staticData into node parameters for trigger nodes
			if isTrigger {
				e.injectStaticData(w, node)
			}

			output, err := handler.ExecuteNode(ctx, node, input, creds)
			if err != nil {
				runData.Error = &ExecutionError{Message: err.Error(), Node: node.Name}
				log.Printf("[executor] node %q error: %v", node.Name, err)
			} else {
				nodeOutputs[nodeName] = output
				totalItems := 0
				for _, items := range output.Items {
					totalItems += len(items)
				}
				runData.Data = map[string]any{"itemCount": totalItems}
			}

			// Update staticData from trigger node
			if isTrigger {
				e.updateStaticData(w, node)
			}
		} else if handler, ok := e.getAction(node.Type); ok {
			// Legacy ActionHandler — run once, wrap result as NodeOutput
			params := make(map[string]any, len(node.Parameters))
			for k, v := range node.Parameters {
				params[k] = v
			}
			legacyCreds := make(map[string]string)
			for credType, credRef := range node.Credentials {
				legacyCreds[credType] = credRef.Name
			}
			result, err := handler.Execute(ctx, params, legacyCreds)
			if err != nil {
				runData.Error = &ExecutionError{Message: err.Error(), Node: node.Name}
			} else {
				runData.Data = map[string]any{"output": result}
				// Wrap legacy output as NodeOutput
				switch r := result.(type) {
				case map[string]any:
					nodeOutputs[nodeName] = &NodeOutput{Items: [][]NodeItem{{NodeItem(r)}}}
				default:
					nodeOutputs[nodeName] = &NodeOutput{Items: [][]NodeItem{{{"output": result}}}}
				}
			}
		} else {
			runData.Error = &ExecutionError{
				Message: fmt.Sprintf("unknown node type: %s", node.Type),
				Node:    node.Name,
			}
		}

		runData.ExecutionTime = time.Since(start).Milliseconds()
		exec.Data.ResultData.RunData[nodeName] = append(exec.Data.ResultData.RunData[nodeName], runData)
		exec.Data.ResultData.LastNodeExecuted = nodeName
	}

	if exec.Status == ExecRunning {
		exec.Status = ExecSuccess
		for _, runs := range exec.Data.ResultData.RunData {
			for _, r := range runs {
				if r.Error != nil {
					exec.Status = ExecError
					break
				}
			}
		}
	}

	stopped := time.Now()
	exec.StoppedAt = &stopped
	exec.Finished = true

	// Skip persisting empty trigger executions (no data flowed)
	empty := isEmptyTriggerExecution(exec, nodeOutputs)
	if e.execStore != nil && !empty {
		_ = e.execStore.SaveExecution(exec)
	}

	if e.onComplete != nil && !empty {
		e.onComplete(exec, w)
	}

	return exec, nil
}

// isEmptyTriggerExecution returns true if the execution produced no items
// from any trigger node, meaning there was nothing to process.
func isEmptyTriggerExecution(exec *Execution, nodeOutputs map[string]*NodeOutput) bool {
	if exec.Status != ExecSuccess {
		return false // always save errors
	}
	totalItems := 0
	for _, out := range nodeOutputs {
		for _, items := range out.Items {
			totalItems += len(items)
		}
	}
	return totalItems == 0
}

// buildInputForNode gathers input items for a node from its predecessors' outputs.
func buildInputForNode(nodeName string, w *Workflow, nodeOutputs map[string]*NodeOutput) []NodeItem {
	var items []NodeItem
	for srcName, conn := range w.Connections {
		for outputIdx, targets := range conn.Main {
			for _, target := range targets {
				if target.Node == nodeName {
					if out, ok := nodeOutputs[srcName]; ok && outputIdx < len(out.Items) {
						items = append(items, out.Items[outputIdx]...)
					}
				}
			}
		}
	}
	return items
}

// resolveNodeCredentials resolves credential references for a node.
func (e *Executor) resolveNodeCredentials(node *Node) map[string]map[string]string {
	creds := make(map[string]map[string]string)
	if e.credResolver == nil || len(node.Credentials) == 0 {
		return creds
	}
	for credType, credRef := range node.Credentials {
		if credRef.ID == "" {
			log.Printf("[executor] credential %q (%s): missing ID", credRef.Name, credType)
			continue
		}
		data, err := e.credResolver.Resolve(credRef.ID)
		if err != nil {
			log.Printf("[executor] resolve credential %q (%s): %v", credRef.ID, credType, err)
			continue
		}
		creds[credType] = data
	}
	return creds
}

// injectStaticData puts workflow staticData into node parameters for trigger nodes.
func (e *Executor) injectStaticData(w *Workflow, node *Node) {
	if w.StaticData == nil {
		return
	}
	sd, ok := w.StaticData.(map[string]any)
	if !ok {
		return
	}
	key := "node:" + node.Name
	if nodeSD, ok := sd[key].(map[string]any); ok {
		if node.Parameters == nil {
			node.Parameters = make(map[string]any)
		}
		node.Parameters["_staticData"] = nodeSD
	}
}

// cleanupNodeParams removes temporary execution params from node parameters.
func cleanupNodeParams(node *Node) {
	delete(node.Parameters, "_newLastUID")
	delete(node.Parameters, "_staticData")
}

// updateStaticData saves new staticData from trigger node execution back to the workflow.
func (e *Executor) updateStaticData(w *Workflow, node *Node) {
	newUID, ok := node.Parameters["_newLastUID"]
	if !ok {
		cleanupNodeParams(node)
		return
	}
	cleanupNodeParams(node)

	if w.StaticData == nil {
		w.StaticData = make(map[string]any)
	}
	sd, ok := w.StaticData.(map[string]any)
	if !ok {
		sd = make(map[string]any)
		w.StaticData = sd
	}
	key := "node:" + node.Name
	sd[key] = map[string]any{"lastMessageUid": newUID}
}

func isTriggerNode(node *Node) bool {
	t := node.Type
	return t == "n8n-nodes-base.emailReadImap" ||
		t == "n8n-nodes-base.webhook" ||
		t == "n8n-nodes-base.cron" ||
		t == "n8n-nodes-base.scheduleTrigger" ||
		t == "n8n-nodes-base.manualTrigger"
}

// ExecuteJSON is a helper for API/test use — serializes the execution result.
func (e *Executor) ExecuteJSON(ctx context.Context, w *Workflow) ([]byte, error) {
	exec, err := e.Execute(ctx, w)
	if err != nil {
		return nil, err
	}
	return json.Marshal(exec)
}
