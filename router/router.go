package router

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alex-chenc/agent-runtime/core"
)

// Router 智能任务路由器
type Router struct {
	llmClient core.LLMClient
	fragments []core.PromptFragment
	config    Config
}

// Config 路由器配置
type Config struct {
	EnableLLMRouting  bool          `json:"enable_llm_routing"`   // 是否启用 LLM 路由
	LLMTemperature    float64       `json:"llm_temperature"`      // LLM 分类温度
	LLMTimeout        time.Duration `json:"llm_timeout"`          // LLM 分类超时
	DirectReplyMaxLen int           `json:"direct_reply_max_len"` // 直接回复最大消息长度
}

// DefaultConfig 默认配置
func DefaultConfig() Config {
	return Config{
		EnableLLMRouting:  true,
		LLMTemperature:    0.1,
		LLMTimeout:        30 * time.Second,
		DirectReplyMaxLen: 10,
	}
}

// RouteInput 路由输入
type RouteInput struct {
	TaskID      string
	UserMessage string
	Tools       []core.ToolDescriptor
	MaxSteps    int
}

// New 创建路由器
func New(llmClient core.LLMClient, fragments []core.PromptFragment, cfg Config) *Router {
	return &Router{
		llmClient: llmClient,
		fragments: fragments,
		config:    cfg,
	}
}

// Route 路由入口：返回动作、分类结果和拼接后的提示词
func (r *Router) Route(ctx context.Context, input RouteInput) (*core.RouteResult, error) {
	// 第一级：规则匹配（不调 LLM）
	if action, fragments, ok := r.matchRules(input.UserMessage); ok {
		composed := r.compose(fragments)
		return &core.RouteResult{
			Action:            action,
			SelectedFragments: fragments,
			ComposedPrompt:    composed,
		}, nil
	}

	// 第二级：LLM 语义分析 + 片段选择
	if !r.config.EnableLLMRouting {
		// LLM 路由禁用，返回 full_plan + 默认片段
		defaultFragments := r.defaultFragments()
		return &core.RouteResult{
			Action:            core.ActionFullPlan,
			SelectedFragments: defaultFragments,
			ComposedPrompt:    r.compose(defaultFragments),
		}, nil
	}

	classification, err := r.classify(ctx, input)
	if err != nil {
		// 分类失败，使用默认片段
		defaultFragments := r.defaultFragments()
		return &core.RouteResult{
			Action:            core.ActionFullPlan,
			SelectedFragments: defaultFragments,
			ComposedPrompt:    r.compose(defaultFragments),
		}, nil
	}

	// 第三级：片段拼接
	composed := r.compose(classification.Fragments)

	return &core.RouteResult{
		Action:            classification.Action,
		Classification:    classification,
		SelectedFragments: classification.Fragments,
		ComposedPrompt:    composed,
	}, nil
}

// defaultFragments 返回默认片段列表
func (r *Router) defaultFragments() []string {
	var names []string
	for _, f := range r.fragments {
		// 默认包含 base_assistant 和 react_format
		if f.Name == "base_assistant" || f.Name == "react_format" {
			names = append(names, f.Name)
		}
	}
	return names
}

// buildCatalogSummary 构建片段目录摘要（供 LLM 选择）
func (r *Router) buildCatalogSummary() string {
	var buf strings.Builder
	for _, f := range r.fragments {
		buf.WriteString(fmt.Sprintf("- %s: %s", f.Name, f.Description))
		if len(f.Keywords) > 0 {
			buf.WriteString(fmt.Sprintf(" (keywords: %s)", strings.Join(f.Keywords, ", ")))
		}
		buf.WriteString("\n")
	}
	return buf.String()
}
