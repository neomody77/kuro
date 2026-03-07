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
