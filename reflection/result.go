package reflection

import (
	"encoding/json"
	"fmt"

	"github.com/chenchen511/agent-runtime/core"
	"github.com/chenchen511/agent-runtime/internal/textutil"
)

type reflectionJSON struct {
	RootCause       string   `json:"root_cause"`
	Impact          string   `json:"impact"`
	Recoverable     bool     `json:"recoverable"`
	Recommendation  string   `json:"recommendation"`
	DisableTools    []string `json:"disable_tools,omitempty"`
	CorrectionHint  string   `json:"correction_hint,omitempty"`
	ExperienceQuery string   `json:"experience_query,omitempty"`
	ReusableLesson  string   `json:"reusable_lesson,omitempty"`
}

// ParseReflection parses LLM output into a ReflectionResult.
func ParseReflection(content string) (*core.ReflectionResult, error) {
	jsonStr := textutil.ExtractJSON(content)
	if jsonStr == "" {
		jsonStr = content
	}

	var raw reflectionJSON
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, fmt.Errorf("parse reflection: %w", err)
	}

	if raw.RootCause == "" {
		return nil, fmt.Errorf("reflection has empty root_cause")
	}

	rec := core.ReflectRetryStep
	switch raw.Recommendation {
	case "retry_step":
		rec = core.ReflectRetryStep
	case "skip_step":
		rec = core.ReflectSkipStep
	case "correct_plan":
		rec = core.ReflectCorrectPlan
	case "request_experience":
		rec = core.ReflectRequestExperience
	case "summarize_now":
		rec = core.ReflectSummarizeNow
	case "fail":
		rec = core.ReflectFail
	}

	return &core.ReflectionResult{
		RootCause:       raw.RootCause,
		Impact:          raw.Impact,
		Recoverable:     raw.Recoverable,
		Recommendation:  rec,
		DisableTools:    raw.DisableTools,
		CorrectionHint:  raw.CorrectionHint,
		ExperienceQuery: raw.ExperienceQuery,
		ReusableLesson:  raw.ReusableLesson,
	}, nil
}
