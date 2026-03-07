package pipeline

import (
	"context"
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
