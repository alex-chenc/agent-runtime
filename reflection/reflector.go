package reflection

import (
	"context"
	"fmt"
	"time"

	"github.com/alex-chenc/agent-runtime/core"
	"github.com/alex-chenc/agent-runtime/internal/ids"
)

// Reflector performs error reflection to analyze failures and suggest recovery.
type Reflector struct {
	llmClient core.LLMClient
	idGen     core.IDGenerator
}

// New creates a new Reflector.
func New(client core.LLMClient, idGen core.IDGenerator) *Reflector {
	if idGen == nil {
		idGen = ids.Generator{}
	}
	return &Reflector{llmClient: client, idGen: idGen}
}

// ReflectInput contains data for a reflection.
type ReflectInput struct {
	TaskID     string
	StepID     string
	Trigger    string
	Error      core.RuntimeError
	StepResult string
	PlanGoal   string
}

// Reflect analyzes a failure and produces a reflection result.
func (r *Reflector) Reflect(ctx context.Context, input ReflectInput) (*core.ReflectionResult, error) {
	if r.llmClient == nil {
		return r.defaultReflection(input), nil
	}

	prompt := fmt.Sprintf(`Analyze this failure and suggest recovery:

Task goal: %s
Step: %s
Trigger: %s
Error: %s
Step result so far: %s

Respond with JSON:
{
  "root_cause": "why this failed",
  "impact": "how this affects the task",
  "recoverable": true/false,
  "recommendation": "retry_step|skip_step|correct_plan|request_experience|summarize_now|fail",
  "correction_hint": "hint for plan correction if applicable",
  "experience_query": "query to find relevant experience (for request_experience recommendation)",
  "reusable_lesson": "what to remember for similar cases"
}`, input.PlanGoal, input.StepID, input.Trigger, input.Error.Message, input.StepResult)

	resp, err := r.llmClient.Complete(ctx, core.LLMRequest{
		TaskID:  input.TaskID,
		StepID:  input.StepID,
		Purpose: core.PurposeReflect,
		Messages: []core.LLMMessage{
			{Role: "user", Content: prompt},
		},
		Timeout: 30 * time.Second,
	})
	if err != nil {
		return r.defaultReflection(input), nil
	}

	result, err := ParseReflection(resp.Content)
	if err != nil {
		return r.defaultReflection(input), nil
	}

	result.ReflectionID = r.idGen.Generate()
	result.Trigger = input.Trigger
	result.CreatedAt = time.Now()
	return result, nil
}

func (r *Reflector) defaultReflection(input ReflectInput) *core.ReflectionResult {
	return &core.ReflectionResult{
		ReflectionID:   r.idGen.Generate(),
		Trigger:        input.Trigger,
		RootCause:      input.Error.Message,
		Recoverable:    input.Error.Recoverable,
		Recommendation: core.ReflectRetryStep,
		CreatedAt:      time.Now(),
	}
}
