package correction

import (
	"encoding/json"
	"fmt"

	"github.com/alex-chenc/agent-runtime/core"
	"github.com/alex-chenc/agent-runtime/internal/textutil"
)

type correctionJSON struct {
	Reason  string                 `json:"reason"`
	Actions []correctionActionJSON `json:"actions"`
}

type correctionActionJSON struct {
	Type         string `json:"type"`
	StepID       string `json:"step_id,omitempty"`
	NewStepID    string `json:"new_step_id,omitempty"`
	TargetStepID string `json:"target_step_id,omitempty"`
	Reason       string `json:"reason"`
}

// ParseCorrection parses LLM output into a CorrectionResult.
func ParseCorrection(content string) (*core.CorrectionResult, error) {
	jsonStr := textutil.ExtractJSON(content)
	if jsonStr == "" {
		jsonStr = content
	}

	var raw correctionJSON
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, fmt.Errorf("parse correction: %w", err)
	}

	if raw.Reason == "" {
		return nil, fmt.Errorf("correction has empty reason")
	}

	actions := make([]core.CorrectionAction, len(raw.Actions))
	for i, a := range raw.Actions {
		actions[i] = core.CorrectionAction{
			Type:         core.CorrectionActionType(a.Type),
			StepID:       a.StepID,
			NewStepID:    a.NewStepID,
			TargetStepID: a.TargetStepID,
			Reason:       a.Reason,
		}
		if actions[i].TargetStepID == "" {
			actions[i].TargetStepID = actions[i].StepID
		}
	}

	return &core.CorrectionResult{
		Actions: actions,
		Reason:  raw.Reason,
		Valid:   true,
	}, nil
}
