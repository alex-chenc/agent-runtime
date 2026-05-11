package plan

import (
	"testing"

	"github.com/chenchen511/agent-runtime/core"
)

func makePlan(steps ...core.PlanStep) *core.Plan {
	return &core.Plan{
		PlanID: "test-plan",
		Goal:   "test goal",
		Steps:  steps,
	}
}

func pendingStep(id string, deps ...string) core.PlanStep {
	return core.PlanStep{StepID: id, Title: "title-" + id, Objective: "obj", Status: core.StepPending, Dependencies: deps}
}

func completedStep(id string) core.PlanStep {
	return core.PlanStep{StepID: id, Title: "title-" + id, Objective: "obj", Status: core.StepCompleted}
}

func TestManager_NextExecutableStep_NoDeps(t *testing.T) {
	m := NewManager()
	m.SetInitialPlan(makePlan(
		pendingStep("step_1"),
		pendingStep("step_2"),
	))
	step := m.NextExecutableStep()
	if step == nil || step.StepID != "step_1" {
		t.Errorf("expected step_1, got %v", step)
	}
}

func TestManager_NextExecutableStep_WithDeps(t *testing.T) {
	m := NewManager()
	m.SetInitialPlan(makePlan(
		completedStep("step_1"),
		pendingStep("step_2", "step_1"),
		pendingStep("step_3", "step_2"),
	))
	step := m.NextExecutableStep()
	if step == nil || step.StepID != "step_2" {
		t.Errorf("expected step_2 (dep met), got %v", step)
	}
}

func TestManager_NextExecutableStep_UnmetDep(t *testing.T) {
	m := NewManager()
	m.SetInitialPlan(makePlan(
		pendingStep("step_1", "step_2"),
		pendingStep("step_2"),
	))
	step := m.NextExecutableStep()
	if step == nil || step.StepID != "step_2" {
		t.Errorf("expected step_2 (no deps), got %v", step)
	}
}

func TestManager_NextExecutableStep_AllDone(t *testing.T) {
	m := NewManager()
	m.SetInitialPlan(makePlan(
		completedStep("step_1"),
		completedStep("step_2"),
	))
	step := m.NextExecutableStep()
	if step != nil {
		t.Errorf("expected nil when all steps done, got %v", step)
	}
}

func TestManager_UpdateStepStatus(t *testing.T) {
	m := NewManager()
	m.SetInitialPlan(makePlan(pendingStep("step_1")))
	if err := m.UpdateStepStatus("step_1", core.StepCompleted); err != nil {
		t.Fatal(err)
	}
	step := m.NextExecutableStep()
	if step != nil {
		t.Error("expected nil after completing only step")
	}
}

func TestManager_UpdateStepStatus_NotFound(t *testing.T) {
	m := NewManager()
	m.SetInitialPlan(makePlan(pendingStep("step_1")))
	if err := m.UpdateStepStatus("nonexistent", core.StepCompleted); err == nil {
		t.Error("expected error for non-existent step")
	}
}

func TestManager_ApplyCorrection_SkipStep(t *testing.T) {
	m := NewManager()
	m.SetInitialPlan(makePlan(
		pendingStep("step_1"),
		pendingStep("step_2"),
	))
	err := m.ApplyCorrection(&core.CorrectionResult{
		ToPlanVersion: 2,
		Actions: []core.CorrectionAction{
			{Type: core.CorrectionSkipStep, TargetStepID: "step_1"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	step := m.NextExecutableStep()
	if step == nil || step.StepID != "step_2" {
		t.Errorf("expected step_2 after skipping step_1, got %v", step)
	}
}

func TestManager_ApplyCorrection_SkipStepUsesStepIDFallback(t *testing.T) {
	m := NewManager()
	m.SetInitialPlan(makePlan(
		pendingStep("step_1"),
		pendingStep("step_2"),
	))
	err := m.ApplyCorrection(&core.CorrectionResult{
		ToPlanVersion: 2,
		Actions: []core.CorrectionAction{
			{Type: core.CorrectionSkipStep, StepID: "step_1"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	step := m.NextExecutableStep()
	if step == nil || step.StepID != "step_2" {
		t.Errorf("expected step_2 after skipping step_1 via StepID, got %v", step)
	}
}

func TestManager_ApplyCorrection_AddStep(t *testing.T) {
	m := NewManager()
	m.SetInitialPlan(makePlan(pendingStep("step_1")))
	err := m.ApplyCorrection(&core.CorrectionResult{
		ToPlanVersion: 2,
		Actions: []core.CorrectionAction{
			{Type: core.CorrectionAddStep, NewStepID: "step_new", Reason: "new step added"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	plan := m.CurrentPlan()
	if len(plan.Steps) != 2 {
		t.Errorf("expected 2 steps after add, got %d", len(plan.Steps))
	}
}

func TestManager_ApplyCorrection_ReplaceStep(t *testing.T) {
	m := NewManager()
	m.SetInitialPlan(makePlan(pendingStep("step_1")))
	err := m.ApplyCorrection(&core.CorrectionResult{
		ToPlanVersion: 2,
		Actions: []core.CorrectionAction{
			{Type: core.CorrectionReplaceStep, TargetStepID: "step_1", Reason: "replaced objective"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	plan := m.CurrentPlan()
	if plan.Steps[0].Title != "replaced objective" {
		t.Errorf("expected replaced title, got %q", plan.Steps[0].Title)
	}
}

func TestManager_ApplyCorrection_NilPlan(t *testing.T) {
	m := NewManager()
	err := m.ApplyCorrection(&core.CorrectionResult{ToPlanVersion: 2})
	if err == nil {
		t.Error("expected error when no plan set")
	}
}

func TestManager_AllStepsTerminal(t *testing.T) {
	m := NewManager()
	m.SetInitialPlan(makePlan(
		completedStep("step_1"),
		completedStep("step_2"),
	))
	if !m.AllStepsTerminal() {
		t.Error("expected all steps terminal")
	}
}

func TestManager_AllStepsTerminal_Pending(t *testing.T) {
	m := NewManager()
	m.SetInitialPlan(makePlan(
		completedStep("step_1"),
		pendingStep("step_2"),
	))
	if m.AllStepsTerminal() {
		t.Error("expected not all steps terminal")
	}
}
