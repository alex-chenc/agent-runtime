package contextbudget

import (
	"strings"
	"testing"

	"github.com/chenchen511/agent-runtime/core"
)

func TestCompressor_NoCompressionNeeded(t *testing.T) {
	config := core.DefaultConfig()
	mock := &mockLLMClient{response: `{"summary": "test"}`}
	estimator := NewDefaultEstimator()
	compressor := NewCompressor(config, estimator, mock)

	messages := []core.LLMMessage{
		{Role: "system", Content: "You are a security analyst."},
		{Role: "user", Content: "Analyze this alert."},
	}

	steps := []core.StepExecution{}
	currentStepID := ""

	result, records, err := compressor.Compress(messages, steps, currentStepID)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	if len(records) != 0 {
		t.Errorf("Expected no compression records, got %d", len(records))
	}

	// Messages should be unchanged
	if len(result) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(result))
	}
}

func TestCompressor_ToolCompression(t *testing.T) {
	// Config tuned so that longContent pushes ratio into 70-80% range
	// longContent = 10000 chars ~ 2500 tokens, other msgs ~ 28 tokens
	// Total ~ 2528 tokens. For ratio 0.75: maxCtx = (2528+100)/0.75 ≈ 3504
	config := core.RuntimeConfig{
		MaxContextTokens:      3500,
		ReservedOutputTokens:  100,
		EnableContextCompress: true,
		ToolCompressRatio:     0.70,
		StepCompressRatio:     0.80,
		LLMCompressRatio:      0.95,
		CompressTargetRatio:   0.60,
		RecentTurnsToKeep:     6,
	}
	mock := &mockLLMClient{response: `{"summary": "test"}`}
	estimator := NewDefaultEstimator()
	compressor := NewCompressor(config, estimator, mock)

	longContent := strings.Repeat("x", 10000) // ~2500 tokens

	messages := []core.LLMMessage{
		{Role: "system", Content: "System prompt."},
		{Role: "user", Content: "User input."},
		{Role: "assistant", Content: "Analyzing..."},
		{Role: "tool", Content: longContent},
	}

	steps := []core.StepExecution{}
	currentStepID := ""

	result, records, err := compressor.Compress(messages, steps, currentStepID)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	// Should have compressed
	if len(records) == 0 {
		t.Error("Expected compression records for tool compression")
	}

	// Result should be shorter
	totalLen := 0
	for _, msg := range result {
		totalLen += len(msg.Content)
	}
	originalLen := 0
	for _, msg := range messages {
		originalLen += len(msg.Content)
	}
	if totalLen >= originalLen {
		t.Error("Compressed result should be shorter than original")
	}
}

func TestCompressor_StepFolding(t *testing.T) {
	// Two long contents ~1500 tokens each + ~28 overhead = ~3028 tokens
	// For ratio 0.85: maxCtx = (3028+100)/0.85 ≈ 3680
	config := core.RuntimeConfig{
		MaxContextTokens:      3680,
		ReservedOutputTokens:  100,
		EnableContextCompress: true,
		ToolCompressRatio:     0.70,
		StepCompressRatio:     0.80,
		LLMCompressRatio:      0.95,
		CompressTargetRatio:   0.60,
		RecentTurnsToKeep:     6,
	}
	mock := &mockLLMClient{response: `{"summary": "test"}`}
	estimator := NewDefaultEstimator()
	compressor := NewCompressor(config, estimator, mock)

	longContent := strings.Repeat("x", 6000) // ~1500 tokens

	messages := []core.LLMMessage{
		{Role: "system", Content: "System prompt."},
		{Role: "user", Content: "User input."},
		{Role: "assistant", Content: "Step 1 analysis."},
		{Role: "tool", Content: longContent},
		{Role: "assistant", Content: "Step 2 analysis."},
		{Role: "tool", Content: longContent},
	}

	steps := []core.StepExecution{
		{StepID: "step_1", Status: core.StepCompleted, Result: "Result 1"},
		{StepID: "step_2", Status: core.StepRunning},
	}
	currentStepID := "step_2"

	result, records, err := compressor.Compress(messages, steps, currentStepID)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	// Should have compression records
	if len(records) == 0 {
		t.Error("Expected compression records")
	}

	// Should have folded step summary
	foundFolded := false
	for _, msg := range result {
		if strings.Contains(msg.Content, "step_1") {
			foundFolded = true
		}
	}
	if !foundFolded {
		t.Error("Should have folded step_1 summary")
	}
}

