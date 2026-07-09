package planner

import (
	"encoding/json"
	"fmt"

	"github.com/alex-chenc/agent-runtime/core"
	"github.com/alex-chenc/agent-runtime/internal/textutil"
)

// planJSON is the intermediate JSON structure for parsing LLM output.
type planJSON struct {
	Goal        string     `json:"goal"`
	Assumptions []string   `json:"assumptions,omitempty"`
	Steps       []stepJSON `json:"steps"`
}

type stepJSON struct {
	Title          string   `json:"title"`
	Objective      string   `json:"objective"`
	ExpectedOutput string   `json:"expected_output"`
	SuggestedTools []string `json:"suggested_tools,omitempty"`
	Dependencies   []string `json:"dependencies,omitempty"`
	RiskLevel      string   `json:"risk_level,omitempty"`
}

// ParseAssess parses a JSON string into an AssessResult.
func ParseAssess(content string) (*AssessResult, error) {
	jsonStr := textutil.ExtractJSON(content)
	if jsonStr == "" {
		jsonStr = content
	}

	var raw AssessResult
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse assess JSON: %w", err)
	}
	return &raw, nil
}

// ParsePlan parses a JSON string into a Plan.
func ParsePlan(content string) (*core.Plan, error) {
	jsonStr := textutil.ExtractJSON(content)
	if jsonStr == "" {
		jsonStr = content
	}

	var raw planJSON
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %w", err)
	}

	if raw.Goal == "" {
		// 容错：goal 为空时不报错，使用默认值
		raw.Goal = "Execute the user task"
	}
	if len(raw.Steps) == 0 {
		return nil, fmt.Errorf("plan has no steps")
	}

	steps := make([]core.PlanStep, len(raw.Steps))
	for i, s := range raw.Steps {
		if s.Title == "" {
			return nil, fmt.Errorf("step %d has empty title", i)
		}
		if s.Objective == "" {
			return nil, fmt.Errorf("step %q has empty objective", s.Title)
		}
		risk := core.RiskReadOnly
		if s.RiskLevel != "" {
			risk = core.RiskLevel(s.RiskLevel)
		}
		steps[i] = core.PlanStep{
			Title:          s.Title,
			Objective:      s.Objective,
			ExpectedOutput: s.ExpectedOutput,
			SuggestedTools: s.SuggestedTools,
			Dependencies:   s.Dependencies,
			RiskLevel:      risk,
		}
	}

	return &core.Plan{
		Goal:        raw.Goal,
		Assumptions: raw.Assumptions,
		Steps:       steps,
	}, nil
}
