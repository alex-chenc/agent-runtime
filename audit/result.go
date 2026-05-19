package audit

import (
	"encoding/json"
	"fmt"

	"github.com/alex-chenc/agent-runtime/core"
	"github.com/alex-chenc/agent-runtime/internal/textutil"
)

type auditJSON struct {
	Drifted        bool     `json:"drifted"`
	RiskLevel      string   `json:"risk_level"`
	Findings       []string `json:"findings,omitempty"`
	Decision       string   `json:"decision"`
	CorrectionHint string   `json:"correction_hint,omitempty"`
	NeedExperience bool     `json:"need_experience"`
	NeedUserInput  bool     `json:"need_user_input"`
	ShouldExit     bool     `json:"should_exit"`
	ExitReason     string   `json:"exit_reason,omitempty"`
}

// ParseAuditResult parses LLM output into an AuditResult.
func ParseAuditResult(content string) (*core.AuditResult, error) {
	jsonStr := textutil.ExtractJSON(content)
	if jsonStr == "" {
		jsonStr = content
	}

	var raw auditJSON
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, fmt.Errorf("parse audit: %w", err)
	}

	decision := core.AuditContinue
	switch raw.Decision {
	case "continue":
		decision = core.AuditContinue
	case "minor_adjustment":
		decision = core.AuditMinorAdjustment
	case "correct_plan":
		decision = core.AuditCorrectPlan
	case "request_experience":
		decision = core.AuditRequestExperience
	case "summarize_now":
		decision = core.AuditSummarizeNow
	case "fail":
		decision = core.AuditFail
	}

	return &core.AuditResult{
		Drifted:        raw.Drifted,
		RiskLevel:      core.RiskLevel(raw.RiskLevel),
		Findings:       raw.Findings,
		Decision:       decision,
		CorrectionHint: raw.CorrectionHint,
		NeedExperience: raw.NeedExperience,
		NeedUserInput:  raw.NeedUserInput,
		ShouldExit:     raw.ShouldExit,
		ExitReason:     core.ExitReason(raw.ExitReason),
	}, nil
}
