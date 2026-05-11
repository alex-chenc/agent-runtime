package hook

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/chenchen511/agent-runtime/core"
)

type recordingSink struct {
	mu     sync.Mutex
	events []core.HookEvent
}

type errorSink struct{}

func (errorSink) Handle(context.Context, core.HookEvent) error {
	return errors.New("sink failed")
}

func (s *recordingSink) Handle(_ context.Context, event core.HookEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return nil
}

func (s *recordingSink) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.events)
}

func TestManager_Emit(t *testing.T) {
	sink := &recordingSink{}
	m := NewManager([]core.HookSink{sink}, nil, 5*time.Second)
	err := m.Emit(context.Background(), core.HookEvent{
		TaskID: "task-1",
		Type:   core.HookTaskStarted,
	})
	if err != nil {
		t.Fatal(err)
	}
	if sink.count() != 1 {
		t.Errorf("event count = %d, want 1", sink.count())
	}
	if sink.events[0].TaskID != "task-1" {
		t.Errorf("task_id = %q", sink.events[0].TaskID)
	}
	if sink.events[0].EventID == "" {
		t.Error("event_id should be auto-assigned")
	}
}

func TestManager_Emit_AutoID(t *testing.T) {
	sink := &recordingSink{}
	m := NewManager([]core.HookSink{sink}, nil, 5*time.Second)
	_ = m.Emit(context.Background(), core.HookEvent{TaskID: "t", Type: core.HookTaskStarted})
	_ = m.Emit(context.Background(), core.HookEvent{TaskID: "t", Type: core.HookTaskFinished})
	if sink.events[0].EventID == sink.events[1].EventID {
		t.Error("auto-generated event IDs should be unique")
	}
}

func TestManager_EmitType(t *testing.T) {
	sink := &recordingSink{}
	m := NewManager([]core.HookSink{sink}, nil, 5*time.Second)
	m.EmitType(context.Background(), core.HookStepCompleted, "task-1", nil)
	time.Sleep(50 * time.Millisecond)
	if sink.count() != 1 {
		t.Errorf("event count = %d, want 1", sink.count())
	}
	if sink.events[0].Type != core.HookStepCompleted {
		t.Errorf("type = %q", sink.events[0].Type)
	}
}

func TestManager_MultipleSinks(t *testing.T) {
	sink1 := &recordingSink{}
	sink2 := &recordingSink{}
	m := NewManager([]core.HookSink{sink1, sink2}, nil, 5*time.Second)
	_ = m.Emit(context.Background(), core.HookEvent{TaskID: "t", Type: core.HookTaskStarted})
	if sink1.count() != 1 {
		t.Errorf("sink1 count = %d", sink1.count())
	}
	if sink2.count() != 1 {
		t.Errorf("sink2 count = %d", sink2.count())
	}
}

func TestManager_NoSinks(t *testing.T) {
	m := NewManager(nil, nil, 5*time.Second)
	err := m.Emit(context.Background(), core.HookEvent{TaskID: "t", Type: core.HookTaskStarted})
	if err != nil {
		t.Errorf("emit with no sinks should not error: %v", err)
	}
}

func TestManager_Emit_ReturnsSinkError(t *testing.T) {
	m := NewManager([]core.HookSink{errorSink{}}, nil, 5*time.Second)
	err := m.Emit(context.Background(), core.HookEvent{TaskID: "t", Type: core.HookTaskFinished})
	if err == nil {
		t.Fatal("expected sink error")
	}
}
