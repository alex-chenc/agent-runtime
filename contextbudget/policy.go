package contextbudget

import (
	"github.com/chenchen511/agent-runtime/core"
)

// CompressionAction represents the action to take based on context ratio.
type CompressionAction struct {
	Strategy     core.CompressionStrategy
	TriggerRatio float64
	Message      string
	Level        int // 0=none, 1=record, 2=tool, 3=step, 4=llm, 5=emergency
}

var (
	ActionNone = CompressionAction{Level: 0, Message: "context sufficient"}
	ActionRecordOnly = CompressionAction{Level: 1, Strategy: "", Message: "recording budget"}
	ActionCompressToolResults = CompressionAction{
		Level: 2, Strategy: core.StrategyToolResults,
		Message: "compressing tool results",
	}
	ActionFoldHistoricalSteps = CompressionAction{
		Level: 3, Strategy: core.StrategyHistoricalSteps,
		Message: "folding historical steps",
	}
	ActionLLMSummarize = CompressionAction{
		Level: 4, Strategy: core.StrategyLLMPriorTurns,
		Message: "LLM summarizing older turns",
	}
	ActionEmergencyOrReject = CompressionAction{
		Level: 5, Strategy: core.StrategyEmergency,
		Message: "emergency compression or reject",
	}
)

// CompressionPolicy determines what compression to apply based on context ratio.
type CompressionPolicy struct {
	config core.RuntimeConfig
}

// NewCompressionPolicy creates a new CompressionPolicy.
func NewCompressionPolicy(config core.RuntimeConfig) *CompressionPolicy {
	return &CompressionPolicy{config: config}
}

// Assess determines the compression action needed for the given context snapshot.
func (p *CompressionPolicy) Assess(snapshot core.ContextBudgetSnapshot) CompressionAction {
	ratio := snapshot.ContextRatio

	if ratio > 1.0 {
		return CompressionAction{
			Level: 5, Strategy: core.StrategyEmergency,
			TriggerRatio: ratio, Message: "emergency compression or reject",
		}
	}

	llmRatio := p.config.LLMCompressRatio
	if llmRatio <= 0 {
		llmRatio = 0.95
	}
	stepRatio := p.config.StepCompressRatio
	if stepRatio <= 0 {
		stepRatio = 0.80
	}
	toolRatio := p.config.ToolCompressRatio
	if toolRatio <= 0 {
		toolRatio = 0.70
	}

	if ratio >= llmRatio {
		return CompressionAction{
			Level: 4, Strategy: core.StrategyLLMPriorTurns,
			TriggerRatio: ratio, Message: "LLM summarizing older turns",
		}
	}
	if ratio >= stepRatio {
		return CompressionAction{
			Level: 3, Strategy: core.StrategyHistoricalSteps,
			TriggerRatio: ratio, Message: "folding historical steps",
		}
	}
	if ratio >= toolRatio {
		return CompressionAction{
			Level: 2, Strategy: core.StrategyToolResults,
			TriggerRatio: ratio, Message: "compressing tool results",
		}
	}
	if ratio >= 0.60 {
		return CompressionAction{
			Level: 1, Strategy: "",
			TriggerRatio: ratio, Message: "recording budget",
		}
	}

	return CompressionAction{Level: 0, TriggerRatio: ratio, Message: "context sufficient"}
}
