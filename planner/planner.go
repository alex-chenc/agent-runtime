package planner

import (
	"context"
	"fmt"
	"time"

	"github.com/alex-chenc/agent-runtime/core"
	"github.com/alex-chenc/agent-runtime/internal/ids"
)

// Planner generates structured execution plans using an LLM.
type Planner struct {
	llmClient core.LLMClient
	idGen     core.IDGenerator
	provider  core.PromptProvider
}

// New creates a new Planner.
func New(client core.LLMClient, idGen core.IDGenerator, provider core.PromptProvider) *Planner {
	if idGen == nil {
		idGen = ids.Generator{}
	}
	return &Planner{
		llmClient: client,
		idGen:     idGen,
		provider:  provider,
	}
}

// PlanInput contains the data needed to generate a plan.
type PlanInput struct {
	TaskID         string
	UserInput      string
	UserContext    map[string]any
	Tools          []core.ToolDescriptor
	MaxSteps       int
	Experience     string
	DisabledTools  []string
	AllowHighRisk  bool
	AllowDangerous bool
}

// Generate calls the LLM to generate a structured plan.
func (p *Planner) Generate(ctx context.Context, input PlanInput) (*core.Plan, error) {
	prompt, err := p.buildPrompt(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("planner: build prompt: %w", err)
	}

	timeout := 60 * time.Second
	if dl, ok := ctx.Deadline(); ok {
		timeout = time.Until(dl)
		if timeout <= 0 {
			return nil, fmt.Errorf("planner: context already expired")
		}
	}

	resp, err := p.llmClient.Complete(ctx, core.LLMRequest{
		TaskID:         input.TaskID,
		Purpose:        core.PurposePlan,
		Messages:       prompt.Messages,
		ResponseSchema: "plan_generation",
		Timeout:        timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("planner: LLM call failed: %w", err)
	}

	plan, err := ParsePlan(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("planner: parse plan: %w", err)
	}

	plan.PlanID = p.idGen.Generate()
	plan.Version = 1
	plan.CreatedAt = time.Now()
	plan.UpdatedAt = plan.CreatedAt

	// Assign step IDs
	for i := range plan.Steps {
		if plan.Steps[i].StepID == "" {
			plan.Steps[i].StepID = fmt.Sprintf("step_%d", i+1)
		}
		plan.Steps[i].Status = core.StepPending
		plan.Steps[i].CreatedBy = "planner"
	}

	return plan, nil
}

func (p *Planner) buildPrompt(ctx context.Context, input PlanInput) (core.PromptBundle, error) {
	if p.provider != nil {
		bundle, err := p.provider.Build(ctx, core.PromptRequest{
			TaskID:  input.TaskID,
			Purpose: core.PurposePlan,
		})
		if err == nil && (bundle.SystemPrompt != "" || len(bundle.Messages) > 0) {
			return bundle, nil
		}
	}

	// Build default prompt
	toolDesc := ""
	for _, t := range input.Tools {
		toolDesc += fmt.Sprintf("- %s: %s (risk: %s)\n", t.Name, t.Description, t.RiskLevel)
	}

	system := `You are an AI agent planner. Generate a structured execution plan as JSON.

Output a JSON object with this exact structure:
{
  "goal": "string describing the overall goal",
  "assumptions": ["list of assumptions"],
  "steps": [
    {
      "title": "step title",
      "objective": "what this step achieves",
      "expected_output": "what output is expected",
      "suggested_tools": ["tool_name"],
      "dependencies": ["step_1"],
      "risk_level": "read_only|low|high|dangerous"
    }
  ]
}

Rules:
- Steps must be actionable and have clear completion criteria
- Each step's objective must be specific, not vague like "continue analysis"
- Only use tools from the available list
- Dependencies must reference step IDs (step_1, step_2, etc.), NOT step titles
- Max steps: ` + fmt.Sprintf("%d", input.MaxSteps)

	userMsg := fmt.Sprintf("Task: %s\n\nAvailable tools:\n%s", input.UserInput, toolDesc)
	if input.Experience != "" {
		userMsg += "\n\nRelevant experience:\n" + input.Experience
	}
	if len(input.DisabledTools) > 0 {
		userMsg += fmt.Sprintf("\n\nDisabled tools: %v", input.DisabledTools)
	}

	return core.PromptBundle{
		SystemPrompt: system,
		Messages: []core.LLMMessage{
			{Role: "user", Content: userMsg},
		},
	}, nil
}
