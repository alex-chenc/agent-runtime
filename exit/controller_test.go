package exit

import (
	"testing"

	"github.com/alex-chenc/agent-runtime/core"
	"github.com/alex-chenc/agent-runtime/internal/limiter"
)

func newLimiter() *limiter.Limiter {
	return &limiter.Limiter{}
}

func TestCheck_NotInterrupted(t *testing.T) {
	c := NewController(core.DefaultConfig())
	decision := c.Check(false, newLimiter())
	if decision.ShouldExit {
		t.Error("should not exit when not interrupted")
	}
}

func TestCheck_Interrupted(t *testing.T) {
	c := NewController(core.DefaultConfig())
	decision := c.Check(true, newLimiter())
	if !decision.ShouldExit {
		t.Error("should exit when interrupted")
	}
	if decision.Reason != core.ExitUserInterrupted {
		t.Errorf("reason = %q, want %q", decision.Reason, core.ExitUserInterrupted)
	}
}

func TestCheck_MaxTotalTurns(t *testing.T) {
	cfg := core.DefaultConfig()
	cfg.MaxTotalTurns = 5
	c := NewController(cfg)
	l := newLimiter()
	for i := 0; i < 6; i++ {
		l.IncrTotalTurns()
	}
	decision := c.Check(false, l)
	if !decision.ShouldExit {
		t.Error("should exit when max total turns exceeded")
	}
	if decision.Reason != core.ExitMaxTotalTurns {
		t.Errorf("reason = %q, want %q", decision.Reason, core.ExitMaxTotalTurns)
	}
}

func TestCheck_MaxToolCalls(t *testing.T) {
	cfg := core.DefaultConfig()
	cfg.MaxToolCalls = 3
	c := NewController(cfg)
	l := newLimiter()
	for i := 0; i < 4; i++ {
		l.IncrToolCalls()
	}
	decision := c.Check(false, l)
	if !decision.ShouldExit {
		t.Error("should exit when max tool calls exceeded")
	}
	if decision.Reason != core.ExitMaxToolCalls {
		t.Errorf("reason = %q, want %q", decision.Reason, core.ExitMaxToolCalls)
	}
}

func TestCheck_MaxToolFailures(t *testing.T) {
	cfg := core.DefaultConfig()
	cfg.MaxToolFailures = 2
	c := NewController(cfg)
	l := newLimiter()
	for i := 0; i < 3; i++ {
		l.IncrToolFailures()
	}
	decision := c.Check(false, l)
	if !decision.ShouldExit {
		t.Error("should exit when max tool failures exceeded")
	}
	if decision.Reason != core.ExitMaxToolFailures {
		t.Errorf("reason = %q, want %q", decision.Reason, core.ExitMaxToolFailures)
	}
}

func TestCheck_MaxParseFailures(t *testing.T) {
	cfg := core.DefaultConfig()
	cfg.MaxParseFailures = 2
	c := NewController(cfg)
	l := newLimiter()
	for i := 0; i < 3; i++ {
		l.IncrParseFailures()
	}
	decision := c.Check(false, l)
	if !decision.ShouldExit {
		t.Error("should exit when max parse failures exceeded")
	}
	if decision.Reason != core.ExitMaxParseFailures {
		t.Errorf("reason = %q, want %q", decision.Reason, core.ExitMaxParseFailures)
	}
}

func TestCheck_WithinLimits(t *testing.T) {
	cfg := core.DefaultConfig()
	c := NewController(cfg)
	l := newLimiter()
	l.IncrTotalTurns()
	l.IncrToolCalls()
	l.IncrToolFailures()
	decision := c.Check(false, l)
	if decision.ShouldExit {
		t.Error("should not exit when within limits")
	}
}

func TestCheckStep_Exceeded(t *testing.T) {
	cfg := core.DefaultConfig()
	cfg.MaxToolCallsPerStep = 3
	c := NewController(cfg)
	decision := c.CheckStep(3)
	if !decision.ShouldExit {
		t.Error("should exit when max tool calls per step reached")
	}
}

func TestCheckStep_WithinLimit(t *testing.T) {
	cfg := core.DefaultConfig()
	cfg.MaxToolCallsPerStep = 4
	c := NewController(cfg)
	decision := c.CheckStep(2)
	if decision.ShouldExit {
		t.Error("should not exit when within per-step limit")
	}
}
