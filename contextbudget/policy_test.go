package contextbudget

import (
	"testing"

	"github.com/alex-chenc/agent-runtime/core"
)

func TestCompressionPolicy_ContextSufficient(t *testing.T) {
	config := core.DefaultConfig()
	p := NewCompressionPolicy(config)

	snap := core.ContextBudgetSnapshot{ContextRatio: 0.50}
	action := p.Assess(snap)

	if action.Level != 0 {
		t.Errorf("Level = %d, want 0 (context sufficient)", action.Level)
	}
	if action.Message != "context sufficient" {
		t.Errorf("Message = %q, want \"context sufficient\"", action.Message)
	}
}

func TestCompressionPolicy_RecordingBudget(t *testing.T) {
	config := core.DefaultConfig()
	p := NewCompressionPolicy(config)

	snap := core.ContextBudgetSnapshot{ContextRatio: 0.65}
	action := p.Assess(snap)

	if action.Level != 1 {
		t.Errorf("Level = %d, want 1 (recording budget)", action.Level)
	}
}

func TestCompressionPolicy_ToolCompression(t *testing.T) {
	config := core.DefaultConfig()
	p := NewCompressionPolicy(config)

	snap := core.ContextBudgetSnapshot{ContextRatio: 0.75}
	action := p.Assess(snap)

	if action.Level != 2 {
		t.Errorf("Level = %d, want 2 (tool compression)", action.Level)
	}
	if action.Strategy != core.StrategyToolResults {
		t.Errorf("Strategy = %q, want %q", action.Strategy, core.StrategyToolResults)
	}
}

func TestCompressionPolicy_StepFolding(t *testing.T) {
	config := core.DefaultConfig()
	p := NewCompressionPolicy(config)

	snap := core.ContextBudgetSnapshot{ContextRatio: 0.85}
	action := p.Assess(snap)

	if action.Level != 3 {
		t.Errorf("Level = %d, want 3 (step folding)", action.Level)
	}
	if action.Strategy != core.StrategyHistoricalSteps {
		t.Errorf("Strategy = %q, want %q", action.Strategy, core.StrategyHistoricalSteps)
	}
}

func TestCompressionPolicy_LLMSummarize(t *testing.T) {
	config := core.DefaultConfig()
	p := NewCompressionPolicy(config)

	snap := core.ContextBudgetSnapshot{ContextRatio: 0.97}
	action := p.Assess(snap)

	if action.Level != 4 {
		t.Errorf("Level = %d, want 4 (LLM summarize)", action.Level)
	}
	if action.Strategy != core.StrategyLLMPriorTurns {
		t.Errorf("Strategy = %q, want %q", action.Strategy, core.StrategyLLMPriorTurns)
	}
}

func TestCompressionPolicy_Emergency(t *testing.T) {
	config := core.DefaultConfig()
	p := NewCompressionPolicy(config)

	snap := core.ContextBudgetSnapshot{ContextRatio: 1.05}
	action := p.Assess(snap)

	if action.Level != 5 {
		t.Errorf("Level = %d, want 5 (emergency)", action.Level)
	}
	if action.Strategy != core.StrategyEmergency {
		t.Errorf("Strategy = %q, want %q", action.Strategy, core.StrategyEmergency)
	}
}

func TestCompressionPolicy_ExactBoundary_ToolToStep(t *testing.T) {
	config := core.DefaultConfig()
	p := NewCompressionPolicy(config)

	// Exactly at step threshold (0.80)
	snap := core.ContextBudgetSnapshot{ContextRatio: 0.80}
	action := p.Assess(snap)

	if action.Level != 3 {
		t.Errorf("At ratio 0.80, Level = %d, want 3 (step folding)", action.Level)
	}
}

func TestCompressionPolicy_ExactBoundary_StepToLLM(t *testing.T) {
	config := core.DefaultConfig()
	p := NewCompressionPolicy(config)

	// Exactly at LLM threshold (0.95)
	snap := core.ContextBudgetSnapshot{ContextRatio: 0.95}
	action := p.Assess(snap)

	if action.Level != 4 {
		t.Errorf("At ratio 0.95, Level = %d, want 4 (LLM summarize)", action.Level)
	}
}

func TestCompressionPolicy_JustBelowTool(t *testing.T) {
	config := core.DefaultConfig()
	p := NewCompressionPolicy(config)

	snap := core.ContextBudgetSnapshot{ContextRatio: 0.69}
	action := p.Assess(snap)

	if action.Level != 1 {
		t.Errorf("At ratio 0.69, Level = %d, want 1 (recording)", action.Level)
	}
}

func TestCompressionPolicy_JustAboveTool(t *testing.T) {
	config := core.DefaultConfig()
	p := NewCompressionPolicy(config)

	snap := core.ContextBudgetSnapshot{ContextRatio: 0.71}
	action := p.Assess(snap)

	if action.Level != 2 {
		t.Errorf("At ratio 0.71, Level = %d, want 2 (tool compression)", action.Level)
	}
}

func TestCompressionPolicy_CustomThresholds(t *testing.T) {
	config := core.RuntimeConfig{
		ToolCompressRatio: 0.50,
		StepCompressRatio: 0.60,
		LLMCompressRatio:  0.70,
	}
	p := NewCompressionPolicy(config)

	tests := []struct {
		ratio     float64
		wantLevel int
	}{
		{0.45, 0}, // context sufficient (below hardcoded 0.60 recording threshold)
		{0.55, 2}, // tool (above custom 0.50)
		{0.65, 3}, // step (above custom 0.60)
		{0.75, 4}, // LLM (above custom 0.70)
		{1.05, 5}, // emergency
	}

	for _, tt := range tests {
		snap := core.ContextBudgetSnapshot{ContextRatio: tt.ratio}
		action := p.Assess(snap)
		if action.Level != tt.wantLevel {
			t.Errorf("ratio=%.2f: Level=%d, want %d", tt.ratio, action.Level, tt.wantLevel)
		}
	}
}

func TestCompressionPolicy_ZeroThresholdsUseDefaults(t *testing.T) {
	config := core.RuntimeConfig{} // all zeros
	p := NewCompressionPolicy(config)

	// Should use defaults: tool=0.70, step=0.80, llm=0.95
	snap := core.ContextBudgetSnapshot{ContextRatio: 0.75}
	action := p.Assess(snap)

	if action.Level != 2 {
		t.Errorf("With zero config at ratio 0.75, Level = %d, want 2", action.Level)
	}
}

func TestCompressionAction_TriggerRatio(t *testing.T) {
	config := core.DefaultConfig()
	p := NewCompressionPolicy(config)

	snap := core.ContextBudgetSnapshot{ContextRatio: 0.85}
	action := p.Assess(snap)

	if action.TriggerRatio != 0.85 {
		t.Errorf("TriggerRatio = %f, want 0.85", action.TriggerRatio)
	}
}
