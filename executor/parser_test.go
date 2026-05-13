package executor

import (
	"testing"

	"github.com/chenchen511/agent-runtime/core"
)

func TestParseAction_ToolCall(t *testing.T) {
	input := `{"action":"tool_call","summary":"searching files","tool_call":{"tool_name":"grep","reason":"find pattern","args":{"pattern":"TODO"}}}`
	action, err := ParseAction(input)
	if err != nil {
		t.Fatal(err)
	}
	if action.Type != core.ActionToolCall {
		t.Errorf("type = %q, want tool_call", action.Type)
	}
	if action.ToolName != "grep" {
		t.Errorf("tool_name = %q, want grep", action.ToolName)
	}
	if action.ToolArgs == nil || action.ToolArgs["pattern"] != "TODO" {
		t.Errorf("tool_args = %v, want pattern=TODO", action.ToolArgs)
	}
}

func TestParseAction_StepResult(t *testing.T) {
	input := `{"action":"step_result","summary":"done","step_result":{"result":"found 5 files","evidence":["file1.go","file2.go"],"confidence":"high"}}`
	action, err := ParseAction(input)
	if err != nil {
		t.Fatal(err)
	}
	if action.Type != core.ActionStepResult {
		t.Errorf("type = %q, want step_result", action.Type)
	}
	if action.StepResult != "found 5 files" {
		t.Errorf("step_result = %q", action.StepResult)
	}
	if len(action.Evidence) != 2 {
		t.Errorf("evidence count = %d, want 2", len(action.Evidence))
	}
	if action.Confidence != "high" {
		t.Errorf("confidence = %q, want high", action.Confidence)
	}
}

func TestParseAction_FailStep(t *testing.T) {
	input := `{"action":"fail_step","summary":"failed","failure":{"reason":"tool not available","recoverable":true}}`
	action, err := ParseAction(input)
	if err != nil {
		t.Fatal(err)
	}
	if action.Type != core.ActionFailStep {
		t.Errorf("type = %q, want fail_step", action.Type)
	}
	if action.FailureReason != "tool not available" {
		t.Errorf("failure_reason = %q", action.FailureReason)
	}
	if action.Recoverable == nil || !*action.Recoverable {
		t.Error("recoverable should be true")
	}
}

func TestParseAction_RequestExperience(t *testing.T) {
	input := `{"action":"request_experience","summary":"need help","experience_request":{"query":"prior fixes","reason":"avoid repeated mistakes"}}`
	action, err := ParseAction(input)
	if err != nil {
		t.Fatal(err)
	}
	if action.Type != core.ActionRequestExperience {
		t.Errorf("type = %q, want request_experience", action.Type)
	}
	if !action.NeedsExperience {
		t.Error("needs_experience should be true")
	}
	if action.ExperienceQuery != "prior fixes" {
		t.Errorf("experience_query = %q", action.ExperienceQuery)
	}
}

func TestParseAction_RequestExperienceMissingQuery(t *testing.T) {
	input := `{"action":"request_experience","summary":"need help","experience_request":{"query":"","reason":"missing"}}`
	_, err := ParseAction(input)
	if err == nil {
		t.Error("expected error for missing experience query")
	}
}

func TestParseAction_UnknownAction(t *testing.T) {
	input := `{"action":"unknown_type","summary":"test"}`
	_, err := ParseAction(input)
	if err == nil {
		t.Error("expected error for unknown action type")
	}
}

func TestParseAction_InvalidJSON(t *testing.T) {
	_, err := ParseAction("not json at all")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseAction_ToolCallMissingToolName(t *testing.T) {
	input := `{"action":"tool_call","summary":"test","tool_call":{"tool_name":"","reason":"r","args":{}}}`
	_, err := ParseAction(input)
	if err == nil {
		t.Error("expected error for empty tool_name")
	}
}

func TestParseAction_ToolCallMissingField(t *testing.T) {
	input := `{"action":"tool_call","summary":"test"}`
	_, err := ParseAction(input)
	if err == nil {
		t.Error("expected error for missing tool_call field")
	}
}

func TestParseAction_StepResultMissingResult(t *testing.T) {
	input := `{"action":"step_result","summary":"test","step_result":{"result":""}}`
	_, err := ParseAction(input)
	if err == nil {
		t.Error("expected error for empty step_result.result")
	}
}

func TestParseAction_TypeFieldFallback(t *testing.T) {
	// LLM sometimes outputs "type" instead of "action" after tool errors
	input := `{"type":"tool_call","summary":"retry","tool_call":{"tool_name":"GetProcessTree","reason":"retry with different pid","args":{"host_id":"h1","pid":123}}}`
	action, err := ParseAction(input)
	if err != nil {
		t.Fatal(err)
	}
	if action.Type != core.ActionToolCall {
		t.Errorf("type = %q, want tool_call", action.Type)
	}
	if action.ToolName != "GetProcessTree" {
		t.Errorf("tool_name = %q, want GetProcessTree", action.ToolName)
	}
}

func TestParseAction_DegradedToolCallFormat(t *testing.T) {
	// LLM degrades to flat format after tool errors: tool_name/tool_args at top level
	input := `{"type":"tool_call","summary":"retry","tool_name":"GetProcessTree","tool_args":{"host_id":"h1","pid":50350}}`
	action, err := ParseAction(input)
	if err != nil {
		t.Fatal(err)
	}
	if action.Type != core.ActionToolCall {
		t.Errorf("type = %q, want tool_call", action.Type)
	}
	if action.ToolName != "GetProcessTree" {
		t.Errorf("tool_name = %q, want GetProcessTree", action.ToolName)
	}
	if action.ToolArgs == nil || action.ToolArgs["host_id"] != "h1" {
		t.Errorf("tool_args = %v", action.ToolArgs)
	}
}

func TestParseAction_DegradedFormatWithArgsAtTopLevel(t *testing.T) {
	// LLM uses "args" (not "tool_args") at top level after tool errors
	input := `{"type":"tool_call","summary":"retry","tool_name":"GetNetworkConnections","args":{"host_id":"h1"}}`
	action, err := ParseAction(input)
	if err != nil {
		t.Fatal(err)
	}
	if action.Type != core.ActionToolCall {
		t.Errorf("type = %q, want tool_call", action.Type)
	}
	if action.ToolName != "GetNetworkConnections" {
		t.Errorf("tool_name = %q, want GetNetworkConnections", action.ToolName)
	}
	if action.ToolArgs == nil || action.ToolArgs["host_id"] != "h1" {
		t.Errorf("tool_args = %v, want host_id=h1", action.ToolArgs)
	}
}

func TestParseAction_DegradedFormatWithActionField(t *testing.T) {
	// Degraded format but with "action" instead of "type"
	input := `{"action":"tool_call","summary":"retry","tool_name":"GetNetworkConnections","tool_args":{"host_id":"h1"}}`
	action, err := ParseAction(input)
	if err != nil {
		t.Fatal(err)
	}
	if action.Type != core.ActionToolCall {
		t.Errorf("type = %q, want tool_call", action.Type)
	}
	if action.ToolName != "GetNetworkConnections" {
		t.Errorf("tool_name = %q, want GetNetworkConnections", action.ToolName)
	}
}

func TestParseAction_WithMarkdownWrapper(t *testing.T) {
	input := "```json\n{\"action\":\"step_result\",\"summary\":\"done\",\"step_result\":{\"result\":\"ok\"}}\n```"
	action, err := ParseAction(input)
	if err != nil {
		t.Fatal(err)
	}
	if action.Type != core.ActionStepResult {
		t.Errorf("type = %q, want step_result", action.Type)
	}
}
