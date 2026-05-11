package agentruntime

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type sequenceLLM struct {
	responses []LLMResponse
	requests  []LLMRequest
}

func (f *sequenceLLM) Complete(_ context.Context, req LLMRequest) (LLMResponse, error) {
	f.requests = append(f.requests, req)
	if len(f.responses) == 0 {
		return LLMResponse{Content: `{"action":"fail_step","failure":{"reason":"no response configured"}}`}, nil
	}
	resp := f.responses[0]
	f.responses = f.responses[1:]
	return resp, nil
}

type successToolGateway struct {
	requests []ToolRequest
}

type staticExperienceProvider struct {
	items []ExperienceItem
}

func (p staticExperienceProvider) Fetch(context.Context, ExperienceRequest) (ExperienceResponse, error) {
	return ExperienceResponse{Items: p.items}, nil
}

type recordingExperienceProvider struct {
	items    []ExperienceItem
	requests []ExperienceRequest
}

func (p *recordingExperienceProvider) Fetch(_ context.Context, req ExperienceRequest) (ExperienceResponse, error) {
	p.requests = append(p.requests, req)
	return ExperienceResponse{Items: p.items}, nil
}

type failingHookSink struct{}

func (failingHookSink) Handle(context.Context, HookEvent) error {
	return errors.New("hook sink failed")
}

func (g *successToolGateway) Call(_ context.Context, req ToolRequest) (ToolResponse, error) {
	g.requests = append(g.requests, req)
	now := time.Now()
	return ToolResponse{
		CallID:    req.CallID,
		ToolName:  req.ToolName,
		Status:    ToolCallSuccess,
		Content:   `{"hostname":"test-host"}`,
		Summary:   "host info collected",
		StartedAt: now,
		EndedAt:   now,
	}, nil
}

func (g *successToolGateway) Cancel(context.Context, string, string) error {
	return nil
}

func runtimeTestConfig() RuntimeConfig {
	cfg := DefaultConfig()
	cfg.EnableAudit = false
	cfg.EnableReflection = false
	cfg.EnableCorrection = false
	cfg.TaskTimeout = time.Minute
	return cfg
}

