package audit

import (
	"context"
	"errors"
	"testing"

	"github.com/alex-chenc/agent-runtime/core"
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

func TestParseAuditResult_Continue(t *testing.T) {
	input := `{"drifted":false,"risk_level":"read_only","decision":"continue","should_exit":false}`
	r, err := ParseAuditResult(input)
	if err != nil {
		t.Fatal(err)
	}
	if r.Drifted {
		t.Error("Drifted should be false")
	}
	if r.Decision != core.AuditContinue {
		t.Errorf("Decision = %q, want continue", r.Decision)
	}
	if r.ShouldExit {
		t.Error("ShouldExit should be false")
	}
}

func TestParseAuditResult_AllDecisions(t *testing.T) {
	cases := map[string]core.AuditDecision{
		"continue":           core.AuditContinue,
		"minor_adjustment":   core.AuditMinorAdjustment,
		"correct_plan":       core.AuditCorrectPlan,
		"request_experience": core.AuditRequestExperience,
		"summarize_now":      core.AuditSummarizeNow,
		"fail":               core.AuditFail,
	}
	for input, want := range cases {
		r, err := ParseAuditResult(`{"decision":"` + input + `","drifted":false}`)
		if err != nil {
			t.Errorf("ParseAuditResult(%q): %v", input, err)
			continue
		}
		if r.Decision != want {
			t.Errorf("decision %q: got %q, want %q", input, r.Decision, want)
		}
	}
}

func TestParseAuditResult_EmbeddedInProse(t *testing.T) {
	input := "Here is my analysis:\n{\"drifted\":true,\"risk_level\":\"high\",\"decision\":\"correct_plan\",\"findings\":[\"drift detected\"],\"should_exit\":false}\nDone."
	r, err := ParseAuditResult(input)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Drifted {
		t.Error("Drifted should be true")
	}
	if r.Decision != core.AuditCorrectPlan {
		t.Errorf("Decision = %q", r.Decision)
	}
	if len(r.Findings) != 1 || r.Findings[0] != "drift detected" {
		t.Errorf("Findings = %v", r.Findings)
	}
}

func TestParseAuditResult_InvalidJSON(t *testing.T) {
	_, err := ParseAuditResult("not json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseAuditResult_WithExitReason(t *testing.T) {
	input := `{"drifted":true,"risk_level":"high","decision":"fail","should_exit":true,"exit_reason":"audit_unrecoverable"}`
	r, err := ParseAuditResult(input)
	if err != nil {
		t.Fatal(err)
	}
	if !r.ShouldExit {
		t.Error("ShouldExit should be true")
	}
	if r.ExitReason != core.ExitAuditUnrecoverable {
		t.Errorf("ExitReason = %q", r.ExitReason)
	}
}

func TestPolicy_ShouldAuditBySteps(t *testing.T) {
	p := NewPolicy(3, 0)
	cases := []struct {
		steps int
		want  bool
	}{
		{0, false},
		{1, false},
		{2, false},
		{3, true},
		{4, false},
		{5, false},
		{6, true},
	}
	for _, c := range cases {
		got := p.ShouldAuditBySteps(c.steps)
		if got != c.want {
			t.Errorf("ShouldAuditBySteps(%d) = %v, want %v", c.steps, got, c.want)
		}
	}
}

func TestPolicy_ShouldAuditByTurns(t *testing.T) {
	p := NewPolicy(0, 5)
	cases := []struct {
		turns int
		want  bool
	}{
		{0, false},
		{1, false},
		{4, false},
		{5, true},
		{9, false},
		{10, true},
	}
	for _, c := range cases {
		got := p.ShouldAuditByTurns(c.turns)
		if got != c.want {
			t.Errorf("ShouldAuditByTurns(%d) = %v, want %v", c.turns, got, c.want)
		}
	}
}

func TestPolicy_ZeroDivisor(t *testing.T) {
	p := NewPolicy(0, 0)
	if p.ShouldAuditBySteps(5) {
		t.Error("ShouldAuditBySteps with 0 divisor should return false")
	}
	if p.ShouldAuditByTurns(5) {
		t.Error("ShouldAuditByTurns with 0 divisor should return false")
	}
}

func TestAuditor_NilLLM(t *testing.T) {
	a := New(nil, fixedIDGen{id: "test-id"})
	result, err := a.Audit(context.Background(), AuditInput{
		TaskID:  "task-1",
		Goal:    "test goal",
		Trigger: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Decision != core.AuditContinue {
		t.Errorf("Decision = %q, want continue", result.Decision)
	}
	if result.Drifted {
		t.Error("Drifted should be false for default audit")
	}
	if result.AuditID != "test-id" {
		t.Errorf("AuditID = %q, want test-id", result.AuditID)
	}
}

func TestAuditor_LLMError_Fallback(t *testing.T) {
	a := New(&mockLLM{err: errors.New("llm down")}, fixedIDGen{id: "fallback"})
	result, err := a.Audit(context.Background(), AuditInput{
		TaskID:  "task-1",
		Goal:    "test",
		Trigger: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Decision != core.AuditContinue {
		t.Errorf("Decision = %q, want continue (fallback)", result.Decision)
	}
	if result.AuditID != "fallback" {
		t.Errorf("AuditID = %q", result.AuditID)
	}
}

func TestAuditor_SuccessfulLLM(t *testing.T) {
	llmResp := `{"drifted":true,"risk_level":"high","decision":"correct_plan","findings":["off track"],"should_exit":false}`
	a := New(&mockLLM{resp: core.LLMResponse{Content: llmResp}}, fixedIDGen{id: "audit-1"})
	result, err := a.Audit(context.Background(), AuditInput{
		TaskID:  "task-1",
		Goal:    "test",
		Trigger: "periodic",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Drifted {
		t.Error("Drifted should be true")
	}
	if result.Decision != core.AuditCorrectPlan {
		t.Errorf("Decision = %q", result.Decision)
	}
	if result.AuditID != "audit-1" {
		t.Errorf("AuditID = %q", result.AuditID)
	}
	if result.Trigger != "periodic" {
		t.Errorf("Trigger = %q", result.Trigger)
	}
}

func TestAuditor_LLMReturnsInvalidJSON_Fallback(t *testing.T) {
	a := New(&mockLLM{resp: core.LLMResponse{Content: "not json"}}, fixedIDGen{id: "fb"})
	result, err := a.Audit(context.Background(), AuditInput{TaskID: "t", Trigger: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Decision != core.AuditContinue {
		t.Errorf("Decision = %q, want continue (fallback)", result.Decision)
	}
}
