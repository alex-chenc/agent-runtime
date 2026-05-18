package contextbudget

import (
	"github.com/chenchen511/agent-runtime/core"
)

// TokenEstimator estimates the number of tokens in messages or text.
type TokenEstimator interface {
	EstimateMessages(messages []core.LLMMessage) int
	EstimateText(text string) int
}

// DefaultEstimator uses a simple heuristic: ~4 characters per token.
// This is a conservative overestimate for English text and a reasonable
// approximation for mixed-language content. Accuracy improves when
// calibrated with real usage.prompt_tokens from LLM responses.
type DefaultEstimator struct {
	// charsPerToken is the average number of characters per token.
	// Default is 4.0 (conservative overestimate).
	charsPerToken float64
}

// NewDefaultEstimator creates a new DefaultEstimator with ~4 chars/token heuristic.
func NewDefaultEstimator() *DefaultEstimator {
	return &DefaultEstimator{charsPerToken: 4.0}
}

// EstimateMessages estimates the total tokens across all messages.
func (e *DefaultEstimator) EstimateMessages(messages []core.LLMMessage) int {
	total := 0
	for _, m := range messages {
		// Each message has overhead for role, formatting (~4 tokens)
		total += 4 + e.EstimateText(m.Content)
	}
	return total
}

// EstimateText estimates the number of tokens in a text string.
func (e *DefaultEstimator) EstimateText(text string) int {
	if len(text) == 0 {
		return 0
	}
	// Conservative: round up
	return int(float64(len(text))/e.charsPerToken) + 1
}

// Calibrate adjusts the estimator based on observed real usage.
// observedTokens is the actual prompt_tokens from the LLM response.
// estimatedTokens is what the estimator predicted.
func (e *DefaultEstimator) Calibrate(observedTokens, estimatedTokens int) {
	if estimatedTokens <= 0 || observedTokens <= 0 {
		return
	}
	// Adjust charsPerToken to better match reality
	ratio := float64(estimatedTokens) / float64(observedTokens)
	newCharsPerToken := e.charsPerToken * ratio
	// Clamp to reasonable range [2.0, 8.0]
	if newCharsPerToken < 2.0 {
		newCharsPerToken = 2.0
	}
	if newCharsPerToken > 8.0 {
		newCharsPerToken = 8.0
	}
	e.charsPerToken = newCharsPerToken
}
