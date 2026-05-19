package plan

import (
	"fmt"

	"github.com/alex-chenc/agent-runtime/core"
)

// ValidationResult contains the outcome of plan validation.
type ValidationResult struct {
	Valid  bool
	Errors []string
}

// Validator validates a plan against constraints.
type Validator struct {
	maxSteps      int
	knownTools    map[string]bool
	disabledTools map[string]bool
}

// NewValidator creates a plan validator.
func NewValidator(maxSteps int, tools []core.ToolDescriptor, disabledTools []string) *Validator {
	known := make(map[string]bool, len(tools))
	for _, t := range tools {
		known[t.Name] = true
	}
	disabled := make(map[string]bool, len(disabledTools))
	for _, d := range disabledTools {
		disabled[d] = true
	}
	return &Validator{
		maxSteps:      maxSteps,
		knownTools:    known,
		disabledTools: disabled,
	}
}

// Validate checks a plan for structural and semantic errors.
func (v *Validator) Validate(plan *core.Plan) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if plan == nil {
		result.Valid = false
		result.Errors = append(result.Errors, "plan is nil")
		return result
	}

	if len(plan.Steps) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "plan has no steps")
	}

	if len(plan.Steps) > v.maxSteps {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("plan has %d steps, max is %d", len(plan.Steps), v.maxSteps))
	}

	// Check for duplicate step IDs and titles
	ids := make(map[string]bool)
	titles := make(map[string]bool)
	for _, step := range plan.Steps {
		if step.StepID == "" {
			result.Valid = false
			result.Errors = append(result.Errors, "step has empty ID")
		}
		if ids[step.StepID] {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("duplicate step ID: %q", step.StepID))
		}
		ids[step.StepID] = true

		if step.Title == "" {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("step %q has empty title", step.StepID))
		}
		if titles[step.Title] {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("duplicate step title: %q", step.Title))
		}
		titles[step.Title] = true

		if step.Objective == "" {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("step %q has empty objective", step.StepID))
		}

		// Check tools exist
		for _, toolName := range step.SuggestedTools {
			if !v.knownTools[toolName] {
				result.Valid = false
				result.Errors = append(result.Errors, fmt.Sprintf("step %q references unknown tool %q", step.StepID, toolName))
			}
			if v.disabledTools[toolName] {
				result.Valid = false
				result.Errors = append(result.Errors, fmt.Sprintf("step %q references disabled tool %q", step.StepID, toolName))
			}
		}

		// Check dependencies exist
		for _, dep := range step.Dependencies {
			if !ids[dep] && dep != step.StepID {
				// Forward reference -- check in full steps list
				found := false
				for _, s := range plan.Steps {
					if s.StepID == dep {
						found = true
						break
					}
				}
				if !found {
					result.Valid = false
					result.Errors = append(result.Errors, fmt.Sprintf("step %q depends on non-existent step %q", step.StepID, dep))
				}
			}
		}
	}

	// Check for dependency cycles
	if v.hasCycles(plan) {
		result.Valid = false
		result.Errors = append(result.Errors, "plan has dependency cycle")
	}

	if len(result.Errors) > 0 {
		result.Valid = false
	}

	return result
}

// hasCycles checks for dependency cycles using DFS.
func (v *Validator) hasCycles(plan *core.Plan) bool {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int)
	for _, s := range plan.Steps {
		color[s.StepID] = white
	}

	var dfs func(string) bool
	dfs = func(id string) bool {
		color[id] = gray
		for _, s := range plan.Steps {
			if s.StepID == id {
				for _, dep := range s.Dependencies {
					if color[dep] == gray {
						return true
					}
					if color[dep] == white && dfs(dep) {
						return true
					}
				}
				break
			}
		}
		color[id] = black
		return false
	}

	for _, s := range plan.Steps {
		if color[s.StepID] == white {
			if dfs(s.StepID) {
				return true
			}
		}
	}
	return false
}
