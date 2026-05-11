package correction

import (
	"context"
	"errors"
	"testing"

	"github.com/chenchen511/agent-runtime/core"
)

type fixedIDGen struct {
	id string
}

func (g fixedIDGen) Generate() string { return g.id }

type mockLLM struct {
	resp core.LLMResponse
	err  error
}

func (m *mockLLM) Complete(_ context.Context, _ core.LLMRequest) (core.LLMResponse, error) {
	return m.resp, m.err
}

func TestParseCorrection_Valid(t *testing.T) {
	input := `{"reason":"step 2 is blocked","actions":[{"type":"skip_step","step_id":"step_2","reason":"blocked by dependency"}]}`
	r, err := ParseCorrection(input)
	if err != nil {
		t.Fatal(err)
	}
	if r.Reason != "step 2 is blocked" {
		t.Errorf("Reason = %q", r.Reason)
	}
	if len(r.Actions) != 1 {
		t.Fatalf("Actions count = %d, want 1", len(r.Actions))
	}
	if r.Actions[0].Type != core.CorrectionSkipStep {
		t.Errorf("Action.Type = %q", r.Actions[0].Type)
	}
	if r.Actions[0].StepID != "step_2" {
		t.Errorf("Action.StepID = %q", r.Actions[0].StepID)
	}
	if r.Actions[0].TargetStepID != "step_2" {
		t.Errorf("Action.TargetStepID = %q", r.Actions[0].TargetStepID)
	}
	if !r.Valid {
		t.Error("Valid should be true")
	}
}

func TestParseCorrection_EmptyReason(t *testing.T) {
	input := `{"reason":"","actions":[]}`
	_, err := ParseCorrection(input)
	if err == nil {
		t.Error("expected error for empty reason")
	}
}

