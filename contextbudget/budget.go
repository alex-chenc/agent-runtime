package contextbudget

import (
	"github.com/chenchen511/agent-runtime/core"
)

// BudgetCalculator computes the context budget snapshot for a set of messages.
type BudgetCalculator struct {
	config    core.RuntimeConfig
	estimator TokenEstimator
}

// NewBudgetCalculator creates a new BudgetCalculator.
func NewBudgetCalculator(config core.RuntimeConfig, estimator TokenEstimator) *BudgetCalculator {
	return &BudgetCalculator{
		config:    config,
		estimator: estimator,
	}
}

// EstimateContext computes the context budget snapshot for the given messages.
func (bc *BudgetCalculator) EstimateContext(messages []core.LLMMessage) core.ContextBudgetSnapshot {
	estimatedPrompt := bc.estimator.EstimateMessages(messages)
	reserved := bc.config.ReservedOutputTokens
	maxCtx := bc.config.MaxContextTokens

	if maxCtx <= 0 {
		maxCtx = 256000
	}
	if reserved <= 0 {
		reserved = 8192
	}

	ratio := float64(estimatedPrompt+reserved) / float64(maxCtx)

	return core.ContextBudgetSnapshot{
		MaxContextTokens:      maxCtx,
		ReservedOutputTokens:  reserved,
		EstimatedPromptTokens: estimatedPrompt,
		ContextRatio:          ratio,
	}
}

// EstimateText returns the estimated tokens for a text string.
func (bc *BudgetCalculator) EstimateText(text string) int {
	return bc.estimator.EstimateText(text)
}
