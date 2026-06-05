package router

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/alex-chenc/agent-runtime/core"
	"github.com/alex-chenc/agent-runtime/internal/textutil"
)

// classify 第二级：LLM 语义分析 + 片段选择
func (r *Router) classify(ctx context.Context, input RouteInput) (*core.TaskClassification, error) {
	catalogSummary := r.buildCatalogSummary()

	systemPrompt := fmt.Sprintf(`你是任务分类器和提示词选择器。分析用户消息，完成两个任务：
1. 分类任务类型和复杂度
2. 从提示词目录中选择需要的片段

## 输出格式（严格只输出JSON）
{"task_type":"类型","intent":"意图","complexity":"simple/moderate/complex","action":"动作","fragments":["片段名"],"reason":"原因"}

## 任务类型
- greeting: 问候、闲聊
- query: 简单数据查询
- analysis: 安全分析
- investigation: 攻击调查
- remediation: 修复操作

## 复杂度判断
- simple: 1-2步可完成 → action: simple_call
- moderate/complex: 3步以上 → action: full_plan

## 可用提示词目录
%s

## 选择规则
- 必须包含 base_assistant
- 必须包含 react_format
- 根据任务类型选择对应的功能片段
- 不要选择与任务无关的片段
- 选择 2-5 个片段`, catalogSummary)

	userPrompt := fmt.Sprintf("用户消息：%s\n可用工具数：%d", input.UserMessage, len(input.Tools))

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
