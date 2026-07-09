package plan

import (
	"testing"

	"github.com/alex-chenc/agent-runtime/core"
)

func TestValidator_ValidPlan(t *testing.T) {
	v := NewValidator(10, nil, nil)
	plan := &core.Plan{
		Goal: "test",
		Steps: []core.PlanStep{
			{StepID: "step_1", Title: "A", Objective: "do A"},
			{StepID: "step_2", Title: "B", Objective: "do B", Dependencies: []string{"step_1"}},
		},
	}
	result := v.Validate(plan)
	if !result.Valid {
		t.Errorf("expected valid plan, got errors: %v", result.Errors)
	}
}

func TestValidator_NilPlan(t *testing.T) {
	v := NewValidator(10, nil, nil)
	result := v.Validate(nil)
	if result.Valid {
		t.Error("nil plan should be invalid")
	}
}

func TestValidator_NoSteps(t *testing.T) {
	v := NewValidator(10, nil, nil)
	result := v.Validate(&core.Plan{Goal: "test"})
	if result.Valid {
		t.Error("empty plan should be invalid")
	}
}

func TestValidator_ExceedsMaxSteps(t *testing.T) {
	v := NewValidator(2, nil, nil)
	plan := &core.Plan{
		Goal: "test",
		Steps: []core.PlanStep{
			{StepID: "s1", Title: "A", Objective: "a"},
			{StepID: "s2", Title: "B", Objective: "b"},
			{StepID: "s3", Title: "C", Objective: "c"},
		},
	}
	result := v.Validate(plan)
	if result.Valid {
		t.Error("plan exceeding max steps should be invalid")
	}
}

func TestValidator_DuplicateStepID(t *testing.T) {
	v := NewValidator(10, nil, nil)
	plan := &core.Plan{
		Goal: "test",
		Steps: []core.PlanStep{
			{StepID: "s1", Title: "A", Objective: "a"},
			{StepID: "s1", Title: "B", Objective: "b"},
		},
	}
	result := v.Validate(plan)
	if result.Valid {
		t.Error("duplicate step ID should be invalid")
	}
}

func TestValidator_EmptyStepID(t *testing.T) {
	v := NewValidator(10, nil, nil)
	plan := &core.Plan{
		Goal: "test",
		Steps: []core.PlanStep{
			{StepID: "", Title: "A", Objective: "a"},
		},
	}
	result := v.Validate(plan)
	if result.Valid {
		t.Error("empty step ID should be invalid")
	}
}

func TestValidator_CycleDetection(t *testing.T) {
	v := NewValidator(10, nil, nil)
	plan := &core.Plan{
		Goal: "test",
		Steps: []core.PlanStep{
			{StepID: "s1", Title: "A", Objective: "a", Dependencies: []string{"s2"}},
			{StepID: "s2", Title: "B", Objective: "b", Dependencies: []string{"s1"}},
		},
	}
	result := v.Validate(plan)
	if result.Valid {
		t.Error("cyclic plan should be invalid")
	}
}

func TestValidator_CycleDetection_ThreeNodes(t *testing.T) {
	v := NewValidator(10, nil, nil)
	plan := &core.Plan{
		Goal: "test",
		Steps: []core.PlanStep{
			{StepID: "s1", Title: "A", Objective: "a", Dependencies: []string{"s3"}},
			{StepID: "s2", Title: "B", Objective: "b", Dependencies: []string{"s1"}},
			{StepID: "s3", Title: "C", Objective: "c", Dependencies: []string{"s2"}},
		},
	}
	result := v.Validate(plan)
	if result.Valid {
		t.Error("3-node cycle should be detected")
	}
}

func TestValidator_NoCycle_LinearDeps(t *testing.T) {
	v := NewValidator(10, nil, nil)
	plan := &core.Plan{
		Goal: "test",
		Steps: []core.PlanStep{
			{StepID: "s1", Title: "A", Objective: "a"},
			{StepID: "s2", Title: "B", Objective: "b", Dependencies: []string{"s1"}},
			{StepID: "s3", Title: "C", Objective: "c", Dependencies: []string{"s2"}},
		},
	}
	result := v.Validate(plan)
	if !result.Valid {
		t.Errorf("linear deps should be valid, got: %v", result.Errors)
	}
}

func TestValidator_NonExistentDep(t *testing.T) {
	v := NewValidator(10, nil, nil)
	plan := &core.Plan{
		Goal: "test",
		Steps: []core.PlanStep{
			{StepID: "s1", Title: "A", Objective: "a", Dependencies: []string{"nonexistent"}},
		},
	}
	result := v.Validate(plan)
	if result.Valid {
		t.Error("non-existent dependency should be invalid")
	}
}

func TestValidator_UnknownSuggestedToolIsHintOnly(t *testing.T) {
	tools := []core.ToolDescriptor{{Name: "grep"}}
	v := NewValidator(10, tools, nil)
	plan := &core.Plan{
		Goal: "test",
		Steps: []core.PlanStep{
			{StepID: "s1", Title: "A", Objective: "a", SuggestedTools: []string{"nonexistent_tool"}},
		},
	}
	result := v.Validate(plan)
	if !result.Valid {
		t.Fatalf("unknown suggested tool should remain a non-blocking hint: %v", result.Errors)
	}
}

func TestValidator_UnknownAllowedToolIsInvalid(t *testing.T) {
	tools := []core.ToolDescriptor{{Name: "grep"}}
	v := NewValidator(10, tools, nil)
	result := v.Validate(&core.Plan{
		Goal: "test",
		Steps: []core.PlanStep{{
			StepID:       "s1",
			Title:        "A",
			Objective:    "a",
			AllowedTools: []string{"missing"},
		}},
	})
	if result.Valid {
		t.Fatal("unknown allowed tool must invalidate a caller-provided constrained step")
	}
}

func TestValidator_DisabledTool(t *testing.T) {
	tools := []core.ToolDescriptor{{Name: "dangerous_cmd"}}
	v := NewValidator(10, tools, []string{"dangerous_cmd"})
	plan := &core.Plan{
		Goal: "test",
		Steps: []core.PlanStep{
			{StepID: "s1", Title: "A", Objective: "a", SuggestedTools: []string{"dangerous_cmd"}},
		},
	}
	result := v.Validate(plan)
	if result.Valid {
		t.Error("disabled tool should be invalid")
	}
}
