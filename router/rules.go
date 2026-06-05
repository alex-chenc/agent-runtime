package router

import (
	"strings"

	"github.com/alex-chenc/agent-runtime/core"
)

// matchRules 第一级：规则匹配（不调 LLM）
// 返回：动作、选中的片段名称、是否匹配
func (r *Router) matchRules(message string) (core.TaskAction, []string, bool) {
	msg := strings.TrimSpace(message)

	// 1. 问候/闲聊检测 → direct_reply
	if isGreeting(msg) {
		return core.ActionDirectReply, nil, true
	}

	// 2. 简单查询检测 → simple_call + 基础片段
	if isSimpleQuery(msg) {
		fragments := []string{"base_assistant", "react_format"}
		// 尝试添加匹配的功能片段
		if matched := matchDomainFragment(msg, r.fragments); matched != "" {
			fragments = insertFragment(fragments, matched)
		}
		return core.ActionSimpleCall, fragments, true
	}

	return "", nil, false
}

// isGreeting 判断是否为问候/闲聊
func isGreeting(msg string) bool {
	// 长度超过阈值不是问候
	if len(msg) > 15 {
		return false
	}

	greetings := []string{
		"你好", "您好", "hello", "hi", "hey", "嗨", "哈喽",
		"早上好", "下午好", "晚上好", "早安", "晚安",
		"在吗", "在不在", "你是谁", "干嘛",
		"谢谢", "感谢", "thanks", "thank you",
		"好的", "ok", "嗯", "哦", "行", "可以",
	}

	lower := strings.ToLower(msg)
	for _, g := range greetings {
		if lower == g {
			return true
		}
	}
	return false
}

// isSimpleQuery 判断是否为简单查询
func isSimpleQuery(msg string) bool {
	simplePatterns := []string{
		"查看", "查询", "列出", "列表", "显示", "获取",
		"有哪些", "多少", "状态",
		"list", "show", "get", "find", "check",
	}

	lower := strings.ToLower(msg)
	for _, p := range simplePatterns {
		if strings.Contains(lower, p) {
			// 查询类消息不应太长
			if len(msg) <= 30 {
				return true
			}
		}
	}
	return false
}

// matchDomainFragment 根据消息内容匹配功能片段
func matchDomainFragment(msg string, fragments []core.PromptFragment) string {
	lower := strings.ToLower(msg)
	bestMatch := ""
	bestScore := 0

	for _, f := range fragments {
		// 跳过基础片段
		if f.Name == "base_assistant" || f.Name == "react_format" || f.Name == "plan_decision" {
			continue
		}

		score := 0
		for _, kw := range f.Keywords {
			if strings.Contains(lower, strings.ToLower(kw)) {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			bestMatch = f.Name
		}
	}

	if bestScore > 0 {
		return bestMatch
	}
	return ""
}

// insertFragment 将片段名插入到列表中（保持 base_assistant 在首位，react_format 在末位）
func insertFragment(fragments []string, name string) []string {
	// 检查是否已存在
	for _, f := range fragments {
		if f == name {
			return fragments
		}
	}
	// 插入到 base_assistant 之后、react_format 之前
	if len(fragments) >= 2 {
		result := make([]string, 0, len(fragments)+1)
		result = append(result, fragments[0])
		result = append(result, name)
		result = append(result, fragments[1:]...)
		return result
	}
	return append(fragments, name)
}
