package plan

import "github.com/chenchen511/agent-runtime/core"

// PlanDiff describes the differences between two plan versions.
type PlanDiff struct {
	FromVersion int               `json:"from_version"`
	ToVersion   int               `json:"to_version"`
	Added       []core.PlanStep   `json:"added,omitempty"`
	Skipped     []string          `json:"skipped,omitempty"`
	Replaced    []StepReplacement `json:"replaced,omitempty"`
	Unchanged   []string          `json:"unchanged,omitempty"`
}

// StepReplacement records a step that was replaced.
type StepReplacement struct {
	OldStepID string `json:"old_step_id"`
	NewStepID string `json:"new_step_id"`
	Reason    string `json:"reason"`
}

// ComputeDiff computes the diff between two plan versions.
func ComputeDiff(old, new *core.Plan) *PlanDiff {
	if old == nil || new == nil {
		return &PlanDiff{}
	}
	diff := &PlanDiff{
		FromVersion: old.Version,
		ToVersion:   new.Version,
	}

	oldSteps := make(map[string]core.PlanStep)
	for _, s := range old.Steps {
		oldSteps[s.StepID] = s
	}

	newSteps := make(map[string]core.PlanStep)
	for _, s := range new.Steps {
		newSteps[s.StepID] = s
	}

	for _, s := range new.Steps {
		if _, existed := oldSteps[s.StepID]; !existed {
			diff.Added = append(diff.Added, s)
		}
	}

	for _, s := range old.Steps {
		if _, exists := newSteps[s.StepID]; !exists {
			diff.Skipped = append(diff.Skipped, s.StepID)
		}
	}

	for _, s := range old.Steps {
		if newS, exists := newSteps[s.StepID]; exists {
			if s.Status != newS.Status && newS.Status == core.StepReplaced {
				diff.Replaced = append(diff.Replaced, StepReplacement{
					OldStepID: s.StepID,
					NewStepID: s.StepID,
					Reason:    newS.ChangeReason,
				})
			} else {
				diff.Unchanged = append(diff.Unchanged, s.StepID)
			}
		}
	}

	return diff
}
