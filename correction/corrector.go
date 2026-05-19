package correction

import (
	"context"
	"fmt"
	"time"

	"github.com/alex-chenc/agent-runtime/core"
	"github.com/alex-chenc/agent-runtime/internal/ids"
)

// Corrector generates plan corrections based on audit or reflection results.
type Corrector struct {
	llmClient core.LLMClient
	idGen     core.IDGenerator
}

// New creates a new Corrector.
func New(client core.LLMClient, idGen core.IDGenerator) *Corrector {
	if idGen == nil {
		idGen = ids.Generator{}
	}
	return &Corrector{llmClient: client, idGen: idGen}
}

// CorrectionInput contains data for generating a correction.
type CorrectionInput struct {
	TaskID        string
	CurrentPlan   *core.Plan
	Trigger       string
	Hint          string
	CompletedIDs  []string
	FailedIDs     []string
	AllowNewSteps bool
	MaxSteps      int
}

// Correct generates a plan correction.
func (c *Corrector) Correct(ctx context.Context, input CorrectionInput) (*core.CorrectionResult, error) {
	if c.llmClient == nil {
		return c.defaultCorrection(input), nil
	}

	stepsSummary := ""
	for _, s := range input.CurrentPlan.Steps {
		stepsSummary += fmt.Sprintf("- %s [%s]: %s\n", s.StepID, s.Status, s.Title)
	}

	prompt := fmt.Sprintf(`Correct this plan:

Trigger: %s
Hint: %s
Current plan steps:
%s
Completed step IDs: %v
Failed step IDs: %v
Allow adding new steps: %v
Max total steps: %d

Respond with JSON:
{
  "reason": "why correction is needed",
  "actions": [
    {"type": "skip_step|add_step|replace_step", "step_id": "...", "reason": "..."}
  ]
}`, input.Trigger, input.Hint, stepsSummary, input.CompletedIDs, input.FailedIDs, input.AllowNewSteps, input.MaxSteps)

	resp, err := c.llmClient.Complete(ctx, core.LLMRequest{
		TaskID:  input.TaskID,
		Purpose: core.PurposeCorrect,
		Messages: []core.LLMMessage{
			{Role: "user", Content: prompt},
		},
		Timeout: 30 * time.Second,
	})
	if err != nil {
		return c.defaultCorrection(input), nil
	}

	result, err := ParseCorrection(resp.Content)
	if err != nil {
		return c.defaultCorrection(input), nil
	}

	result.CorrectionID = c.idGen.Generate()
	result.Trigger = input.Trigger
	result.FromPlanVersion = input.CurrentPlan.Version
	result.ToPlanVersion = input.CurrentPlan.Version + 1
	result.CreatedAt = time.Now()
	return result, nil
}

func (c *Corrector) defaultCorrection(input CorrectionInput) *core.CorrectionResult {
	return &core.CorrectionResult{
		CorrectionID:    c.idGen.Generate(),
		Trigger:         input.Trigger,
		FromPlanVersion: input.CurrentPlan.Version,
		ToPlanVersion:   input.CurrentPlan.Version + 1,
		Reason:          input.Hint,
		Valid:           true,
		CreatedAt:       time.Now(),
	}
}
