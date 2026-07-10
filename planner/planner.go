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

// AssessResult holds the LLM's pre-assessment of whether a plan is needed.
type AssessResult struct {
	NeedsPlan      bool   `json:"needs_plan"`
	EstimatedSteps int    `json:"estimated_steps"`
	Reason         string `json:"reason"`
}

const assessmentSystemPrompt = `You are a task-complexity assessor. Determine whether the user's task needs a structured, multi-step execution plan.

Assessment rules:
- Set needs_plan=false when the task can be completed in one or two direct steps, such as a simple query, list, or single-resource detail lookup.
- Set needs_plan=true when the task has three or more meaningful steps, dependencies, multiple data sources, conditional branches, asynchronous state, or verification requirements.
- Assess the actual goal and available tool contracts. Do not apply a predefined business workflow.

Return exactly one JSON object and no other text. Start with { and end with }.
Schema: {"needs_plan":true,"estimated_steps":3,"reason":"concise rationale"}`

const planJSONRetryPrompt = `The previous response was not valid JSON. Return exactly one JSON object and no other text. Start with {. Use this schema: {"goal":"...","steps":[{"title":"...","objective":"...","expected_output":"..."}]}`

// Assess asks the LLM to quickly evaluate whether the task needs a structured plan.
// Tasks with fewer than 3 estimated steps are considered simple and skip planning.
func (p *Planner) Assess(ctx context.Context, input PlanInput) (*AssessResult, error) {
	toolList := ""
	for _, t := range input.Tools {
		toolList += fmt.Sprintf("- %s: %s\n", t.Name, t.Description)
	}

	userPrompt := fmt.Sprintf("Task:\n%s\n\nAvailable tools:\n%s", input.UserInput, toolList)
	if input.Experience != "" {
		userPrompt += "\n\nRelevant execution experience:\n" + input.Experience
	}

	timeout := 30 * time.Second
	if dl, ok := ctx.Deadline(); ok {
		timeout = time.Until(dl)
		if timeout <= 0 {
			return nil, fmt.Errorf("planner: context already expired")
		}
	}

	resp, err := p.llmClient.Complete(ctx, core.LLMRequest{
		TaskID:         input.TaskID,
		Purpose:        core.PurposePlan,
		Messages:       []core.LLMMessage{{Role: "system", Content: assessmentSystemPrompt}, {Role: "user", Content: userPrompt}},
		ResponseSchema: "plan_assessment",
		Timeout:        timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("planner: assess LLM call failed: %w", err)
	}

	result, err := ParseAssess(resp.Content)
	if err != nil {
		// 如果解析失败，默认需要计划（保守策略）
		return &AssessResult{NeedsPlan: true, EstimatedSteps: 3, Reason: "assessment parse failed, defaulting to plan"}, nil
	}

	return result, nil
}

// Generate calls the LLM to generate a structured plan.
// If the first attempt fails to parse, it retries once with a stricter JSON-only prompt.
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
		// Retry with a stricter prompt that includes the failed response
		retryMessages := append(prompt.Messages, core.LLMMessage{
			Role:    "assistant",
			Content: resp.Content,
		}, core.LLMMessage{
			Role:    "user",
			Content: planJSONRetryPrompt,
		})
		retryResp, retryErr := p.llmClient.Complete(ctx, core.LLMRequest{
			TaskID:         input.TaskID,
			Purpose:        core.PurposePlan,
			Messages:       retryMessages,
			ResponseSchema: "plan_generation",
			Timeout:        timeout,
		})
		if retryErr != nil {
			return nil, fmt.Errorf("planner: parse plan: %w (retry also failed: %v)", err, retryErr)
		}
		plan, err = ParsePlan(retryResp.Content)
		if err != nil {
			return nil, fmt.Errorf("planner: parse plan: %w", err)
		}
	}

	plan.PlanID = p.idGen.Generate()
	plan.Version = 1
	plan.NeedsPlan = true
	plan.EstSteps = len(plan.Steps)
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
	applyGeneratedStepToolBoundaries(plan, input.Tools)
	normalizePlanStepRiskLevels(plan, input.Tools)

	return plan, nil
}

// applyGeneratedStepToolBoundaries turns the planner's tool selection into an
// executable boundary. The task remains model-planned, but a later ReAct turn
// cannot invent a tool outside the registry or cross into another plan step's
// responsibility. A correction can still replace the step when a different
// registered tool is genuinely required.
func applyGeneratedStepToolBoundaries(plan *core.Plan, descriptors []core.ToolDescriptor) {
	if plan == nil || len(descriptors) == 0 {
		return
	}
	known := make(map[string]struct{}, len(descriptors))
	for _, descriptor := range descriptors {
		known[descriptor.Name] = struct{}{}
	}
	for index := range plan.Steps {
		seen := make(map[string]struct{}, len(plan.Steps[index].SuggestedTools))
		allowed := make([]string, 0, len(plan.Steps[index].SuggestedTools))
		for _, toolName := range plan.Steps[index].SuggestedTools {
			if _, ok := known[toolName]; !ok {
				continue
			}
			if _, duplicate := seen[toolName]; duplicate {
				continue
			}
			seen[toolName] = struct{}{}
			allowed = append(allowed, toolName)
		}
		plan.Steps[index].AllowedTools = allowed
	}
}

func normalizePlanStepRiskLevels(plan *core.Plan, descriptors []core.ToolDescriptor) {
	if plan == nil || len(descriptors) == 0 {
		return
	}
	risks := make(map[string]core.RiskLevel, len(descriptors))
	for _, descriptor := range descriptors {
		risks[descriptor.Name] = descriptor.RiskLevel
	}
	for index := range plan.Steps {
		var (
			highest core.RiskLevel
			found   bool
		)
		for _, toolName := range plan.Steps[index].SuggestedTools {
			risk, ok := risks[toolName]
			if !ok {
				continue
			}
			if !found || riskRank(risk) > riskRank(highest) {
				highest = risk
			}
			found = true
		}
		if found {
			plan.Steps[index].RiskLevel = highest
		}
	}
}

func riskRank(risk core.RiskLevel) int {
	switch risk {
	case core.RiskDangerous:
		return 4
	case core.RiskHigh:
		return 3
	case core.RiskLow:
		return 2
	case core.RiskReadOnly:
		return 1
	default:
		return 0
	}
}

// GenerateNoPlan creates a minimal single-step plan for simple tasks that don't need structured planning.
// The single step will be executed directly by the ReAct executor.
func (p *Planner) GenerateNoPlan(input PlanInput) *core.Plan {
	return &core.Plan{
		PlanID:    p.idGen.Generate(),
		Version:   1,
		Goal:      input.UserInput,
		NeedsPlan: false,
		EstSteps:  1,
		Steps: []core.PlanStep{
			{
				StepID:         "step_1",
				Title:          "Execute request",
				Objective:      input.UserInput,
				ExpectedOutput: "Complete the user request",
				Status:         core.StepPending,
				CreatedBy:      "planner_skip",
				RiskLevel:      core.RiskReadOnly,
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
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
