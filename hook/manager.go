package hook

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chenchen511/agent-runtime/core"
	"github.com/chenchen511/agent-runtime/internal/ids"
)

// Manager dispatches hook events to registered sinks.
type Manager struct {
	mu      sync.Mutex
	sinks   []core.HookSink
	idGen   core.IDGenerator
	timeout time.Duration
}

// NewManager creates a new hook manager.
func NewManager(sinks []core.HookSink, idGen core.IDGenerator, timeout time.Duration) *Manager {
	if idGen == nil {
		idGen = ids.Generator{}
	}
	return &Manager{
		sinks:   sinks,
		idGen:   idGen,
		timeout: timeout,
	}
}

// Emit sends a hook event to all registered sinks synchronously.
func (m *Manager) Emit(ctx context.Context, event core.HookEvent) error {
	if event.EventID == "" {
		event.EventID = m.idGen.Generate()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}

	m.mu.Lock()
	sinks := make([]core.HookSink, len(m.sinks))
	copy(sinks, m.sinks)
	m.mu.Unlock()

	for _, sink := range sinks {
		hookCtx, cancel := context.WithTimeout(ctx, m.timeout)
		err := sink.Handle(hookCtx, event)
		cancel()
		if err != nil {
			return fmt.Errorf("hook %s failed: %w", event.Type, err)
		}
	}
	return nil
}

// EmitAsync sends a hook event asynchronously.
func (m *Manager) EmitAsync(ctx context.Context, event core.HookEvent) {
	go func() {
		_ = m.Emit(ctx, event)
	}()
}

// EmitType is a convenience method to emit an event with just a type and task ID.
func (m *Manager) EmitType(ctx context.Context, eventType core.HookEventType, taskID string, snapshot *core.TaskSnapshot) {
	m.EmitAsync(ctx, core.HookEvent{
		TaskID:   taskID,
		Type:     eventType,
		Snapshot: snapshot,
	})
}
