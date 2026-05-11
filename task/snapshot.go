package task

import "github.com/chenchen511/agent-runtime/core"

// Snapshot returns a read-only snapshot of the task context.
func (c *Context) Snapshot() *core.TaskSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()

	completed, failed := 0, 0
	for _, s := range c.Steps {
		switch s.Status {
		case core.StepCompleted:
			completed++
		case core.StepFailed:
			failed++
		}
	}

	// Copy recent errors (last 5)
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
		StartedAt:       c.StartedAt,
		Metadata:        metaCopy,
	}
}
