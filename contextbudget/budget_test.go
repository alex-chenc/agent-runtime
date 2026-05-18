package contextbudget

import (
	"testing"

	"github.com/chenchen511/agent-runtime/core"
)

func TestBudgetCalculator_DefaultConfig(t *testing.T) {
	config := core.DefaultConfig()
	estimator := NewDefaultEstimator()
	bc := NewBudgetCalculator(config, estimator)

	messages := []core.LLMMessage{
		{Role: "system", Content: "You are a security analyst."},
		{Role: "user", Content: "Analyze this host for vulnerabilities."},
	}

	snap := bc.EstimateContext(messages)

	if snap.MaxContextTokens != 256000 {
		t.Errorf("MaxContextTokens = %d, want 256000", snap.MaxContextTokens)
	}
	if snap.ReservedOutputTokens != 8192 {
		t.Errorf("ReservedOutputTokens = %d, want 8192", snap.ReservedOutputTokens)
	}
	if snap.EstimatedPromptTokens <= 0 {
		t.Errorf("EstimatedPromptTokens = %d, want > 0", snap.EstimatedPromptTokens)
	}
	if snap.ContextRatio <= 0 || snap.ContextRatio > 1.0 {
		t.Errorf("ContextRatio = %f, want (0, 1.0]", snap.ContextRatio)
	}
}

func TestBudgetCalculator_CustomConfig(t *testing.T) {
	config := core.RuntimeConfig{
		MaxContextTokens:     128000,
		ReservedOutputTokens: 4096,
	}
	estimator := NewDefaultEstimator()
	bc := NewBudgetCalculator(config, estimator)

	messages := []core.LLMMessage{
		{Role: "user", Content: "Hello"},
	}

	snap := bc.EstimateContext(messages)

	if snap.MaxContextTokens != 128000 {
		t.Errorf("MaxContextTokens = %d, want 128000", snap.MaxContextTokens)
	}
	if snap.ReservedOutputTokens != 4096 {
		t.Errorf("ReservedOutputTokens = %d, want 4096", snap.ReservedOutputTokens)
	}
}

func TestBudgetCalculator_ZeroConfigDefaults(t *testing.T) {
	config := core.RuntimeConfig{} // all zeros
	estimator := NewDefaultEstimator()
	bc := NewBudgetCalculator(config, estimator)

	messages := []core.LLMMessage{
		{Role: "user", Content: "Hello"},
	}

	snap := bc.EstimateContext(messages)

	// Should use defaults: 256000 and 8192
	if snap.MaxContextTokens != 256000 {
		t.Errorf("MaxContextTokens = %d, want 256000 (default)", snap.MaxContextTokens)
	}
	if snap.ReservedOutputTokens != 8192 {
		t.Errorf("ReservedOutputTokens = %d, want 8192 (default)", snap.ReservedOutputTokens)
	}
}

func TestBudgetCalculator_RatioCalculation(t *testing.T) {
	config := core.RuntimeConfig{
		MaxContextTokens:     1000,
		ReservedOutputTokens: 100,
	}
	estimator := NewDefaultEstimator()
	bc := NewBudgetCalculator(config, estimator)

	// Create a message that will produce ~50 tokens
	// "test" = 2 tokens, so we need a longer message
	// 200 chars / 4 = 50 tokens
	messages := []core.LLMMessage{
		{Role: "user", Content: "this is a test message with enough characters to produce approximately fifty tokens when estimated using the default heuristic of four characters per token"},
	}

	snap := bc.EstimateContext(messages)

	// ratio = (estimatedPrompt + reserved) / maxCtx
	// estimatedPrompt = ~45 tokens (176 chars / 4 + 1 = 45, + 4 overhead = 49)
	// ratio = (49 + 100) / 1000 = 0.149
	if snap.ContextRatio <= 0 {
		t.Errorf("ContextRatio = %f, want > 0", snap.ContextRatio)
	}
	if snap.ContextRatio >= 1.0 {
		t.Errorf("ContextRatio = %f, want < 1.0", snap.ContextRatio)
	}
}

func TestBudgetCalculator_EstimateText(t *testing.T) {
	config := core.DefaultConfig()
	estimator := NewDefaultEstimator()
	bc := NewBudgetCalculator(config, estimator)

	got := bc.EstimateText("test")
	if got != 2 {
		t.Errorf("EstimateText(\"test\") = %d, want 2", got)
	}
}

func TestBudgetCalculator_HighRatio(t *testing.T) {
	config := core.RuntimeConfig{
		MaxContextTokens:     100,
		ReservedOutputTokens: 50,
	}
	estimator := NewDefaultEstimator()
	bc := NewBudgetCalculator(config, estimator)

	// Create messages that will push ratio high
	// Need estimatedPrompt > 50 to get ratio > 1.0 with MaxContextTokens=100, Reserved=50
	// 240 chars / 4 = 60 tokens, + 4 overhead = 64; ratio = (64+50)/100 = 1.14
	messages := []core.LLMMessage{
		{Role: "user", Content: "this is a very long message that should produce enough tokens to exceed the context window limit when combined with the reserved output tokens for the test case and we need even more characters to push it over the edge"},
	}

	snap := bc.EstimateContext(messages)

	// ratio = (104 + 50) / 100 = 1.54
	if snap.ContextRatio <= 1.0 {
		t.Errorf("ContextRatio = %f, want > 1.0 for this test case", snap.ContextRatio)
	}
}
