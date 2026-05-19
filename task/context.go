package task

import (
	"sync"
	"time"

	"github.com/chenchen511/agent-runtime/core"
)

// Context holds the mutable state for a single task execution.
// Only the main execution loop writes to it; hooks and adapters receive snapshots.
type Context struct {
	mu sync.Mutex

	TaskID     string
	Input      TaskInput
	Config     core.RuntimeConfig
	Status     core.TaskStatus
	ExitReason core.ExitReason

	ToolSnapshot  []core.ToolDescriptor
	InitialPlan   *core.Plan
	CurrentPlan   *core.Plan
	CurrentStepID string

	Counters Counters
	Timeline []core.TimelineEvent

	Steps           []core.StepExecution
	ToolCalls       []core.ToolCallRecord
	ModelCalls      []core.ModelCallRecord
	Errors          []core.RuntimeError
	Reflections     []core.ReflectionResult
	Audits          []core.AuditResult
	Corrections     []core.CorrectionResult
	ExperienceUsage []core.ExperienceUsage
	ConfigChanges   []core.ConfigChange
	FinalAnswer     string

	// Context budget tracking
	CompressionRecords    []core.CompressionRecord
	ContextBudget         *core.ContextBudgetSnapshot
	TotalPromptTokens     int
	TotalCompletionTokens int

	Interrupted     bool
	InterruptReason string
	StartedAt       time.Time
	EndedAt         time.Time
}

// SetInterrupted sets the interrupt flag. Safe for concurrent use.
func (c *Context) SetInterrupted(reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Interrupted = true
	c.InterruptReason = reason
}

// IsInterrupted returns whether the task has been interrupted.
func (c *Context) IsInterrupted() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Interrupted
}

// SetStatus updates the task status. Safe for concurrent use.
func (c *Context) SetStatus(status core.TaskStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Status = status
}

// ConfigSnapshot returns the current runtime config. Safe for concurrent use.
func (c *Context) ConfigSnapshot() core.RuntimeConfig {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Config
}

// Lock acquires the context mutex. Must be paired with Unlock.
func (c *Context) Lock() {
	c.mu.Lock()
}

// Unlock releases the context mutex.
func (c *Context) Unlock() {
	c.mu.Unlock()
}

// SetConfigField updates a single config field for ConfigChange tracking.
func (c *Context) SetConfigField(field, oldVal, newVal string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ConfigChanges = append(c.ConfigChanges, core.ConfigChange{
		Field:      field,
		OldValue:   oldVal,
		NewValue:   newVal,
		OccurredAt: time.Now(),
	})
}

// BatchUpdate atomically mutates fields and returns a snapshot.
// The update function is called while holding the lock.
// Call SnapshotCopy inside update to capture a consistent snapshot.
func (c *Context) BatchUpdate(update func()) *core.TaskSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	update()
	return c.snapshotLocked()
}

// snapshotLocked returns a snapshot. Must be called while holding c.mu.
func (c *Context) snapshotLocked() *core.TaskSnapshot {
	completed, failed := 0, 0
	for _, s := range c.Steps {
		switch s.Status {
		case core.StepCompleted:
			completed++
		case core.StepFailed:
			failed++
		}
	}

	recentErrs := c.Errors
	if len(recentErrs) > 5 {
		recentErrs = recentErrs[len(recentErrs)-5:]
	}
	errCopy := make([]core.RuntimeError, len(recentErrs))
	copy(errCopy, recentErrs)

	var planCopy *core.Plan
	if c.CurrentPlan != nil {
		p := *c.CurrentPlan
		p.Steps = make([]core.PlanStep, len(c.CurrentPlan.Steps))
		copy(p.Steps, c.CurrentPlan.Steps)
		planCopy = &p
	}

	metaCopy := make(map[string]string)
	for k, v := range c.Input.Metadata {
		metaCopy[k] = v
	}

	return &core.TaskSnapshot{
		TaskID:          c.TaskID,
		UserInput:       c.Input.UserInput,
		Status:          c.Status,
		ExitReason:      c.ExitReason,
		CurrentPlan:     planCopy,
		CurrentStepID:   c.CurrentStepID,
		CompletedSteps:  completed,
		FailedSteps:     failed,
		TotalToolCalls:  c.Counters.ToolCalls,
		TotalModelCalls: c.Counters.ModelCalls,
		RecentErrors:    errCopy,
		ContextBudget:   c.ContextBudget,
		StartedAt:       c.StartedAt,
		Metadata:        metaCopy,
	}
}