func TestRuntimeRunRecordsModelAndToolCalls(t *testing.T) {
	llm := &sequenceLLM{responses: []LLMResponse{
		{Content: `{"goal":"inspect host","steps":[{"title":"collect host","objective":"collect host info","expected_output":"host info","suggested_tools":["host_info"],"risk_level":"read_only"}]}`, Model: "fake"},
		{Content: `{"action":"tool_call","summary":"collecting host info","tool_call":{"tool_name":"host_info","reason":"need host info","args":{"detail":"basic"}}}`, Model: "fake"},
		{Content: `{"action":"step_result","summary":"done","step_result":{"result":"host info collected","evidence":["host info collected"],"confidence":"high"}}`, Model: "fake"},
		{Content: `{"final_answer":"Host info collected successfully."}`, Model: "fake"},
	}}
	tools := &successToolGateway{}
	rt, err := New(
		WithConfig(runtimeTestConfig()),
		WithLLMClient(llm),
		WithToolGateway(tools),
		WithTools([]ToolDescriptor{{
			Name:         "host_info",
			Description:  "read host info",
			RiskLevel:    RiskReadOnly,
			AutoCallable: true,
			ArgsSchema: map[string]any{
				"required": []any{"detail"},
				"properties": map[string]any{
					"detail": map[string]any{"type": "string"},
				},
			},
		}}),
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := rt.Run(context.Background(), TaskInput{TaskID: "task-1", UserInput: "inspect host"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusCompleted {
		t.Fatalf("status = %q, want completed; errors=%v", result.Status, result.Errors)
	}
	if result.Completion.ModelCalls != 4 || result.Metrics.TotalModelCalls != 4 || len(result.ModelCalls) != 4 {
		t.Fatalf("model call accounting mismatch: completion=%d metrics=%d records=%d", result.Completion.ModelCalls, result.Metrics.TotalModelCalls, len(result.ModelCalls))
	}
	if result.Completion.ToolCalls != 1 || result.Metrics.TotalToolCalls != 1 || len(result.ToolCalls) != 1 {
		t.Fatalf("tool call accounting mismatch: completion=%d metrics=%d records=%d", result.Completion.ToolCalls, result.Metrics.TotalToolCalls, len(result.ToolCalls))
	}
	if result.StepExecutions[0].ReactTurns[0].ModelCallID != result.ModelCalls[1].CallID {
		t.Fatalf("react model call ID %q does not match recorded model call %q", result.StepExecutions[0].ReactTurns[0].ModelCallID, result.ModelCalls[1].CallID)
	}
	if result.FinalAnswer != "Host info collected successfully." {
		t.Fatalf("final answer = %q", result.FinalAnswer)
	}
	if len(tools.requests) != 1 {
		t.Fatalf("tool gateway calls = %d, want 1", len(tools.requests))
	}
}

func TestRuntimeRunWithoutToolsCanCompleteStepResult(t *testing.T) {
	llm := &sequenceLLM{responses: []LLMResponse{
		{Content: `{"goal":"summarize","steps":[{"title":"answer","objective":"produce answer","expected_output":"answer","risk_level":"read_only"}]}`},
		{Content: `{"action":"step_result","summary":"done","step_result":{"result":"answer produced","evidence":["model response"],"confidence":"medium"}}`},
		{Content: `{"final_answer":"answer produced"}`},
	}}
	rt, err := New(
		WithConfig(runtimeTestConfig()),
		WithLLMClient(llm),
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := rt.Run(context.Background(), TaskInput{TaskID: "task-2", UserInput: "answer directly"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusCompleted {
		t.Fatalf("status = %q, want completed; errors=%v", result.Status, result.Errors)
	}
	if len(result.ToolCalls) != 0 {
		t.Fatalf("tool calls = %d, want 0", len(result.ToolCalls))
	}
	if len(result.StepExecutions) != 1 || result.StepExecutions[0].Status != StepCompleted {
		t.Fatalf("step executions = %+v, want one completed step", result.StepExecutions)
	}
}

func TestRuntimeRunLoadsExperienceForPlanning(t *testing.T) {
	llm := &sequenceLLM{responses: []LLMResponse{
		{Content: `{"goal":"summarize","steps":[{"title":"answer","objective":"produce answer","expected_output":"answer","risk_level":"read_only"}]}`},
		{Content: `{"action":"step_result","summary":"done","step_result":{"result":"answer produced","evidence":["model response"],"confidence":"medium"}}`},
		{Content: `{"final_answer":"answer produced"}`},
	}}
	rt, err := New(
		WithConfig(runtimeTestConfig()),
		WithLLMClient(llm),
		WithExperienceProvider(staticExperienceProvider{items: []ExperienceItem{{
			ID:      "exp-1",
			Summary: "prefer concise answers",
		}}}),
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := rt.Run(context.Background(), TaskInput{TaskID: "task-3", UserInput: "answer directly"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.ExperienceUsage) != 1 || result.ExperienceUsage[0].ItemID != "exp-1" {
		t.Fatalf("experience usage = %+v, want exp-1", result.ExperienceUsage)
	}
	if len(llm.requests) == 0 || !strings.Contains(llm.requests[0].Messages[0].Content, "prefer concise answers") {
		t.Fatalf("planner request did not include experience summary: %+v", llm.requests)
	}
}

func TestRuntimeRunHandlesReactExperienceRequest(t *testing.T) {
	llm := &sequenceLLM{responses: []LLMResponse{
		{Content: `{"goal":"summarize","steps":[{"title":"answer","objective":"produce answer","expected_output":"answer","risk_level":"read_only"}]}`},
		{Content: `{"action":"request_experience","summary":"need guidance","experience_request":{"query":"answer style","reason":"choose style"}}`},
		{Content: `{"action":"step_result","summary":"done","step_result":{"result":"used experience","evidence":["experience"],"confidence":"medium"}}`},
		{Content: `{"final_answer":"used experience"}`},
	}}
	provider := &recordingExperienceProvider{items: []ExperienceItem{{ID: "exp-2", Summary: "use direct language"}}}
	rt, err := New(
		WithConfig(runtimeTestConfig()),
		WithLLMClient(llm),
		WithExperienceProvider(provider),
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := rt.Run(context.Background(), TaskInput{TaskID: "task-4", UserInput: "answer directly"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusCompleted {
		t.Fatalf("status = %q, want completed; errors=%v", result.Status, result.Errors)
	}
	if len(provider.requests) != 2 {
		t.Fatalf("experience provider requests = %d, want planning + react", len(provider.requests))
	}
	if provider.requests[1].Query != "answer style" {
		t.Fatalf("react experience query = %q", provider.requests[1].Query)
	}
	if len(result.ExperienceUsage) != 2 {
		t.Fatalf("experience usage count = %d, want 2", len(result.ExperienceUsage))
	}
	if !strings.Contains(llm.requests[2].Messages[len(llm.requests[2].Messages)-1].Content, "use direct language") {
		t.Fatalf("react follow-up did not include fetched experience: %+v", llm.requests[2].Messages)
	}
}

func TestRuntimeRunFailOnTaskFinishedHookError(t *testing.T) {
	cfg := runtimeTestConfig()
	cfg.FailOnHookError = true
	llm := &sequenceLLM{responses: []LLMResponse{
		{Content: `{"goal":"summarize","steps":[{"title":"answer","objective":"produce answer","expected_output":"answer","risk_level":"read_only"}]}`},
		{Content: `{"action":"step_result","summary":"done","step_result":{"result":"answer produced","evidence":["model response"],"confidence":"medium"}}`},
		{Content: `{"final_answer":"answer produced"}`},
	}}
	rt, err := New(
		WithConfig(cfg),
		WithLLMClient(llm),
		WithHooks(failingHookSink{}),
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := rt.Run(context.Background(), TaskInput{TaskID: "task-5", UserInput: "answer directly"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if result.ExitReason != ExitSystemError {
		t.Fatalf("exit reason = %q, want system_error", result.ExitReason)
	}
	if len(result.Errors) == 0 || !strings.Contains(result.Errors[len(result.Errors)-1].Message, "hook sink failed") {
		t.Fatalf("expected hook error in result, got %+v", result.Errors)
	}
}