func TestCompressor_DisabledCompression(t *testing.T) {
	config := core.RuntimeConfig{
		MaxContextTokens:      1000,
		ReservedOutputTokens:  100,
		EnableContextCompress: false,
	}
	mock := &mockLLMClient{response: `{"summary": "test"}`}
	estimator := NewDefaultEstimator()
	compressor := NewCompressor(config, estimator, mock)

	longContent := strings.Repeat("x", 10000)

	messages := []core.LLMMessage{
		{Role: "system", Content: "System prompt."},
		{Role: "user", Content: "User input."},
		{Role: "assistant", Content: "Analyzing..."},
		{Role: "tool", Content: longContent},
	}

	result, records, err := compressor.Compress(messages, []core.StepExecution{}, "")
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	// Should not compress when disabled
	if len(records) != 0 {
		t.Error("Should not compress when disabled")
	}

	// Messages should be unchanged
	if len(result) != len(messages) {
		t.Errorf("Messages should be unchanged when compression disabled")
	}
}

func TestCompressor_CascadeProgression(t *testing.T) {
	// Three long contents ~1500 tokens each + ~28 overhead = ~4528 tokens
	// For ratio 0.90: maxCtx = (4528+50)/0.90 ≈ 5086
	config := core.RuntimeConfig{
		MaxContextTokens:      5086,
		ReservedOutputTokens:  50,
		EnableContextCompress: true,
		ToolCompressRatio:     0.50,
		StepCompressRatio:     0.60,
		LLMCompressRatio:      0.70,
		CompressTargetRatio:   0.40,
		RecentTurnsToKeep:     2,
	}
	mock := &mockLLMClient{response: `{"summary": "compressed", "facts": [], "timeline": [], "evidence": [], "open_questions": [], "risks": [], "discarded_detail": "test"}`}
	estimator := NewDefaultEstimator()
	compressor := NewCompressor(config, estimator, mock)

	longContent := strings.Repeat("x", 6000) // ~1500 tokens

	messages := []core.LLMMessage{
		{Role: "system", Content: "System prompt."},
		{Role: "user", Content: "User input."},
		{Role: "assistant", Content: "Step 1."},
		{Role: "tool", Content: longContent},
		{Role: "assistant", Content: "Step 2."},
		{Role: "tool", Content: longContent},
		{Role: "assistant", Content: "Step 3."},
		{Role: "tool", Content: longContent},
	}

	steps := []core.StepExecution{
		{StepID: "step_1", Status: core.StepCompleted, Result: "Result 1"},
		{StepID: "step_2", Status: core.StepCompleted, Result: "Result 2"},
		{StepID: "step_3", Status: core.StepRunning},
	}
	currentStepID := "step_3"

	_, records, err := compressor.Compress(messages, steps, currentStepID)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	// Should have at least 1 compression record from cascade
	if len(records) < 1 {
		t.Error("Expected at least 1 compression record from cascade")
	}
}

func TestCompressor_GetBudgetSnapshot(t *testing.T) {
	config := core.DefaultConfig()
	mock := &mockLLMClient{}
	estimator := NewDefaultEstimator()
	compressor := NewCompressor(config, estimator, mock)

	messages := []core.LLMMessage{
		{Role: "system", Content: "System prompt."},
		{Role: "user", Content: "User input."},
	}

	snap := compressor.GetBudgetSnapshot(messages)

	if snap.MaxContextTokens != 256000 {
		t.Errorf("MaxContextTokens = %d, want 256000", snap.MaxContextTokens)
	}
	if snap.ContextRatio <= 0 {
		t.Errorf("ContextRatio = %f, want > 0", snap.ContextRatio)
	}
}
