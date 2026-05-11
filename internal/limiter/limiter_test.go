package limiter

import (
	"testing"
)

func TestLimiter_ZeroState(t *testing.T) {
	var l Limiter
	checks := []struct {
		name string
		got  int
	}{
		{"ToolCalls", l.ToolCalls()},
		{"ToolFailures", l.ToolFailures()},
		{"ModelCalls", l.ModelCalls()},
		{"ModelFailures", l.ModelFailures()},
		{"ParseFailures", l.ParseFailures()},
		{"NoProgressTurns", l.NoProgressTurns()},
		{"TotalTurns", l.TotalTurns()},
		{"Audits", l.Audits()},
		{"Reflections", l.Reflections()},
		{"Corrections", l.Corrections()},
	}
	for _, c := range checks {
		if c.got != 0 {
			t.Errorf("%s = %d, want 0", c.name, c.got)
		}
	}
}

func TestLimiter_IncrementAndGet(t *testing.T) {
	var l Limiter
	l.IncrToolCalls()
	l.IncrToolCalls()
	if l.ToolCalls() != 2 {
		t.Errorf("ToolCalls = %d, want 2", l.ToolCalls())
	}
	l.IncrToolFailures()
	if l.ToolFailures() != 1 {
		t.Errorf("ToolFailures = %d, want 1", l.ToolFailures())
	}
	l.IncrModelCalls()
	if l.ModelCalls() != 1 {
		t.Errorf("ModelCalls = %d, want 1", l.ModelCalls())
	}
	l.IncrModelFailures()
	if l.ModelFailures() != 1 {
		t.Errorf("ModelFailures = %d, want 1", l.ModelFailures())
	}
	l.IncrParseFailures()
	if l.ParseFailures() != 1 {
		t.Errorf("ParseFailures = %d, want 1", l.ParseFailures())
	}
	l.IncrNoProgress()
	if l.NoProgressTurns() != 1 {
		t.Errorf("NoProgressTurns = %d, want 1", l.NoProgressTurns())
	}
	l.IncrTotalTurns()
	if l.TotalTurns() != 1 {
		t.Errorf("TotalTurns = %d, want 1", l.TotalTurns())
	}
	l.IncrAudits()
	if l.Audits() != 1 {
		t.Errorf("Audits = %d, want 1", l.Audits())
	}
	l.IncrReflections()
	if l.Reflections() != 1 {
		t.Errorf("Reflections = %d, want 1", l.Reflections())
	}
	l.IncrCorrections()
	if l.Corrections() != 1 {
		t.Errorf("Corrections = %d, want 1", l.Corrections())
	}
}

func TestLimiter_ResetParseFailures(t *testing.T) {
	var l Limiter
	l.IncrParseFailures()
	l.IncrParseFailures()
	l.ResetParseFailures()
	if l.ParseFailures() != 0 {
		t.Errorf("ParseFailures after reset = %d, want 0", l.ParseFailures())
	}
}

func TestLimiter_ResetNoProgress(t *testing.T) {
	var l Limiter
	l.IncrNoProgress()
	l.IncrNoProgress()
	l.IncrNoProgress()
	l.ResetNoProgress()
	if l.NoProgressTurns() != 0 {
		t.Errorf("NoProgressTurns after reset = %d, want 0", l.NoProgressTurns())
	}
}

func TestLimiter_ExceedsToolCalls_BelowLimit(t *testing.T) {
	var l Limiter
	l.IncrToolCalls()
	if l.ExceedsToolCalls(2) {
		t.Error("ExceedsToolCalls(2) should be false with count=1")
	}
}

func TestLimiter_ExceedsToolCalls_AtLimit(t *testing.T) {
	var l Limiter
	l.IncrToolCalls()
	l.IncrToolCalls()
	if !l.ExceedsToolCalls(2) {
		t.Error("ExceedsToolCalls(2) should be true with count=2")
	}
}

func TestLimiter_Exceeds_ZeroMax(t *testing.T) {
	var l Limiter
	l.IncrToolCalls()
	if !l.ExceedsToolCalls(0) {
		t.Error("ExceedsToolCalls(0) should be true with any nonzero count")
	}
}

func TestLimiter_Exceeds_AllCounters(t *testing.T) {
	var l Limiter
	l.IncrToolFailures()
	l.IncrModelFailures()
	l.IncrParseFailures()
	l.IncrNoProgress()
	l.IncrTotalTurns()
	l.IncrAudits()
	l.IncrReflections()
	l.IncrCorrections()

	if !l.ExceedsToolFailures(1) {
		t.Error("ExceedsToolFailures")
	}
	if !l.ExceedsModelFailures(1) {
		t.Error("ExceedsModelFailures")
	}
	if !l.ExceedsParseFailures(1) {
		t.Error("ExceedsParseFailures")
	}
	if !l.ExceedsNoProgress(1) {
		t.Error("ExceedsNoProgress")
	}
	if !l.ExceedsTotalTurns(1) {
		t.Error("ExceedsTotalTurns")
	}
	if !l.ExceedsAudits(1) {
		t.Error("ExceedsAudits")
	}
	if !l.ExceedsReflections(1) {
		t.Error("ExceedsReflections")
	}
	if !l.ExceedsCorrections(1) {
		t.Error("ExceedsCorrections")
	}
}

func TestLimiter_Exceeds_ZeroState(t *testing.T) {
	var l Limiter
	if l.ExceedsToolCalls(1) {
		t.Error("ExceedsToolCalls(1) should be false with count=0")
	}
	if l.ExceedsTotalTurns(5) {
		t.Error("ExceedsTotalTurns(5) should be false with count=0")
	}
}
