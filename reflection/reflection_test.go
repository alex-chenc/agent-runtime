package reflection

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

func TestParseReflection_RetryStep(t *testing.T) {
	input := `{"root_cause":"timeout","impact":"step blocked","recoverable":true,"recommendation":"retry_step"}`
	r, err := ParseReflection(input)
	if err != nil {
		t.Fatal(err)
	}
	if r.RootCause != "timeout" {
		t.Errorf("RootCause = %q", r.RootCause)
	}
	if !r.Recoverable {
		t.Error("Recoverable should be true")
	}
	if r.Recommendation != core.ReflectRetryStep {
		t.Errorf("Recommendation = %q", r.Recommendation)
	}
}

func TestParseReflection_AllRecommendations(t *testing.T) {
	cases := map[string]core.ReflectionDecision{
		"retry_step":         core.ReflectRetryStep,
		"skip_step":          core.ReflectSkipStep,
		"correct_plan":       core.ReflectCorrectPlan,
		"request_experience": core.ReflectRequestExperience,
		"summarize_now":      core.ReflectSummarizeNow,
		"fail":               core.ReflectFail,
	}
	for input, want := range cases {
		r, err := ParseReflection(`{"root_cause":"x","recommendation":"` + input + `"}`)
		if err != nil {
			t.Errorf("ParseReflection(%q): %v", input, err)
			continue
		}
		if r.Recommendation != want {
			t.Errorf("recommendation %q: got %q, want %q", input, r.Recommendation, want)
		}
	}
}

func TestParseReflection_EmptyRootCause(t *testing.T) {
	input := `{"root_cause":"","recommendation":"retry_step"}`
	_, err := ParseReflection(input)
	if err == nil {
		t.Error("expected error for empty root_cause")
	}
}

func TestParseReflection_InvalidJSON(t *testing.T) {
	_, err := ParseReflection("not json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseReflection_OptionalFields(t *testing.T) {
	input := `{"root_cause":"x","impact":"y","recoverable":false,"recommendation":"skip_step","disable_tools":["tool1","tool2"],"correction_hint":"hint","experience_query":"query","reusable_lesson":"lesson"}`
	r, err := ParseReflection(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.DisableTools) != 2 {
		t.Errorf("DisableTools count = %d, want 2", len(r.DisableTools))
	}
	if r.CorrectionHint != "hint" {
		t.Errorf("CorrectionHint = %q", r.CorrectionHint)
	}
	if r.ExperienceQuery != "query" {
		t.Errorf("ExperienceQuery = %q", r.ExperienceQuery)
	}
	if r.ReusableLesson != "lesson" {
		t.Errorf("ReusableLesson = %q", r.ReusableLesson)
	}
}

func TestParseReflection_EmbeddedInProse(t *testing.T) {
	input := "Analysis:\n```json\n{\"root_cause\":\"disk full\",\"recommendation\":\"fail\"}\n```\nEnd."
	r, err := ParseReflection(input)
	if err != nil {
		t.Fatal(err)
	}
	if r.RootCause != "disk full" {
		t.Errorf("RootCause = %q", r.RootCause)
	}
	if r.Recommendation != core.ReflectFail {
		t.Errorf("Recommendation = %q", r.Recommendation)
	}
}

func TestParseReflection_UnknownRecommendation_DefaultsToRetry(t *testing.T) {
	input := `{"root_cause":"x","recommendation":"something_unknown"}`
	r, err := ParseReflection(input)
	if err != nil {
		t.Fatal(err)
	}
	if r.Recommendation != core.ReflectRetryStep {
		t.Errorf("Recommendation = %q, want retry_step (default)", r.Recommendation)
	}
}

func TestReflector_NilLLM(t *testing.T) {
	r := New(nil, fixedIDGen{id: "ref-1"})
	result, err := r.Reflect(context.Background(), ReflectInput{
		TaskID:  "task-1",
		StepID:  "s1",
		Trigger: "tool_failure",
		Error: core.RuntimeError{
			Message:     "tool timeout",
			Recoverable: true,
		},
		PlanGoal: "test goal",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.RootCause != "tool timeout" {
		t.Errorf("RootCause = %q", result.RootCause)
	}
	if !result.Recoverable {
		t.Error("Recoverable should be true")
	}
	if result.Recommendation != core.ReflectRetryStep {
		t.Errorf("Recommendation = %q", result.Recommendation)
	}
	if result.ReflectionID != "ref-1" {
		t.Errorf("ReflectionID = %q", result.ReflectionID)
	}
}

func TestReflector_LLMError_Fallback(t *testing.T) {
	r := New(&mockLLM{err: errors.New("llm error")}, fixedIDGen{id: "fb"})
	result, err := r.Reflect(context.Background(), ReflectInput{
		TaskID:  "task-1",
		StepID:  "s1",
		Trigger: "test",
		Error:   core.RuntimeError{Message: "err msg", Recoverable: false},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.RootCause != "err msg" {
		t.Errorf("RootCause = %q", result.RootCause)
	}
	if result.Recoverable {
		t.Error("Recoverable should be false")
	}
}

func TestReflector_SuccessfulLLM(t *testing.T) {
	llmResp := `{"root_cause":"permission denied","impact":"cannot write file","recoverable":false,"recommendation":"fail","reusable_lesson":"check permissions first"}`
	r := New(&mockLLM{resp: core.LLMResponse{Content: llmResp}}, fixedIDGen{id: "ref-ok"})
	result, err := r.Reflect(context.Background(), ReflectInput{
		TaskID:  "task-1",
		StepID:  "s1",
		Trigger: "tool_failure",
		Error:   core.RuntimeError{Message: "perm err"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.RootCause != "permission denied" {
		t.Errorf("RootCause = %q", result.RootCause)
	}
	if result.Recommendation != core.ReflectFail {
		t.Errorf("Recommendation = %q", result.Recommendation)
	}
	if result.ReflectionID != "ref-ok" {
		t.Errorf("ReflectionID = %q", result.ReflectionID)
	}
	if result.ReusableLesson != "check permissions first" {
		t.Errorf("ReusableLesson = %q", result.ReusableLesson)
	}
}

func TestReflector_LLMReturnsInvalidJSON_Fallback(t *testing.T) {
	r := New(&mockLLM{resp: core.LLMResponse{Content: "bad json"}}, fixedIDGen{id: "fb2"})
	result, err := r.Reflect(context.Background(), ReflectInput{
		TaskID:  "task-1",
		Trigger: "test",
		Error:   core.RuntimeError{Message: "fallback test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.RootCause != "fallback test" {
		t.Errorf("RootCause = %q, want fallback", result.RootCause)
	}
}
