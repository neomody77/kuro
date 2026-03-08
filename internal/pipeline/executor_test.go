package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type mockAction struct {
	output any
	err    error
	delay  time.Duration
	called atomic.Int32
}

func (m *mockAction) Execute(ctx context.Context, params map[string]any, creds map[string]string) (any, error) {
	m.called.Add(1)
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return m.output, m.err
}

func newExecutor() *Executor {
	return NewExecutor(nil)
}

func TestExecute_LinearChain(t *testing.T) {
	e := newExecutor()
	actionA := &mockAction{output: "result-a"}
	actionB := &mockAction{output: "result-b"}
	actionC := &mockAction{output: "result-c"}
	e.RegisterAction("action_a", actionA)
	e.RegisterAction("action_b", actionB)
	e.RegisterAction("action_c", actionC)

	w := makeWorkflow("linear",
		[]Node{
			{Name: "a", Type: "action_a"},
			{Name: "b", Type: "action_b"},
			{Name: "c", Type: "action_c"},
		},
		map[string]NodeConnection{
			"a": conn("b"),
			"b": conn("c"),
		},
	)

	exec, err := e.Execute(context.Background(), w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exec.Status != ExecSuccess {
		t.Errorf("status = %v, want %v", exec.Status, ExecSuccess)
	}
	for _, name := range []string{"a", "b", "c"} {
		runs, ok := exec.Data.ResultData.RunData[name]
		if !ok || len(runs) == 0 {
			t.Errorf("missing result for node %q", name)
		}
	}
	if actionA.called.Load() != 1 {
		t.Errorf("action_a called %d times, want 1", actionA.called.Load())
	}
}

func TestExecute_ParallelBranches(t *testing.T) {
	e := newExecutor()
	actionA := &mockAction{output: "a-out"}
	actionB := &mockAction{output: "b-out", delay: 10 * time.Millisecond}
	actionC := &mockAction{output: "c-out", delay: 10 * time.Millisecond}
	actionD := &mockAction{output: "d-out"}
	e.RegisterAction("a", actionA)
	e.RegisterAction("b", actionB)
	e.RegisterAction("c", actionC)
	e.RegisterAction("d", actionD)

	w := makeWorkflow("parallel",
		[]Node{
			{Name: "a", Type: "a"},
			{Name: "b", Type: "b"},
			{Name: "c", Type: "c"},
			{Name: "d", Type: "d"},
		},
		map[string]NodeConnection{
			"a": conn("b", "c"),
			"b": conn("d"),
			"c": conn("d"),
		},
	)

	start := time.Now()
	exec, err := e.Execute(context.Background(), w)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exec.Status != ExecSuccess {
		t.Errorf("status = %v, want %v", exec.Status, ExecSuccess)
	}
	if elapsed > 50*time.Millisecond {
		t.Logf("warning: parallel execution took %v, expected ~10ms", elapsed)
	}
}

func TestExecute_ErrorHandling(t *testing.T) {
	e := newExecutor()
	failing := &mockAction{err: errors.New("boom")}
	e.RegisterAction("fail", failing)

	w := makeWorkflow("error-test",
		[]Node{{Name: "bad", Type: "fail"}},
		nil,
	)

	exec, err := e.Execute(context.Background(), w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exec.Status != ExecError {
		t.Errorf("status = %v, want %v", exec.Status, ExecError)
	}
	runs := exec.Data.ResultData.RunData["bad"]
	if len(runs) == 0 || runs[0].Error == nil {
		t.Error("expected error in node result")
	}
}

func TestExecute_UnknownAction(t *testing.T) {
	e := newExecutor()

	w := makeWorkflow("unknown",
		[]Node{{Name: "x", Type: "nonexistent"}},
		nil,
	)

	exec, err := e.Execute(context.Background(), w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exec.Status != ExecError {
		t.Errorf("status = %v, want %v", exec.Status, ExecError)
	}
}

func TestExecute_ContextCancellation(t *testing.T) {
	e := newExecutor()
	slow := &mockAction{output: "ok", delay: 5 * time.Second}
	e.RegisterAction("slow", slow)

	w := makeWorkflow("cancel",
		[]Node{{Name: "s", Type: "slow"}},
		nil,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	exec, err := e.Execute(ctx, w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exec.Status == ExecSuccess {
		t.Errorf("expected non-success status after cancellation, got %v", exec.Status)
	}
}

func TestExecute_WithStore(t *testing.T) {
	dir := t.TempDir()
	store := NewJSONExecutionStore(dir)
	e := NewExecutor(store)
	action := &mockAction{output: "stored"}
	e.RegisterAction("act", action)

	w := makeWorkflow("stored-run",
		[]Node{{Name: "a", Type: "act"}},
		nil,
	)

	exec, err := e.Execute(context.Background(), w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loaded, err := store.GetExecution(exec.ID)
	if err != nil {
		t.Fatalf("failed to load execution: %v", err)
	}
	if loaded.Status != ExecSuccess {
		t.Errorf("loaded status = %v, want %v", loaded.Status, ExecSuccess)
	}
}

func TestExecute_DisabledNode(t *testing.T) {
	e := newExecutor()
	actionA := &mockAction{output: "a-out"}
	actionB := &mockAction{output: "b-out"}
	actionC := &mockAction{output: "c-out"}
	e.RegisterAction("action_a", actionA)
	e.RegisterAction("action_b", actionB)
	e.RegisterAction("action_c", actionC)

	w := makeWorkflow("disabled",
		[]Node{
			{Name: "a", Type: "action_a"},
			{Name: "b", Type: "action_b", Disabled: true},
			{Name: "c", Type: "action_c"},
		},
		map[string]NodeConnection{
			"a": conn("b"),
			"b": conn("c"),
		},
	)

	exec, err := e.Execute(context.Background(), w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exec.Status != ExecSuccess {
		t.Errorf("status = %v, want %v", exec.Status, ExecSuccess)
	}
	if actionA.called.Load() != 1 {
		t.Errorf("action_a called %d times, want 1", actionA.called.Load())
	}
	if actionB.called.Load() != 0 {
		t.Errorf("action_b called %d times, want 0 (disabled)", actionB.called.Load())
	}
	// Node B is disabled so no run data should exist for it.
	if _, ok := exec.Data.ResultData.RunData["b"]; ok {
		t.Error("disabled node b should not have run data")
	}
	// Node A should have run data.
	if _, ok := exec.Data.ResultData.RunData["a"]; !ok {
		t.Error("node a should have run data")
	}
}

type mockNodeHandler struct {
	output *NodeOutput
	err    error
	called atomic.Int32
	gotInput []NodeItem
}

func (m *mockNodeHandler) ExecuteNode(ctx context.Context, node *Node, input []NodeItem, creds map[string]map[string]string) (*NodeOutput, error) {
	m.called.Add(1)
	m.gotInput = input
	return m.output, m.err
}

func TestExecute_NodeHandler(t *testing.T) {
	e := newExecutor()
	handler := &mockNodeHandler{
		output: &NodeOutput{
			Items: [][]NodeItem{
				{{"key": "value"}},
			},
		},
	}
	e.RegisterNodeHandler("custom_node", handler)

	w := makeWorkflow("node-handler",
		[]Node{{Name: "n", Type: "custom_node"}},
		nil,
	)

	exec, err := e.Execute(context.Background(), w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exec.Status != ExecSuccess {
		t.Errorf("status = %v, want %v", exec.Status, ExecSuccess)
	}
	if handler.called.Load() != 1 {
		t.Errorf("handler called %d times, want 1", handler.called.Load())
	}
	runs, ok := exec.Data.ResultData.RunData["n"]
	if !ok || len(runs) == 0 {
		t.Fatal("missing run data for node n")
	}
	if runs[0].Error != nil {
		t.Errorf("unexpected error in run data: %v", runs[0].Error)
	}
}

func TestExecuteJSON(t *testing.T) {
	e := newExecutor()
	action := &mockAction{output: "json-out"}
	e.RegisterAction("act", action)

	w := makeWorkflow("json-test",
		[]Node{{Name: "a", Type: "act"}},
		nil,
	)

	data, err := e.ExecuteJSON(context.Background(), w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := result["id"]; !ok {
		t.Error("JSON missing 'id' field")
	}
	if _, ok := result["status"]; !ok {
		t.Error("JSON missing 'status' field")
	}
	if _, ok := result["workflowId"]; !ok {
		t.Error("JSON missing 'workflowId' field")
	}
	if result["status"] != string(ExecSuccess) {
		t.Errorf("status = %v, want %v", result["status"], ExecSuccess)
	}
}

func TestExecute_DataFlow(t *testing.T) {
	e := newExecutor()
	handlerA := &mockNodeHandler{
		output: &NodeOutput{
			Items: [][]NodeItem{
				{{"msg": "hello"}, {"msg": "world"}},
			},
		},
	}
	handlerB := &mockNodeHandler{
		output: &NodeOutput{
			Items: [][]NodeItem{
				{{"result": "ok"}},
			},
		},
	}
	e.RegisterNodeHandler("type_a", handlerA)
	e.RegisterNodeHandler("type_b", handlerB)

	w := makeWorkflow("data-flow",
		[]Node{
			{Name: "a", Type: "type_a"},
			{Name: "b", Type: "type_b"},
		},
		map[string]NodeConnection{
			"a": {Main: [][]ConnectionTarget{
				{{Node: "b", Type: "main", Index: 0}},
			}},
		},
	)

	exec, err := e.Execute(context.Background(), w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exec.Status != ExecSuccess {
		t.Errorf("status = %v, want %v", exec.Status, ExecSuccess)
	}
	// Verify B received A's output as input.
	if handlerB.called.Load() != 1 {
		t.Fatalf("handler B called %d times, want 1", handlerB.called.Load())
	}
	if len(handlerB.gotInput) != 2 {
		t.Fatalf("handler B got %d input items, want 2", len(handlerB.gotInput))
	}
	if handlerB.gotInput[0]["msg"] != "hello" {
		t.Errorf("first input item msg = %v, want 'hello'", handlerB.gotInput[0]["msg"])
	}
	if handlerB.gotInput[1]["msg"] != "world" {
		t.Errorf("second input item msg = %v, want 'world'", handlerB.gotInput[1]["msg"])
	}
}
