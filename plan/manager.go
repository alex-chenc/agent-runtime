package plan

import (
	"fmt"
	"sync"

	"github.com/alex-chenc/agent-runtime/core"
)

// Manager manages the plan lifecycle for a task.
type Manager struct {
	mu          sync.Mutex
	initialPlan *core.Plan
	currentPlan *core.Plan
}

// NewManager creates a new plan manager.
func NewManager() *Manager {
	return &Manager{}
}

// SetInitialPlan saves the initial plan.
func (m *Manager) SetInitialPlan(p *core.Plan) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.initialPlan = p
	m.currentPlan = p
}

// InitialPlan returns the initial plan.
func (m *Manager) InitialPlan() *core.Plan {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.initialPlan
}

// CurrentPlan returns the current plan.
func (m *Manager) CurrentPlan() *core.Plan {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.currentPlan
}

// NextExecutableStep returns the next pending or retrying step whose dependencies are all satisfied.
func (m *Manager) NextExecutableStep() *core.PlanStep {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.currentPlan == nil {
		return nil
	}
	for i := range m.currentPlan.Steps {
		step := &m.currentPlan.Steps[i]
		if step.Status != core.StepPending && step.Status != core.StepRetrying {
			continue
		}
		if m.dependenciesMet(step) {
			return step
		}
	}
	return nil
}

// dependenciesMet checks if all dependencies of a step are completed.
func (m *Manager) dependenciesMet(step *core.PlanStep) bool {
	if len(step.Dependencies) == 0 {
		return true
	}
	for _, depID := range step.Dependencies {
		found := false
		for _, s := range m.currentPlan.Steps {
			if s.StepID == depID {
				if s.Status != core.StepCompleted && s.Status != core.StepSkipped {
					return false
				}
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// UpdateStepStatus updates the status of a step by ID.
func (m *Manager) UpdateStepStatus(stepID string, status core.StepStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.currentPlan == nil {
		return fmt.Errorf("plan manager: no current plan")
	}
	for i := range m.currentPlan.Steps {
		if m.currentPlan.Steps[i].StepID == stepID {
			m.currentPlan.Steps[i].Status = status
			return nil
		}
	}
	return fmt.Errorf("plan manager: step %q not found", stepID)
}

// ResetStepForRetry resets a failed step to StepRetrying status and increments its RetryCount.
func (m *Manager) ResetStepForRetry(stepID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.currentPlan == nil {
		return fmt.Errorf("plan manager: no current plan")
	}
	for i := range m.currentPlan.Steps {
		if m.currentPlan.Steps[i].StepID == stepID {
			m.currentPlan.Steps[i].Status = core.StepRetrying
			m.currentPlan.Steps[i].RetryCount++
			return nil
		}
	}
	return fmt.Errorf("plan manager: step %q not found", stepID)
}

// AllStepsTerminal returns true if all steps are in a terminal state.
func (m *Manager) AllStepsTerminal() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.currentPlan == nil {
		return true
	}
	for _, s := range m.currentPlan.Steps {
		switch s.Status {
		case core.StepPending, core.StepRunning, core.StepWaitingTool, core.StepRetrying:
			return false
		}
	}
	return true
}

// ApplyCorrection updates the plan with correction actions.
func (m *Manager) ApplyCorrection(correction *core.CorrectionResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.currentPlan == nil {
		return fmt.Errorf("plan manager: no current plan to correct")
	}
	m.currentPlan.Version = correction.ToPlanVersion

	for _, action := range correction.Actions {
		targetID := action.TargetStepID
		if targetID == "" {
			targetID = action.StepID
		}
		switch action.Type {
		case core.CorrectionSkipStep:
			for i := range m.currentPlan.Steps {
				if m.currentPlan.Steps[i].StepID == targetID {
					m.currentPlan.Steps[i].Status = core.StepSkipped
					break
				}
			}
		case core.CorrectionReplaceStep:
			for i := range m.currentPlan.Steps {
				if m.currentPlan.Steps[i].StepID == targetID {
					m.currentPlan.Steps[i].Title = action.Reason
					m.currentPlan.Steps[i].Status = core.StepPending
					break
				}
			}
		case core.CorrectionAddStep:
			newStep := core.PlanStep{
				StepID:    action.NewStepID,
				Title:     action.Reason,
				Objective: action.Reason,
				Status:    core.StepPending,
				CreatedBy: "correction",
			}
			m.currentPlan.Steps = append(m.currentPlan.Steps, newStep)
		case core.CorrectionReduceScope:
			for i := range m.currentPlan.Steps {
				if m.currentPlan.Steps[i].StepID == targetID {
					m.currentPlan.Steps[i].Status = core.StepSkipped
					break
				}
			}
			newStep := core.PlanStep{
				StepID:    action.NewStepID,
				Title:     action.Reason,
				Objective: action.Reason,
				Status:    core.StepPending,
				CreatedBy: "correction",
			}
			m.currentPlan.Steps = append(m.currentPlan.Steps, newStep)
		}
	}
	return nil
}