func TestParseCorrection_InvalidJSON(t *testing.T) {
	_, err := ParseCorrection("not json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseCorrection_MultipleActions(t *testing.T) {
	input := `{"reason":"restructure","actions":[{"type":"skip_step","step_id":"s1","reason":"r1"},{"type":"add_step","reason":"r2"},{"type":"replace_step","step_id":"s3","reason":"r3"}]}`
	r, err := ParseCorrection(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Actions) != 3 {
		t.Fatalf("Actions count = %d, want 3", len(r.Actions))
	}
	if r.Actions[0].Type != core.CorrectionSkipStep {
		t.Errorf("Action[0].Type = %q", r.Actions[0].Type)
	}
	if r.Actions[1].Type != core.CorrectionAddStep {
		t.Errorf("Action[1].Type = %q", r.Actions[1].Type)
	}
	if r.Actions[2].Type != core.CorrectionReplaceStep {
		t.Errorf("Action[2].Type = %q", r.Actions[2].Type)
	}
}

func TestParseCorrection_EmbeddedInProse(t *testing.T) {
	input := "Here is the correction:\n```json\n{\"reason\":\"fix\",\"actions\":[]}\n```\nDone."
	r, err := ParseCorrection(input)
	if err != nil {
		t.Fatal(err)
	}
	if r.Reason != "fix" {
		t.Errorf("Reason = %q", r.Reason)
	}
}

func TestValidateCorrection_NilCorrection(t *testing.T) {
	v := ValidateCorrection(nil, &core.Plan{}, nil)
	if v.Valid {
		t.Error("nil correction should be invalid")
	}
}

func TestValidateCorrection_SkipCompletedStep(t *testing.T) {
	correction := &core.CorrectionResult{
		Actions: []core.CorrectionAction{
			{Type: core.CorrectionSkipStep, StepID: "s1"},
		},
	}
	completed := map[string]bool{"s1": true}
	v := ValidateCorrection(correction, &core.Plan{}, completed)
	if v.Valid {
		t.Error("skipping completed step should be invalid")
	}
}

func TestValidateCorrection_ReplaceCompletedStep(t *testing.T) {
	correction := &core.CorrectionResult{
		Actions: []core.CorrectionAction{
			{Type: core.CorrectionReplaceStep, StepID: "s1"},
		},
	}
	completed := map[string]bool{"s1": true}
	v := ValidateCorrection(correction, &core.Plan{}, completed)
	if v.Valid {
		t.Error("replacing completed step should be invalid")
	}
}

func TestValidateCorrection_TargetStepIDCompleted(t *testing.T) {
	correction := &core.CorrectionResult{
		Actions: []core.CorrectionAction{
			{Type: core.CorrectionSkipStep, TargetStepID: "s1"},
		},
	}
	completed := map[string]bool{"s1": true}
	v := ValidateCorrection(correction, &core.Plan{}, completed)
	if v.Valid {
		t.Error("skipping completed target step should be invalid")
	}
}

func TestValidateCorrection_AddStepAllowed(t *testing.T) {
	correction := &core.CorrectionResult{
		Actions: []core.CorrectionAction{
			{Type: core.CorrectionAddStep},
		},
	}
	v := ValidateCorrection(correction, &core.Plan{}, nil)
	if !v.Valid {
		t.Errorf("add step should be valid: %v", v.Errors)
	}
}

func TestValidateCorrection_SkipNonCompletedStep(t *testing.T) {
	correction := &core.CorrectionResult{
		Actions: []core.CorrectionAction{
			{Type: core.CorrectionSkipStep, StepID: "s2"},
		},
	}
	completed := map[string]bool{"s1": true}
	v := ValidateCorrection(correction, &core.Plan{}, completed)
	if !v.Valid {
		t.Errorf("skipping non-completed step should be valid: %v", v.Errors)
	}
}

func TestCorrector_NilLLM(t *testing.T) {
	c := New(nil, fixedIDGen{id: "corr-1"})
	plan := &core.Plan{Version: 1, Steps: []core.PlanStep{{StepID: "s1", Title: "step 1"}}}
	result, err := c.Correct(context.Background(), CorrectionInput{
		TaskID:      "task-1",
		CurrentPlan: plan,
		Trigger:     "audit",
		Hint:        "step 1 needs fix",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.CorrectionID != "corr-1" {
		t.Errorf("CorrectionID = %q", result.CorrectionID)
	}
	if result.FromPlanVersion != 1 {
		t.Errorf("FromPlanVersion = %d, want 1", result.FromPlanVersion)
	}
	if result.ToPlanVersion != 2 {
		t.Errorf("ToPlanVersion = %d, want 2", result.ToPlanVersion)
	}
	if result.Reason != "step 1 needs fix" {
		t.Errorf("Reason = %q", result.Reason)
	}
}

func TestCorrector_LLMError_Fallback(t *testing.T) {
	c := New(&mockLLM{err: errors.New("llm error")}, fixedIDGen{id: "fb"})
	plan := &core.Plan{Version: 3}
	result, err := c.Correct(context.Background(), CorrectionInput{
		TaskID:      "task-1",
		CurrentPlan: plan,
		Trigger:     "reflection",
		Hint:        "skip step",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.FromPlanVersion != 3 {
		t.Errorf("FromPlanVersion = %d, want 3", result.FromPlanVersion)
	}
	if result.ToPlanVersion != 4 {
		t.Errorf("ToPlanVersion = %d, want 4", result.ToPlanVersion)
	}
}

func TestCorrector_SuccessfulLLM(t *testing.T) {
	llmResp := `{"reason":"restructure needed","actions":[{"type":"skip_step","step_id":"s2","reason":"blocked"}]}`
	c := New(&mockLLM{resp: core.LLMResponse{Content: llmResp}}, fixedIDGen{id: "c-1"})
	plan := &core.Plan{Version: 2, Steps: []core.PlanStep{{StepID: "s1"}, {StepID: "s2"}}}
	result, err := c.Correct(context.Background(), CorrectionInput{
		TaskID:      "task-1",
		CurrentPlan: plan,
		Trigger:     "audit",
		Hint:        "",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Reason != "restructure needed" {
		t.Errorf("Reason = %q", result.Reason)
	}
	if len(result.Actions) != 1 {
		t.Fatalf("Actions count = %d, want 1", len(result.Actions))
	}
	if result.CorrectionID != "c-1" {
		t.Errorf("CorrectionID = %q", result.CorrectionID)
	}
	if result.FromPlanVersion != 2 {
		t.Errorf("FromPlanVersion = %d", result.FromPlanVersion)
	}
	if result.ToPlanVersion != 3 {
		t.Errorf("ToPlanVersion = %d", result.ToPlanVersion)
	}
}
