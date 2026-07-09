package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alex-chenc/agent-runtime/core"
)

type reliabilitySequenceLLM struct {
	responses []core.LLMResponse
	requests  []core.LLMRequest
}

func (l *reliabilitySequenceLLM) Complete(_ context.Context, req core.LLMRequest) (core.LLMResponse, error) {
	l.requests = append(l.requests, req)
	if len(l.responses) == 0 {
		return core.LLMResponse{}, fmt.Errorf("no response configured")
	}
	response := l.responses[0]
	l.responses = l.responses[1:]
	return response, nil
}

type reliabilityIDGenerator struct {
	next int
}

func (g *reliabilityIDGenerator) Generate() string {
	g.next++
	return fmt.Sprintf("id-%d", g.next)
}

type reliabilityToolCaller struct {
	descriptors []core.ToolDescriptor
	responses   []core.ToolResponse
	errs        []error
	requests    []core.ToolRequest
}

func (c *reliabilityToolCaller) CallValidated(_ context.Context, req core.ToolRequest) (core.ToolResponse, error) {
	c.requests = append(c.requests, req)
	if len(c.errs) > 0 {
		err := c.errs[0]
		c.errs = c.errs[1:]
		if err != nil {
			return core.ToolResponse{}, err
		}
	}
	if len(c.responses) == 0 {
		return core.ToolResponse{}, fmt.Errorf("no tool response configured")
	}
	response := c.responses[0]
	c.responses = c.responses[1:]
	response.CallID = req.CallID
	response.ToolName = req.ToolName
	response.StartedAt = time.Now()
	response.EndedAt = response.StartedAt
	return response, nil
}

func (c *reliabilityToolCaller) Cancel(context.Context, string, string) error {
	return nil
}

func (c *reliabilityToolCaller) ToolDescriptors() []core.ToolDescriptor {
	return c.descriptors
}

func reliabilityConfig() core.RuntimeConfig {
	config := core.DefaultConfig()
	config.MaxStepReactTurns = 4
	config.MaxToolCallsPerStep = 8
	config.AsyncPollInitialBackoff = 0
	config.AsyncPollMaxBackoff = 0
	config.EnableContextCompress = false
	return config
}

func reliabilityStep() *core.PlanStep {
	return &core.PlanStep{
		StepID:         "step-1",
		Title:          "Run example tool",
		Objective:      "Run the example tool and use its actual outcome.",
		ExpectedOutput: "A verified terminal result.",
		SuggestedTools: []string{"Example.Run"},
		Status:         core.StepRunning,
		RiskLevel:      core.RiskReadOnly,
	}
}

func reliabilityDescriptor() core.ToolDescriptor {
	return core.ToolDescriptor{
		Name:        "Example.Run",
		Description: "Run the example operation.",
		RiskLevel:   core.RiskReadOnly,
		ArgsSchema: map[string]any{
			"type":     "object",
			"required": []any{"target_ids"},
			"properties": map[string]any{
				"target_ids": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "string"},
				},
				"max_rounds": map[string]any{
					"type":    "integer",
					"minimum": float64(1),
					"maximum": float64(10),
				},
			},
		},
	}
}

func requestText(request core.LLMRequest) string {
	var builder strings.Builder
	for _, message := range request.Messages {
		builder.WriteString(message.Content)
		builder.WriteByte('\n')
	}
	return builder.String()
}

func TestReActObservationExposesCallIDOutcomeAndError(t *testing.T) {
	llm := &reliabilitySequenceLLM{responses: []core.LLMResponse{
		{Content: `{"action":"tool_call","summary":"run","tool_call":{"tool_name":"Example.Run","reason":"run","args":{}}}`},
		{Content: `{"action":"fail_step","summary":"stop","failure":{"reason":"invalid arguments","recoverable":false}}`},
	}}
	validationErr := &core.ToolCallValidationError{
		Stage:    core.ToolValidationArguments,
		ToolName: "Example.Run",
		Message:  `missing required argument "target_ids"`,
	}
	tools := &reliabilityToolCaller{
		descriptors: []core.ToolDescriptor{reliabilityDescriptor()},
		errs:        []error{validationErr},
	}
	executor := NewReActExecutor(llm, tools, nil, &reliabilityIDGenerator{}, nil, reliabilityConfig(), nil, nil)

	result := executor.RunStep(context.Background(), &StepContext{
		TaskID:    "task-1",
		UserInput: "run the example operation",
	}, reliabilityStep())

	if result.Status != core.StepFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("tool calls = %d, want 1", len(result.ToolCalls))
	}
	if len(llm.requests) < 2 {
		t.Fatalf("LLM requests = %d, want at least 2", len(llm.requests))
	}
	observationPrompt := requestText(llm.requests[1])
	if !strings.Contains(observationPrompt, result.ToolCalls[0].CallID) {
		t.Fatalf("observation prompt does not expose call_id %q:\n%s", result.ToolCalls[0].CallID, observationPrompt)
	}
	if !strings.Contains(observationPrompt, validationErr.Message) {
		t.Fatalf("observation prompt does not expose validation error:\n%s", observationPrompt)
	}
	if !strings.Contains(observationPrompt, "call_status") {
		t.Fatalf("observation prompt does not expose call status:\n%s", observationPrompt)
	}
}

