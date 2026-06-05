package router

import (
	"sort"
	"strings"

	"github.com/alex-chenc/agent-runtime/core"
)

// compose 按优先级拼接选中的片段
func (r *Router) compose(selectedNames []string) string {
	if len(selectedNames) == 0 {
		return ""
	}

	// 按名称查找片段
	var selected []core.PromptFragment
	nameSet := make(map[string]bool)
	for _, name := range selectedNames {
		nameSet[name] = true
	}

	for _, f := range r.fragments {
		if nameSet[f.Name] {
			selected = append(selected, f)
		}
	}

	// 按优先级排序（数字越大越靠前）
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].Priority > selected[j].Priority
	})

	// 拼接
	var buf strings.Builder
	for i, f := range selected {
		if i > 0 {
			buf.WriteString("\n\n")
		}
		buf.WriteString(f.Content)
	}
	return buf.String()
}
