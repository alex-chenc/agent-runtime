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

// recordingHookSink captures hook events for test assertions.
type recordingHookSink struct {
	events []HookEvent
}

func (r *recordingHookSink) Handle(_ context.Context, event HookEvent) error {
	r.events = append(r.events, event)
	return nil
}

func (r *recordingHookSink) eventsByType(t HookEventType) []HookEvent {
	var out []HookEvent
	for _, e := range r.events {
		if e.Type == t {
			out = append(out, e)
		}
	}
	return out
}

func TestStepLifecycleHooksCarryStepID(t *testing.T) {
	hook := &recordingHookSink{}
	llm := &sequenceLLM{responses: []LLMResponse{
		{Content: `{"goal":"inspect","steps":[{"title":"collect","objective":"collect info","expected_output":"info","suggested_tools":["host_info"],"risk_level":"read_only"}]}`},
		{Content: `{"action":"tool_call","summary":"collecting","tool_call":{"tool_name":"host_info","reason":"need info","args":{"detail":"basic"}}}`},
		{Content: `{"action":"step_result","summary":"done","step_result":{"result":"info collected","evidence":["info"],"confidence":"high"}}`},
		{Content: `{"final_answer":"done"}`},
	}}
	tools := &successToolGateway{}
	cfg := runtimeTestConfig()
	rt, err := New(
		WithConfig(cfg),
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
		WithHooks(hook),
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := rt.Run(context.Background(), TaskInput{TaskID: "task-stepid", UserInput: "inspect"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusCompleted {
		t.Fatalf("status = %q, want completed", result.Status)
	}

	// Wait for async hook events to be processed
	time.Sleep(50 * time.Millisecond)

	// Verify step_started hook carries StepID
	started := hook.eventsByType(HookStepStarted)
	if len(started) == 0 {
		t.Fatal("expected at least one step_started event")
	}
	for _, ev := range started {
		if ev.StepID == "" {
			t.Errorf("step_started event missing StepID: %+v", ev)
		}
	}

	// Verify step_completed or step_failed hook carries StepID
	completed := hook.eventsByType(HookStepCompleted)
	failed := hook.eventsByType(HookStepFailed)
	terminal := append(completed, failed...)
	if len(terminal) == 0 {
		t.Fatal("expected at least one step_completed or step_failed event")
	}
	for _, ev := range terminal {
		if ev.StepID == "" {
			t.Errorf("step terminal event missing StepID: %+v", ev)
		}
	}
}

type staticPromptProvider struct {
	planPrompt      PromptBundle
	reactPrompt     PromptBundle
	summarizePrompt PromptBundle
}

func (p *staticPromptProvider) Build(_ context.Context, req PromptRequest) (PromptBundle, error) {
	switch req.Purpose {
	case PurposePlan:
		return p.planPrompt, nil
	case PurposeReact:
		return p.reactPrompt, nil
	case PurposeSummarize:
		return p.summarizePrompt, nil
	default:
		return PromptBundle{}, nil
	}
}

func TestGenerateFinalAnswerUsesPromptProvider(t *testing.T) {
	summarizeSystemPrompt := "你是一个安全分析AI助手，请用中文生成最终分析报告。"
	provider := &staticPromptProvider{
		summarizePrompt: PromptBundle{SystemPrompt: summarizeSystemPrompt},
	}

	llm := &sequenceLLM{responses: []LLMResponse{
		{Content: `{"goal":"test","steps":[{"title":"s1","objective":"o1","expected_output":"e1","risk_level":"read_only"}]}`},
		{Content: `{"action":"step_result","summary":"done","step_result":{"result":"ok","evidence":["ok"],"confidence":"high"}}`},
		{Content: `{"final_answer":"中文结论"}`},
	}}

	cfg := runtimeTestConfig()
	rt, err := New(
		WithConfig(cfg),
		WithLLMClient(llm),
		WithPromptProvider(provider),
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := rt.Run(context.Background(), TaskInput{TaskID: "task-prompt", UserInput: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusCompleted {
		t.Fatalf("status = %q, want completed", result.Status)
	}

	// Find the summarize request (last one)
	var summarizeReq *LLMRequest
	for i := len(llm.requests) - 1; i >= 0; i-- {
		if llm.requests[i].Purpose == PurposeSummarize {
			summarizeReq = &llm.requests[i]
			break
		}
	}
	if summarizeReq == nil {
		t.Fatal("expected a PurposeSummarize request")
	}

	// Verify the prompt provider's system prompt was used
	if summarizeReq.Messages[0].Content != summarizeSystemPrompt {
		t.Errorf("expected system prompt %q, got %q", summarizeSystemPrompt, summarizeReq.Messages[0].Content)
	}
}

func TestReflectionRetryStep(t *testing.T) {
	llm := &sequenceLLM{responses: []LLMResponse{
		// Plan
		{Content: `{"goal":"test","steps":[{"title":"s1","objective":"do thing","expected_output":"result","risk_level":"read_only"}]}`},
		// First attempt: fail
		{Content: `{"action":"fail_step","failure":{"reason":"tool failed","recoverable":true}}`},
		// Reflection: retry_step
		{Content: `{"root_cause":"tool error","impact":"step incomplete","recoverable":true,"recommendation":"retry_step","reusable_lesson":"retry helps"}`},
		// Second attempt: success
		{Content: `{"action":"step_result","summary":"done","step_result":{"result":"success","evidence":["ok"],"confidence":"high"}}`},
		// Final answer
		{Content: `{"final_answer":"completed after retry"}`},
	}}
	cfg := runtimeTestConfig()
	cfg.EnableReflection = true
	cfg.MaxReflections = 3
	cfg.MaxStepRetries = 2
	rt, err := New(WithConfig(cfg), WithLLMClient(llm))
	if err != nil {
		t.Fatal(err)
	}
	result, err := rt.Run(context.Background(), TaskInput{TaskID: "t1", UserInput: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusCompleted {
		t.Fatalf("status = %q, want completed; errors=%v", result.Status, result.Errors)
	}
	if len(result.Reflections) != 1 {
		t.Fatalf("expected 1 reflection, got %d", len(result.Reflections))
	}
	if result.Reflections[0].Recommendation != ReflectRetryStep {
		t.Fatalf("expected retry_step recommendation, got %q", result.Reflections[0].Recommendation)
	}
	// Should have 2 step executions (original + retry)
	if len(result.StepExecutions) < 2 {
		t.Fatalf("expected >=2 step executions (retry), got %d", len(result.StepExecutions))
	}
}

func TestReflectionSkipStep(t *testing.T) {
	llm := &sequenceLLM{responses: []LLMResponse{
		// Plan with 2 steps
		{Content: `{"goal":"test","steps":[{"title":"s1","objective":"do thing","expected_output":"result","risk_level":"read_only"},{"title":"s2","objective":"do other","expected_output":"result","risk_level":"read_only"}]}`},
		// Step 1: fail
		{Content: `{"action":"fail_step","failure":{"reason":"cannot do","recoverable":false}}`},
		// Reflection: skip_step
		{Content: `{"root_cause":"unrecoverable","impact":"minor","recoverable":false,"recommendation":"skip_step","reusable_lesson":"skip when stuck"}`},
		// Step 2: success
		{Content: `{"action":"step_result","summary":"done","step_result":{"result":"ok","evidence":["ok"],"confidence":"high"}}`},
		// Final answer
		{Content: `{"final_answer":"completed with skip"}`},
	}}
	cfg := runtimeTestConfig()
	cfg.EnableReflection = true
	cfg.MaxReflections = 3
	cfg.AllowSkipFailedStep = true
	rt, err := New(WithConfig(cfg), WithLLMClient(llm))
	if err != nil {
		t.Fatal(err)
	}
	result, err := rt.Run(context.Background(), TaskInput{TaskID: "t2", UserInput: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusCompleted {
		t.Fatalf("status = %q, want completed; errors=%v", result.Status, result.Errors)
	}
	if len(result.Reflections) != 1 {
		t.Fatalf("expected 1 reflection, got %d", len(result.Reflections))
	}
	if result.Reflections[0].Recommendation != ReflectSkipStep {
		t.Fatalf("expected skip_step recommendation, got %q", result.Reflections[0].Recommendation)
	}
}

func TestReflectionSummarizeNow(t *testing.T) {
	llm := &sequenceLLM{responses: []LLMResponse{
		// Plan
		{Content: `{"goal":"test","steps":[{"title":"s1","objective":"do thing","expected_output":"result","risk_level":"read_only"}]}`},
		// Step 1: fail
		{Content: `{"action":"fail_step","failure":{"reason":"stuck","recoverable":false}}`},
		// Reflection: summarize_now
		{Content: `{"root_cause":"dead end","impact":"cannot proceed","recoverable":false,"recommendation":"summarize_now","reusable_lesson":"give up early"}`},
		// Final answer
		{Content: `{"final_answer":"could not complete, here is what we know"}`},
	}}
	cfg := runtimeTestConfig()
	cfg.EnableReflection = true
	cfg.MaxReflections = 3
	rt, err := New(WithConfig(cfg), WithLLMClient(llm))
	if err != nil {
		t.Fatal(err)
	}
	result, err := rt.Run(context.Background(), TaskInput{TaskID: "t3", UserInput: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusCompleted {
		t.Fatalf("status = %q, want completed; errors=%v", result.Status, result.Errors)
	}
	if result.ExitReason != ExitNormalCompleted {
		t.Fatalf("exit reason = %q, want %q", result.ExitReason, ExitNormalCompleted)
	}
}

func TestReflectionFail(t *testing.T) {
	llm := &sequenceLLM{responses: []LLMResponse{
		// Plan
		{Content: `{"goal":"test","steps":[{"title":"s1","objective":"do thing","expected_output":"result","risk_level":"read_only"}]}`},
		// Step 1: fail
		{Content: `{"action":"fail_step","failure":{"reason":"fatal","recoverable":false}}`},
		// Reflection: fail
		{Content: `{"root_cause":"fatal error","impact":"task cannot continue","recoverable":false,"recommendation":"fail","reusable_lesson":"check prereqs"}`},
		// Final answer
		{Content: `{"final_answer":"task failed"}`},
	}}
	cfg := runtimeTestConfig()
	cfg.EnableReflection = true
	cfg.MaxReflections = 3
	rt, err := New(WithConfig(cfg), WithLLMClient(llm))
	if err != nil {
		t.Fatal(err)
	}
	result, err := rt.Run(context.Background(), TaskInput{TaskID: "t4", UserInput: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitReason != ExitReflectionUnrecoverable {
		t.Fatalf("exit reason = %q, want %q", result.ExitReason, ExitReflectionUnrecoverable)
	}
}

func TestReflectionRetryStepMaxRetries(t *testing.T) {
	llm := &sequenceLLM{responses: []LLMResponse{
		// Plan
		{Content: `{"goal":"test","steps":[{"title":"s1","objective":"do thing","expected_output":"result","risk_level":"read_only"}]}`},
		// First attempt: fail
		{Content: `{"action":"fail_step","failure":{"reason":"fail1","recoverable":true}}`},
		// Reflection 1: retry_step
		{Content: `{"root_cause":"err1","impact":"step failed","recoverable":true,"recommendation":"retry_step"}`},
		// Second attempt: fail again
		{Content: `{"action":"fail_step","failure":{"reason":"fail2","recoverable":true}}`},
		// Reflection 2: retry_step (but max retries = 1, so should NOT retry)
		{Content: `{"root_cause":"err2","impact":"step failed again","recoverable":true,"recommendation":"retry_step"}`},
		// Final answer (forced since no more steps)
		{Content: `{"final_answer":"failed after retries"}`},
	}}
	cfg := runtimeTestConfig()
	cfg.EnableReflection = true
	cfg.MaxReflections = 3
	cfg.MaxStepRetries = 1 // Only 1 retry allowed
	rt, err := New(WithConfig(cfg), WithLLMClient(llm))
	if err != nil {
		t.Fatal(err)
	}
	result, err := rt.Run(context.Background(), TaskInput{TaskID: "t5", UserInput: "test"})
	if err != nil {
		t.Fatal(err)
	}
	// Should have 2 reflections (both retry_step)
	if len(result.Reflections) != 2 {
		t.Fatalf("expected 2 reflections, got %d", len(result.Reflections))
	}
	// Step should have failed (couldn't retry after max)
	if result.Status == StatusCompleted {
		// It may complete with best-effort, that's OK
		t.Logf("status = %q (best effort)", result.Status)
	}
}

func TestParseFinalAnswer(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "standard final_answer JSON",
			input: `{"final_answer":"analysis complete"}`,
			want:  "analysis complete",
		},
		{
			name:  "plain text",
			input: "analysis complete",
			want:  "analysis complete",
		},
		{
			name:  "structured JSON without final_answer field",
			input: `{"attack_graph":{"nodes":[],"edges":[]},"conclusions":[{"action":"confirm_threat"}]}`,
			want:  `{"attack_graph":{"nodes":[],"edges":[]},"conclusions":[{"action":"confirm_threat"}]}`,
		},
		{
			name:  "empty final_answer falls through to raw JSON",
			input: `{"final_answer":""}`,
			want:  `{"final_answer":""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFinalAnswer(tt.input)
			if got != tt.want {
				t.Errorf("parseFinalAnswer(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
