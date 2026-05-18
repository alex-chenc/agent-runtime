package contextbudget

import (
	"github.com/chenchen511/agent-runtime/core"
)

// Compressor orchestrates progressive context compression.
// It cascades through compression levels: tool results (70%) -> step folding (80%) -> LLM summarization (95%).
type Compressor struct {
	config    core.RuntimeConfig
	estimator *DefaultEstimator
	budget    *BudgetCalculator
	policy    *CompressionPolicy
	tool      *ToolCompressor
	step      *StepFolder
	llm       *LLMSummarizer
}

// NewCompressor creates a new Compressor with all sub-components.
func NewCompressor(config core.RuntimeConfig, estimator *DefaultEstimator, llmClient core.LLMClient) *Compressor {
	budget := NewBudgetCalculator(config, estimator)
	policy := NewCompressionPolicy(config)
	tool := NewToolCompressor(estimator)
	step := NewStepFolder()
	llm := NewLLMSummarizer(llmClient, config.RecentTurnsToKeep)

	return &Compressor{
		config:    config,
		estimator: estimator,
		budget:    budget,
		policy:    policy,
		tool:      tool,
		step:      step,
		llm:       llm,
	}
}

// Compress performs progressive compression on the message list.
// Returns the compressed messages, compression records, and any error.
// The compression cascades: if after one level the ratio is still too high,
// the next level is applied.
func (c *Compressor) Compress(messages []core.LLMMessage, steps []core.StepExecution, currentStepID string) ([]core.LLMMessage, []core.CompressionRecord, error) {
	if !c.config.EnableContextCompress {
		return messages, nil, nil
	}

	var records []core.CompressionRecord
	current := messages

	// Cascade through compression levels (max 3 iterations)
	for i := 0; i < 3; i++ {
		snap := c.budget.EstimateContext(current)
		action := c.policy.Assess(snap)

		if action.Level == 0 {
			// Context is sufficient
			break
		}

		switch action.Level {
		case 1:
			// Recording only, no compression
			break
		case 2:
			// Tool result compression
			compressed := c.tool.CompressToolResults(current)
			if c.estimateTotal(compressed) < c.estimateTotal(current) {
				record := core.CompressionRecord{
					Strategy:     core.StrategyToolResults,
					TriggerRatio: snap.ContextRatio,
					BeforeTokens: c.estimateTotal(current),
					AfterTokens:  c.estimateTotal(compressed),
				}
				records = append(records, record)
				current = compressed
			}
		case 3:
			// Step folding
			compressed := c.step.FoldSteps(current, steps, currentStepID)
			if len(compressed) < len(current) {
				record := core.CompressionRecord{
					Strategy:     core.StrategyHistoricalSteps,
					TriggerRatio: snap.ContextRatio,
					BeforeTokens: c.estimateTotal(current),
					AfterTokens:  c.estimateTotal(compressed),
				}
				records = append(records, record)
				current = compressed
			}
		case 4:
			// LLM summarization
			compressed, err := c.llm.Summarize(current)
			if err != nil {
				// Emergency fallback is handled inside Summarize
				return current, records, nil
			}
			record := core.CompressionRecord{
				Strategy:     core.StrategyLLMPriorTurns,
				TriggerRatio: snap.ContextRatio,
				BeforeTokens: c.estimateTotal(current),
				AfterTokens:  c.estimateTotal(compressed),
			}
			records = append(records, record)
			current = compressed
		case 5:
			// Emergency: try LLM summarization as last resort
			compressed, err := c.llm.Summarize(current)
			if err != nil {
				return current, records, nil
			}
			record := core.CompressionRecord{
				Strategy:     core.StrategyEmergency,
				TriggerRatio: snap.ContextRatio,
				BeforeTokens: c.estimateTotal(current),
				AfterTokens:  c.estimateTotal(compressed),
			}
			records = append(records, record)
			current = compressed
		}
	}

	return current, records, nil
}

// GetBudgetSnapshot returns the current budget snapshot for the given messages.
func (c *Compressor) GetBudgetSnapshot(messages []core.LLMMessage) core.ContextBudgetSnapshot {
	return c.budget.EstimateContext(messages)
}

// estimateTotal estimates total tokens across all messages.
func (c *Compressor) estimateTotal(messages []core.LLMMessage) int {
	return c.estimator.EstimateMessages(messages)
}