func TestReActNonTerminalPollsDoNotExhaustReasoningBudget(t *testing.T) {
	llm := &reliabilitySequenceLLM{responses: []core.LLMResponse{
		{Content: `{"action":"tool_call","summary":"poll 1","tool_call":{"tool_name":"Example.Run","reason":"poll","args":{"target_ids":["host-1"]}}}`},
		{Content: `{"action":"tool_call","summary":"poll 2","tool_call":{"tool_name":"Example.Run","reason":"poll","args":{"target_ids":["host-1"]}}}`},
		{Content: `{"action":"tool_call","summary":"poll 3","tool_call":{"tool_name":"Example.Run","reason":"poll","args":{"target_ids":["host-1"]}}}`},
		{Content: `{"action":"tool_call","summary":"poll 4","tool_call":{"tool_name":"Example.Run","reason":"poll","args":{"target_ids":["host-1"]}}}`},
		{Content: `{"action":"step_result","summary":"done","step_result":{"result":"operation completed","evidence":["id-8"],"confidence":"high"}}`},
	}}
	running := core.ToolResponse{
		Status:  core.ToolCallSuccess,
		Content: `{"status":"running"}`,
		Outcome: &core.ToolOutcome{OperationStatus: core.OperationRunning, Terminal: false},
	}
	succeeded := core.ToolResponse{
		Status:  core.ToolCallSuccess,
		Content: `{"status":"succeeded"}`,
		Outcome: &core.ToolOutcome{OperationStatus: core.OperationSucceeded, Terminal: true},
	}
	tools := &reliabilityToolCaller{
		descriptors: []core.ToolDescriptor{reliabilityDescriptor()},
		responses:   []core.ToolResponse{running, running, running, succeeded},
	}
	config := reliabilityConfig()
	config.MaxStepReactTurns = 2
	executor := NewReActExecutor(llm, tools, nil, &reliabilityIDGenerator{}, nil, config, nil, nil)

	result := executor.RunStep(context.Background(), &StepContext{
		TaskID:    "task-async",
		UserInput: "wait for the operation to complete",
	}, reliabilityStep())

	if result.Status != core.StepCompleted {
		t.Fatalf("status = %q, want completed; error=%v errors=%v", result.Status, result.Error, result.Errors)
	}
	if len(result.ToolCalls) != 4 {
		t.Fatalf("tool calls = %d, want 4", len(result.ToolCalls))
	}
	if result.Evidence[0] != "id-8" {
		t.Fatalf("evidence = %v, want terminal call id-8", result.Evidence)
	}
}

func TestReActRequestUsesExactDynamicToolSchema(t *testing.T) {
	llm := &reliabilitySequenceLLM{responses: []core.LLMResponse{
		{Content: `{"action":"fail_step","summary":"stop","failure":{"reason":"test","recoverable":false}}`},
	}}
	tools := &reliabilityToolCaller{descriptors: []core.ToolDescriptor{
		reliabilityDescriptor(),
		{
			Name:        "Example.Other",
			Description: "Other operation.",
			RiskLevel:   core.RiskReadOnly,
			ArgsSchema:  map[string]any{"type": "object"},
		},
	}}
	step := reliabilityStep()
	step.AllowedTools = []string{"Example.Run"}
	executor := NewReActExecutor(llm, tools, nil, &reliabilityIDGenerator{}, nil, reliabilityConfig(), nil, nil)

	executor.RunStep(context.Background(), &StepContext{
		TaskID:    "task-schema",
		UserInput: "run the example operation",
	}, step)

	if len(llm.requests) != 1 {
		t.Fatalf("LLM requests = %d, want 1", len(llm.requests))
	}
	format := llm.requests[0].ResponseFormat
	if format == nil || format.Type != "json_schema" || format.JSONSchema == nil {
		t.Fatalf("response format = %#v, want json_schema", format)
	}
	var schema map[string]any
	if err := json.Unmarshal(format.JSONSchema.Schema, &schema); err != nil {
		t.Fatalf("unmarshal response schema: %v", err)
	}
	encoded, _ := json.Marshal(schema)
	schemaText := string(encoded)
	if !strings.Contains(schemaText, `"const":"Example.Run"`) {
		t.Fatalf("response schema does not contain allowed tool: %s", schemaText)
	}
	if strings.Contains(schemaText, "Example.Other") {
		t.Fatalf("response schema exposes tool outside AllowedTools: %s", schemaText)
	}
	if !strings.Contains(schemaText, `"target_ids"`) || !strings.Contains(schemaText, `"max_rounds"`) {
		t.Fatalf("response schema does not contain exact tool args: %s", schemaText)
	}
}
