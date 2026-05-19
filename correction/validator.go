package correction

import (
	"fmt"

	"github.com/alex-chenc/agent-runtime/core"
)

// ValidateCorrection checks if a correction is safe to apply.
func ValidateCorrection(correction *core.CorrectionResult, plan *core.Plan, completedIDs map[string]bool) *CorrectionValidation {
	result := &CorrectionValidation{Valid: true}

	if correction == nil {
		result.Valid = false
		result.Errors = append(result.Errors, "correction is nil")
		return result
	}

	for _, action := range correction.Actions {
		targetID := action.TargetStepID
		if targetID == "" {
			targetID = action.StepID
		}
		switch action.Type {
		case core.CorrectionSkipStep, core.CorrectionReplaceStep:
			if targetID == "" {
				result.Valid = false
				result.Errors = append(result.Errors, fmt.Sprintf("%s action missing target step ID", action.Type))
				continue
			}
			if completedIDs[targetID] {
				result.Valid = false
				result.Errors = append(result.Errors, fmt.Sprintf("cannot modify completed step %q", targetID))
			}
		case core.CorrectionAddStep:
			// Allowed if AllowDynamicNewSteps is true (checked by caller)
		default:
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("unsupported correction action %q", action.Type))
		}
	}

	return result
}

// CorrectionValidation contains the result of correction validation.
type CorrectionValidation struct {
	Valid  bool
	Errors []string
}
