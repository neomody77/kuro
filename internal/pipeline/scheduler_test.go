package pipeline

import (
	"testing"
	"time"
)

func TestNextCronTime_EveryMinute(t *testing.T) {
	// "* * * * *" means every minute.
	now := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	next := nextCronTime("* * * * *", now)
	expected := time.Date(2025, 6, 15, 10, 31, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("got %v, want %v", next, expected)
	}
}

func TestNextCronTime_SpecificTime(t *testing.T) {
	// "0 9 * * *" means daily at 09:00.
	now := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	next := nextCronTime("0 9 * * *", now)
	expected := time.Date(2025, 6, 16, 9, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("got %v, want %v", next, expected)
	}
}

func TestNextCronTime_BeforeTargetTime(t *testing.T) {
	// "30 14 * * *" means daily at 14:30.
	now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	next := nextCronTime("30 14 * * *", now)
	expected := time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("got %v, want %v", next, expected)
	}
}

func TestNextCronTime_SpecificDOW(t *testing.T) {
	// "0 8 * * 1" means every Monday at 08:00.
	// June 15, 2025 is a Sunday. Next Monday is June 16.
	now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	next := nextCronTime("0 8 * * 1", now)
	expected := time.Date(2025, 6, 16, 8, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("got %v, want %v", next, expected)
	}
}

func TestNextCronTime_InvalidExpr(t *testing.T) {
	now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	next := nextCronTime("bad", now)
	// Should fall back to now + 1 hour.
	expected := now.Add(time.Hour)
	if !next.Equal(expected) {
		t.Errorf("got %v, want %v", next, expected)
	}
}

func TestParseCronField_Wildcard(t *testing.T) {
	result := parseCronField("*", 0, 59)
	if result != nil {
		t.Errorf("wildcard should return nil, got %v", result)
	}
}

func TestParseCronField_SingleValue(t *testing.T) {
	result := parseCronField("30", 0, 59)
	if len(result) != 1 || result[0] != 30 {
		t.Errorf("got %v, want [30]", result)
	}
}

func TestParseCronField_Range(t *testing.T) {
	result := parseCronField("1-3", 0, 59)
	if len(result) != 3 || result[0] != 1 || result[1] != 2 || result[2] != 3 {
		t.Errorf("got %v, want [1 2 3]", result)
	}
}

func TestParseCronField_CommaSeparated(t *testing.T) {
	result := parseCronField("0,15,30,45", 0, 59)
	if len(result) != 4 {
		t.Errorf("got %d values, want 4: %v", len(result), result)
	}
}

func TestParseCronField_OutOfRange(t *testing.T) {
	result := parseCronField("99", 0, 59)
	if len(result) != 0 {
		t.Errorf("expected empty for out-of-range value, got %v", result)
	}
}

func TestMatchesCron(t *testing.T) {
	// 2025-06-15 10:30 is a Sunday (DOW=0).
	tm := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

	if !matchesCron(tm, []int{30}, []int{10}, nil, nil, nil) {
		t.Error("should match minute=30, hour=10, rest wildcard")
	}
	if matchesCron(tm, []int{0}, []int{10}, nil, nil, nil) {
		t.Error("should not match minute=0")
	}
	if !matchesCron(tm, nil, nil, nil, nil, []int{0}) {
		t.Error("should match DOW=0 (Sunday)")
	}
	if matchesCron(tm, nil, nil, nil, nil, []int{1}) {
		t.Error("should not match DOW=1 (Monday)")
	}
}

func TestScheduler_StartStop(t *testing.T) {
	executor := NewExecutor(nil)
	s := NewScheduler(executor, nil)
	s.Start()
	s.Stop()
	// If we reach here without panic, the test passes.
}

func TestScheduler_RegisterUnregister(t *testing.T) {
	executor := NewExecutor(nil)
	s := NewScheduler(executor, nil)
	s.Start()
	defer s.Stop()

	p := &Pipeline{
		ID:   "wf-1",
		Name: "test-workflow",
		Nodes: []Node{
			{Name: "trigger", Type: "n8n-nodes-base.manualTrigger"},
		},
	}

	s.Register(p)

	// Verify entry exists.
	s.mu.Lock()
	_, exists := s.entries[p.ID]
	s.mu.Unlock()
	if !exists {
		t.Fatal("expected workflow to be registered")
	}

	s.Unregister(p.ID)

	s.mu.Lock()
	_, exists = s.entries[p.ID]
	s.mu.Unlock()
	if exists {
		t.Fatal("expected workflow to be unregistered")
	}
}

func TestScheduler_TriggerManual(t *testing.T) {
	executor := NewExecutor(nil)
	done := make(chan struct{})
	action := &mockAction{output: "manual-ok"}
	executor.RegisterAction("act", action)

	s := NewScheduler(executor, nil)
	s.Start()
	defer s.Stop()

	p := &Pipeline{
		ID:   "wf-manual",
		Name: "manual-test",
		Nodes: []Node{
			{Name: "a", Type: "act"},
		},
	}

	go func() {
		// Poll until the action has been called.
		for i := 0; i < 100; i++ {
			if action.called.Load() > 0 {
				close(done)
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	s.TriggerManual(p)

	select {
	case <-done:
		// success
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for manual trigger execution")
	}
}

func TestParseCronInterval_ScheduleTrigger(t *testing.T) {
	params := map[string]any{
		"rule": map[string]any{
			"interval": []any{
				map[string]any{"field": "minutes", "minutesInterval": float64(5)},
			},
		},
	}
	got := parseCronInterval(params)
	want := 5 * time.Minute
	if got != want {
		t.Errorf("parseCronInterval() = %v, want %v", got, want)
	}
}

func TestParseCronInterval_EveryMinute(t *testing.T) {
	params := map[string]any{
		"triggerTimes": map[string]any{
			"item": []any{
				map[string]any{"mode": "everyMinute"},
			},
		},
	}
	got := parseCronInterval(params)
	want := time.Minute
	if got != want {
		t.Errorf("parseCronInterval() = %v, want %v", got, want)
	}
}

func TestParseCronInterval_Default(t *testing.T) {
	params := map[string]any{}
	got := parseCronInterval(params)
	want := 60 * time.Second
	if got != want {
		t.Errorf("parseCronInterval() = %v, want %v", got, want)
	}
}
