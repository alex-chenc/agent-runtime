package executor

import "github.com/alex-chenc/agent-runtime/core"

// StepContext provides the execution context for a step.
type StepContext struct {
	TaskID        string
	UserInput     string
	PlanGoal      string
	Metadata      map[string]string
	PreviousSteps []core.StepExecution
}
