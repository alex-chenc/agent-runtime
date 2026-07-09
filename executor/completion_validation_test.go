package executor

import (
	"testing"

	"github.com/alex-chenc/agent-runtime/core"
)

func TestValidateStepCompletionRejectsFailedToolCall(t *testing.T) {
	step := &core.PlanStep{
		StepID:       "step-1",
		AllowedTools: []string{"Example.Execute"},
	}
	action := core.ReactAction{
		Type:       core.ActionStepResult,
		StepResult: "completed",
		Evidence:   []string{"call-failed"},
	}
	calls := []core.ToolCallRecord{{
		CallID:   "call-failed",
		StepID:   step.StepID,
		ToolName: "Example.Execute",
		Status:   core.ToolCallFailed,
	}}

	result := validateStepCompletion(step, action, calls)
	if result.Passed {
		t.Fatal("failed tool call must not satisfy step completion")
	}
	if result.Reason == "" {
		t.Fatal("completion rejection must include a reason")
	}
}

func TestValidateStepCompletionRejectsNonTerminalAcceptedOutcome(t *testing.T) {
	step := &core.PlanStep{
		StepID:       "step-1",
		AllowedTools: []string{"Example.Generate"},
	}
	action := core.ReactAction{
		Type:       core.ActionStepResult,
		StepResult: "generated",
		Evidence:   []string{"call-accepted"},
	}
	calls := []core.ToolCallRecord{{
		CallID:   "call-accepted",
		StepID:   step.StepID,
		ToolName: "Example.Generate",
		Status:   core.ToolCallSuccess,
		Outcome: &core.ToolOutcome{
			OperationStatus: core.OperationAccepted,
			Terminal:        false,
		},
	}}

	result := validateStepCompletion(step, action, calls)
	if result.Passed {
		t.Fatal("accepted asynchronous operation must not satisfy step completion")
	}
}

func TestValidateStepCompletionAcceptsTerminalEvidenceCall(t *testing.T) {
	step := &core.PlanStep{
		StepID:       "step-1",
		AllowedTools: []string{"Example.Generate", "Example.Status"},
	}
	action := core.ReactAction{
		Type:       core.ActionStepResult,
		StepResult: "generated",
		Evidence:   []string{"call-status"},
	}
	calls := []core.ToolCallRecord{
		{
			CallID:   "call-generate",
			StepID:   step.StepID,
			ToolName: "Example.Generate",
			Status:   core.ToolCallSuccess,
			Outcome: &core.ToolOutcome{
				OperationStatus: core.OperationAccepted,
				Terminal:        false,
			},
		},
		{
			CallID:   "call-status",
			StepID:   step.StepID,
			ToolName: "Example.Status",
			Status:   core.ToolCallSuccess,
			Outcome: &core.ToolOutcome{
				OperationStatus: core.OperationSucceeded,
				Terminal:        true,
			},
		},
	}

	result := validateStepCompletion(step, action, calls)
	if !result.Passed {
		t.Fatalf("terminal evidence should satisfy completion: %s", result.Reason)
	}
}

func TestValidateStepCompletionRejectsToolStepWithoutToolEvidence(t *testing.T) {
	step := &core.PlanStep{
		StepID:       "step-1",
		AllowedTools: []string{"Example.Execute"},
	}
	action := core.ReactAction{
		Type:       core.ActionStepResult,
		StepResult: "completed",
	}

	result := validateStepCompletion(step, action, nil)
	if result.Passed {
		t.Fatal("tool-backed step must not complete without a tool call")
	}
}
