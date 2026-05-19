package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/alex-chenc/agent-runtime/core"
	"github.com/alex-chenc/agent-runtime/internal/ids"
)

// Auditor performs periodic audits to check if execution is on track.
type Auditor struct {
	llmClient core.LLMClient
	idGen     core.IDGenerator
}

// New creates a new Auditor.
func New(client core.LLMClient, idGen core.IDGenerator) *Auditor {
	if idGen == nil {
		idGen = ids.Generator{}
	}
	return &Auditor{llmClient: client, idGen: idGen}
}

// AuditInput contains data for an audit.
type AuditInput struct {
	TaskID         string
	Goal           string
	CompletedSteps []string
	FailedSteps    []string
	CurrentStep    string
	RecentErrors   []core.RuntimeError
	Trigger        string
}

// Audit checks if execution is aligned with the goal.
func (a *Auditor) Audit(ctx context.Context, input AuditInput) (*core.AuditResult, error) {
	if a.llmClient == nil {
		return a.defaultAudit(input), nil
	}

	completedSummary := "none"
	if len(input.CompletedSteps) > 0 {
		completedSummary = fmt.Sprintf("%v", input.CompletedSteps)
	}

	prompt := fmt.Sprintf(`Audit this task execution:

Goal: %s
Completed steps: %s
Failed steps: %v
Current step: %s
Recent errors: %d
Trigger: %s

Is the execution on track? Respond with JSON:
{
  "drifted": false,
  "risk_level": "read_only",
  "findings": [],
  "decision": "continue|minor_adjustment|correct_plan|summarize_now|fail",
  "correction_hint": "",
  "should_exit": false
}`, input.Goal, completedSummary, input.FailedSteps, input.CurrentStep, len(input.RecentErrors), input.Trigger)

	resp, err := a.llmClient.Complete(ctx, core.LLMRequest{
		TaskID:  input.TaskID,
		Purpose: core.PurposeAudit,
		Messages: []core.LLMMessage{
			{Role: "user", Content: prompt},
		},
		Timeout: 30 * time.Second,
	})
	if err != nil {
		return a.defaultAudit(input), nil
	}

	result, err := ParseAuditResult(resp.Content)
	if err != nil {
		return a.defaultAudit(input), nil
	}

	result.AuditID = a.idGen.Generate()
	result.Trigger = input.Trigger
	result.CreatedAt = time.Now()
	return result, nil
}

func (a *Auditor) defaultAudit(input AuditInput) *core.AuditResult {
	return &core.AuditResult{
		AuditID:   a.idGen.Generate(),
		Trigger:   input.Trigger,
		Drifted:   false,
		RiskLevel: core.RiskReadOnly,
		Decision:  core.AuditContinue,
		CreatedAt: time.Now(),
	}
}
