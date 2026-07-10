package planner

import (
	"testing"

	"github.com/alex-chenc/agent-runtime/core"
)

func TestNormalizePlanStepRiskLevelsUsesRuntimeDescriptors(t *testing.T) {
	plan := &core.Plan{Steps: []core.PlanStep{
		{StepID: "read", SuggestedTools: []string{"Host.List"}, RiskLevel: core.RiskHigh},
		{StepID: "write", SuggestedTools: []string{"Host.List", "Task.Execute"}, RiskLevel: core.RiskReadOnly},
	}}
	normalizePlanStepRiskLevels(plan, []core.ToolDescriptor{
		{Name: "Host.List", RiskLevel: core.RiskReadOnly},
		{Name: "Task.Execute", RiskLevel: core.RiskHigh},
	})

	if got := plan.Steps[0].RiskLevel; got != core.RiskReadOnly {
		t.Fatalf("read step risk = %q, want read_only", got)
	}
	if got := plan.Steps[1].RiskLevel; got != core.RiskHigh {
		t.Fatalf("write step risk = %q, want high", got)
	}
}

func TestApplyGeneratedStepToolBoundariesKeepsOnlyRegisteredSuggestions(t *testing.T) {
	plan := &core.Plan{Steps: []core.PlanStep{{
		StepID:         "execute",
		SuggestedTools: []string{"Host.List", "Task.Create", "Host.List", "Vulnerability.Script.Execute"},
	}}}
	applyGeneratedStepToolBoundaries(plan, []core.ToolDescriptor{
		{Name: "Host.List"},
		{Name: "Vulnerability.Script.Execute"},
	})

	got := plan.Steps[0].AllowedTools
	if len(got) != 2 || got[0] != "Host.List" || got[1] != "Vulnerability.Script.Execute" {
		t.Fatalf("allowed tools = %#v, want registered unique suggestions", got)
	}
}
