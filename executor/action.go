package executor

// StepContext provides the execution context for a step.
type StepContext struct {
	TaskID    string
	UserInput string
	PlanGoal  string
	Metadata  map[string]string
}
