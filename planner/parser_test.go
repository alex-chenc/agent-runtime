package planner

import (
	"testing"

	"github.com/chenchen511/agent-runtime/core"
)

func TestParsePlan_Valid(t *testing.T) {
	input := `{"goal":"analyze codebase","assumptions":["Go project"],"steps":[{"title":"Scan files","objective":"find all Go files","expected_output":"file list","suggested_tools":["find"],"dependencies":[],"risk_level":"read_only"}]}`
	plan, err := ParsePlan(input)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Goal != "analyze codebase" {
		t.Errorf("goal = %q", plan.Goal)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("steps count = %d, want 1", len(plan.Steps))
	}
	if plan.Steps[0].Title != "Scan files" {
		t.Errorf("step title = %q", plan.Steps[0].Title)
	}
	if plan.Steps[0].RiskLevel != core.RiskReadOnly {
		t.Errorf("risk_level = %q", plan.Steps[0].RiskLevel)
	}
}

func TestParsePlan_MultipleSteps(t *testing.T) {
	input := `{"goal":"test","steps":[{"title":"A","objective":"a","expected_output":"x"},{"title":"B","objective":"b","expected_output":"y","dependencies":["step_1"]}]}`
	plan, err := ParsePlan(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps) != 2 {
		t.Fatalf("steps count = %d, want 2", len(plan.Steps))
	}
	if len(plan.Steps[1].Dependencies) != 1 || plan.Steps[1].Dependencies[0] != "step_1" {
		t.Errorf("dependencies = %v", plan.Steps[1].Dependencies)
	}
}

func TestParsePlan_EmptyGoal(t *testing.T) {
	input := `{"goal":"","steps":[{"title":"A","objective":"a"}]}`
	_, err := ParsePlan(input)
	if err == nil {
		t.Error("expected error for empty goal")
	}
}

func TestParsePlan_NoSteps(t *testing.T) {
	input := `{"goal":"test","steps":[]}`
	_, err := ParsePlan(input)
	if err == nil {
		t.Error("expected error for no steps")
	}
}

func TestParsePlan_EmptyTitle(t *testing.T) {
	input := `{"goal":"test","steps":[{"title":"","objective":"a"}]}`
	_, err := ParsePlan(input)
	if err == nil {
		t.Error("expected error for empty title")
	}
}

func TestParsePlan_EmptyObjective(t *testing.T) {
	input := `{"goal":"test","steps":[{"title":"A","objective":""}]}`
	_, err := ParsePlan(input)
	if err == nil {
		t.Error("expected error for empty objective")
	}
}

func TestParsePlan_InvalidJSON(t *testing.T) {
	_, err := ParsePlan("not json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParsePlan_DefaultRiskLevel(t *testing.T) {
	input := `{"goal":"test","steps":[{"title":"A","objective":"a"}]}`
	plan, err := ParsePlan(input)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Steps[0].RiskLevel != core.RiskReadOnly {
		t.Errorf("default risk_level = %q, want read_only", plan.Steps[0].RiskLevel)
	}
}

func TestParsePlan_WithMarkdownWrapper(t *testing.T) {
	input := "```json\n{\"goal\":\"test\",\"steps\":[{\"title\":\"A\",\"objective\":\"a\"}]}\n```"
	plan, err := ParsePlan(input)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Goal != "test" {
		t.Errorf("goal = %q", plan.Goal)
	}
}
