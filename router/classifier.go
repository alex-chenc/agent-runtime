package router

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/alex-chenc/agent-runtime/core"
	"github.com/alex-chenc/agent-runtime/internal/textutil"
)

func buildClassificationSystemPrompt(catalogSummary string) string {
	return fmt.Sprintf(`You are a task classifier and prompt-fragment selector. Analyze the user message and:
1. Classify the task type and complexity.
2. Select only relevant fragments from the current fragment catalog.

## Output format
Return JSON only:
{"task_type":"greeting|query|analysis|investigation|remediation|other","intent":"concise semantic intent","complexity":"simple|moderate|complex","action":"direct_reply|simple_call|full_plan","fragments":["fragment_name"],"reason":"concise rationale"}

## Complexity
- simple: one or two direct steps; use direct_reply when no tool is needed or simple_call when a direct tool call is enough.
- moderate or complex: dependencies, three or more meaningful steps, multiple objects or sources, conditional branches, asynchronous state, or verification; use full_plan.

## Available fragment catalog
%s

## Selection rules
- Always include base_assistant.
- Always include react_format.
- Select only fragments whose generic guidance is relevant to the current goal.
- Do not infer a fixed business workflow from fragment names.
- Select two to five fragments.
- Keep fragment names and all machine identifiers in exact English catalog form. Natural-language intent and reason may follow the user's language.`, catalogSummary)
}

// classify 第二级：LLM 语义分析 + 片段选择
func (r *Router) classify(ctx context.Context, input RouteInput) (*core.TaskClassification, error) {
	catalogSummary := r.buildCatalogSummary()

	systemPrompt := buildClassificationSystemPrompt(catalogSummary)
	userPrompt := fmt.Sprintf("User message:\n%s\n\nAvailable tool count: %d", input.UserMessage, len(input.Tools))

	timeout := r.config.LLMTimeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	if dl, ok := ctx.Deadline(); ok {
		if remaining := time.Until(dl); remaining < timeout {
			timeout = remaining
		}
	}

	temperature := float32(r.config.LLMTemperature)
	if temperature == 0 {
		temperature = 0.1
	}

	resp, err := r.llmClient.Complete(ctx, core.LLMRequest{
		TaskID:  input.TaskID,
		Purpose: core.PurposeClassify,
		Messages: []core.LLMMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		ResponseSchema: "task_classification",
		Timeout:        timeout,
		Temperature:    &temperature,
	})
	if err != nil {
		return nil, fmt.Errorf("router: classify LLM call failed: %w", err)
	}

	classification, err := parseClassification(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("router: parse classification failed: %w", err)
	}

	// 验证片段名称有效性
	classification.Fragments = r.validateFragments(classification.Fragments)

	// 确保基础片段存在
	classification.Fragments = ensureBaseFragments(classification.Fragments)

	return classification, nil
}

// parseClassification 解析 LLM 分类结果
func parseClassification(content string) (*core.TaskClassification, error) {
	jsonStr := textutil.ExtractJSON(content)
	if jsonStr == "" {
		jsonStr = content
	}

	var raw core.TaskClassification
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse classification JSON: %w", err)
	}

	// 验证必填字段
	if raw.TaskType == "" {
		return nil, fmt.Errorf("classification missing task_type")
	}
	if raw.Action == "" {
		// 根据复杂度推断动作
		if raw.Complexity == "simple" {
			raw.Action = core.ActionSimpleCall
		} else {
			raw.Action = core.ActionFullPlan
		}
	}

	return &raw, nil
}

// validateFragments 验证片段名称是否在目录中
func (r *Router) validateFragments(names []string) []string {
	valid := make(map[string]bool)
	for _, f := range r.fragments {
		valid[f.Name] = true
	}

	var result []string
	for _, name := range names {
		if valid[name] {
			result = append(result, name)
		}
	}
	return result
}

// ensureBaseFragments 确保基础片段存在
func ensureBaseFragments(fragments []string) []string {
	hasBase := false
	hasReact := false
	for _, f := range fragments {
		if f == "base_assistant" {
			hasBase = true
		}
		if f == "react_format" {
			hasReact = true
		}
	}

	result := make([]string, 0, len(fragments)+2)
	if !hasBase {
		result = append(result, "base_assistant")
	}
	result = append(result, fragments...)
	if !hasReact {
		result = append(result, "react_format")
	}
	return result
}
