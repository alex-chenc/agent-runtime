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
