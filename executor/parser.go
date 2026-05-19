package executor

import (
	"encoding/json"
	"fmt"

	"github.com/alex-chenc/agent-runtime/core"
	"github.com/alex-chenc/agent-runtime/internal/textutil"
)

// actionJSON is the intermediate JSON structure for parsing ReAct output.
type actionJSON struct {
	Action     string          `json:"action"`
	Type       string          `json:"type"` // alias for action (LLM fallback)
	Summary    string          `json:"summary"`
	ToolCall   *toolCallJSON   `json:"tool_call,omitempty"`
	StepResult *stepResultJSON `json:"step_result,omitempty"`
	Experience *experienceJSON `json:"experience_request,omitempty"`
	Failure    *failureJSON    `json:"failure,omitempty"`
	// Degraded format: tool_name/tool_args at top level (LLM fallback after tool errors)
	ToolNameTop string         `json:"tool_name"`
	ToolArgsTop map[string]any `json:"tool_args"`
	ArgsTop     map[string]any `json:"args"` // fallback: LLM sometimes uses "args" at top level
}

type toolCallJSON struct {
	ToolName string         `json:"tool_name"`
	Reason   string         `json:"reason"`
	Args     map[string]any `json:"args"`
}

type stepResultJSON struct {
	Result     string   `json:"result"`
	Evidence   []string `json:"evidence,omitempty"`
	Confidence string   `json:"confidence,omitempty"`
}

type experienceJSON struct {
	Query  string `json:"query"`
	Reason string `json:"reason"`
}

type failureJSON struct {
	Reason      string `json:"reason"`
	Recoverable *bool  `json:"recoverable,omitempty"`
}

// ParseAction parses a JSON string into a ReactAction.
func ParseAction(content string) (core.ReactAction, error) {
	jsonStr := textutil.ExtractJSON(content)
	if jsonStr == "" {
		jsonStr = content
	}

	var raw actionJSON
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return core.ReactAction{}, fmt.Errorf("failed to parse action JSON: %w", err)
	}

	action := core.ReactAction{
		Summary: raw.Summary,
	}

	// Resolve action type: prefer "action", fall back to "type" (LLM degraded format)
	actionType := raw.Action
	if actionType == "" {
		actionType = raw.Type
	}

	switch actionType {
	case "tool_call":
		// Handle both normal format (tool_call object) and degraded format (tool_name at top level)
		if raw.ToolCall != nil {
			action.Type = core.ActionToolCall
			action.ToolName = raw.ToolCall.ToolName
			action.ToolArgs = raw.ToolCall.Args
			if action.Summary == "" {
				action.Summary = raw.ToolCall.Reason
			}
		} else if raw.ToolNameTop != "" {
			// Degraded format: tool_name at top level (LLM fallback after tool errors)
			action.Type = core.ActionToolCall
			action.ToolName = raw.ToolNameTop
			// Try tool_args first, then args (LLM uses either)
			if raw.ToolArgsTop != nil {
				action.ToolArgs = raw.ToolArgsTop
			} else {
				action.ToolArgs = raw.ArgsTop
			}
		} else {
			return core.ReactAction{}, fmt.Errorf("tool_call action missing tool_call field")
		}
		if action.ToolName == "" {
			return core.ReactAction{}, fmt.Errorf("tool_call missing tool_name")
		}

	case "step_result":
		if raw.StepResult == nil {
			return core.ReactAction{}, fmt.Errorf("step_result action missing step_result field")
		}
		if raw.StepResult.Result == "" {
			return core.ReactAction{}, fmt.Errorf("step_result missing result")
		}
		action.Type = core.ActionStepResult
		action.StepResult = raw.StepResult.Result
		action.Evidence = raw.StepResult.Evidence
		action.Confidence = raw.StepResult.Confidence

	case "request_experience":
		if raw.Experience == nil {
			return core.ReactAction{}, fmt.Errorf("request_experience action missing experience_request field")
		}
		if raw.Experience.Query == "" {
			return core.ReactAction{}, fmt.Errorf("request_experience missing query")
		}
		action.Type = core.ActionRequestExperience
		action.NeedsExperience = true
		action.ExperienceQuery = raw.Experience.Query
		if action.Summary == "" {
			action.Summary = raw.Experience.Reason
		}

	case "need_user_input":
		action.Type = core.ActionNeedUserInput
		action.NeedsUserInput = true

	case "fail_step":
		action.Type = core.ActionFailStep
		if raw.Failure != nil {
			action.FailureReason = raw.Failure.Reason
			action.Recoverable = raw.Failure.Recoverable
		}

	default:
		return core.ReactAction{}, fmt.Errorf("unknown action type: %q", raw.Action)
	}

	return action, nil
}
