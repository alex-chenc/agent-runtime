package executor

import (
	"encoding/json"
	"sort"

	"github.com/alex-chenc/agent-runtime/core"
)

type toolDescriptorProvider interface {
	ToolDescriptors() []core.ToolDescriptor
}

func (e *ReActExecutor) reactResponseFormat(step *core.PlanStep) *core.ResponseFormat {
	provider, ok := e.toolGW.(toolDescriptorProvider)
	if !ok {
		return &core.ResponseFormat{Type: "json_object"}
	}

	descriptors := provider.ToolDescriptors()
	sort.SliceStable(descriptors, func(i, j int) bool {
		return descriptors[i].Name < descriptors[j].Name
	})

	allowed := make(map[string]struct{}, len(step.AllowedTools))
	for _, name := range step.AllowedTools {
		allowed[name] = struct{}{}
	}

	toolBranches := make([]any, 0, len(descriptors))
	for _, descriptor := range descriptors {
		if len(allowed) > 0 {
			if _, ok := allowed[descriptor.Name]; !ok {
				continue
			}
		}
		argsSchema := cloneSchema(descriptor.ArgsSchema)
		if len(argsSchema) == 0 {
			argsSchema = map[string]any{"type": "object"}
		}
		toolBranches = append(toolBranches, map[string]any{
			"type": "object",
			"properties": map[string]any{
				"tool_name": map[string]any{"const": descriptor.Name},
				"reason":    map[string]any{"type": "string"},
				"args":      argsSchema,
			},
			"required":             []any{"tool_name", "reason", "args"},
			"additionalProperties": false,
		})
	}

	actionBranches := make([]any, 0, 4)
	if len(toolBranches) > 0 {
		actionBranches = append(actionBranches, reactActionBranch(
			"tool_call",
			"tool_call",
			map[string]any{"oneOf": toolBranches},
		))
	}
	actionBranches = append(actionBranches,
		reactActionBranch("step_result", "step_result", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"result":     map[string]any{"type": "string", "minLength": 1},
				"evidence":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"confidence": map[string]any{"type": "string", "enum": []any{"low", "medium", "high"}},
			},
			"required":             []any{"result", "evidence", "confidence"},
			"additionalProperties": false,
		}),
		reactActionBranch("request_experience", "experience_request", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query":  map[string]any{"type": "string", "minLength": 1},
				"reason": map[string]any{"type": "string"},
			},
			"required":             []any{"query", "reason"},
			"additionalProperties": false,
		}),
		reactActionBranch("fail_step", "failure", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"reason":      map[string]any{"type": "string", "minLength": 1},
				"recoverable": map[string]any{"type": "boolean"},
			},
			"required":             []any{"reason", "recoverable"},
			"additionalProperties": false,
		}),
	)

	schema, err := json.Marshal(map[string]any{
		"type":  "object",
		"oneOf": actionBranches,
	})
	if err != nil {
		return &core.ResponseFormat{Type: "json_object"}
	}
	return &core.ResponseFormat{
		Type: "json_schema",
		JSONSchema: &core.ResponseFormatSchema{
			Name:        "react_action",
			Description: "One valid ReAct action constrained to the current runtime tool registry.",
			Schema:      schema,
			Strict:      false,
		},
	}
}

func reactActionBranch(action, detailName string, detailSchema map[string]any) map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action":   map[string]any{"const": action},
			"summary":  map[string]any{"type": "string"},
			detailName: detailSchema,
		},
		"required":             []any{"action", "summary", detailName},
		"additionalProperties": false,
	}
}

func cloneSchema(schema map[string]any) map[string]any {
	if len(schema) == 0 {
		return nil
	}
	encoded, err := json.Marshal(schema)
	if err != nil {
		return nil
	}
	var cloned map[string]any
	if err := json.Unmarshal(encoded, &cloned); err != nil {
		return nil
	}
	return cloned
}
