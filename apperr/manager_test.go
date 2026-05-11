package apperr

import (
	"testing"

	"github.com/chenchen511/agent-runtime/core"
)

func TestManager_Add(t *testing.T) {
	m := NewManager(nil)
	m.Add(core.RuntimeError{Kind: core.ErrToolCall, Message: "test error"})
	if len(m.Errors()) != 1 {
		t.Errorf("errors count = %d, want 1", len(m.Errors()))
	}
	if m.Errors()[0].ErrorID == "" {
		t.Error("error ID should be auto-assigned")
	}
}

func TestManager_Add_PreservesID(t *testing.T) {
	m := NewManager(nil)
	m.Add(core.RuntimeError{ErrorID: "custom-id", Kind: core.ErrToolCall, Message: "test"})
	if m.Errors()[0].ErrorID != "custom-id" {
		t.Errorf("error ID = %q, want custom-id", m.Errors()[0].ErrorID)
	}
}

func TestManager_Count(t *testing.T) {
	m := NewManager(nil)
	m.Add(core.RuntimeError{Kind: core.ErrToolCall, Message: "a"})
	m.Add(core.RuntimeError{Kind: core.ErrToolCall, Message: "b"})
	m.Add(core.RuntimeError{Kind: core.ErrModelCall, Message: "c"})
	if m.Count(core.ErrToolCall) != 2 {
		t.Errorf("tool call count = %d, want 2", m.Count(core.ErrToolCall))
	}
	if m.Count(core.ErrModelCall) != 1 {
		t.Errorf("model call count = %d, want 1", m.Count(core.ErrModelCall))
	}
}

func TestManager_Recent(t *testing.T) {
	m := NewManager(nil)
	for i := 0; i < 10; i++ {
		m.Add(core.RuntimeError{Kind: core.ErrSystem, Message: "err"})
	}
	recent := m.Recent(3)
	if len(recent) != 3 {
		t.Errorf("recent count = %d, want 3", len(recent))
	}
}

func TestManager_Recent_All(t *testing.T) {
	m := NewManager(nil)
	m.Add(core.RuntimeError{Kind: core.ErrSystem, Message: "a"})
	recent := m.Recent(5)
	if len(recent) != 1 {
		t.Errorf("recent count = %d, want 1", len(recent))
	}
}

func TestNew(t *testing.T) {
	e := New(core.ErrToolCall, "react", "task-1", "step-1", "something failed")
	if e.Kind != core.ErrToolCall {
		t.Errorf("kind = %q", e.Kind)
	}
	if e.Stage != "react" {
		t.Errorf("stage = %q", e.Stage)
	}
}

func TestRecoverable(t *testing.T) {
	e := Recoverable(core.ErrModelParse, "react", "t", "s", "parse failed")
	if !e.Recoverable {
		t.Error("should be recoverable")
	}
}

func TestWithCause(t *testing.T) {
	e := New(core.ErrSystem, "test", "t", "s", "err")
	e = WithCause(e, "root cause")
	if e.Cause != "root cause" {
		t.Errorf("cause = %q", e.Cause)
	}
}
