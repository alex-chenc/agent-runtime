package contextbudget

import (
	"testing"

	"github.com/alex-chenc/agent-runtime/core"
)

func TestDefaultEstimator_EmptyText(t *testing.T) {
	e := NewDefaultEstimator()
	got := e.EstimateText("")
	if got != 0 {
		t.Errorf("EstimateText(\"\") = %d, want 0", got)
	}
}

func TestDefaultEstimator_ShortText(t *testing.T) {
	e := NewDefaultEstimator()
	// 4 chars / 4.0 charsPerToken = 1, +1 = 2
	got := e.EstimateText("test")
	if got != 2 {
		t.Errorf("EstimateText(\"test\") = %d, want 2", got)
	}
}

func TestDefaultEstimator_LongText(t *testing.T) {
	e := NewDefaultEstimator()
	text := "this is a longer test string with more characters"
	// 50 chars / 4.0 = 12.5, int = 12, +1 = 13
	got := e.EstimateText(text)
	if got != 13 {
		t.Errorf("EstimateText(%q) = %d, want 13", text, got)
	}
}

func TestDefaultEstimator_Messages(t *testing.T) {
	e := NewDefaultEstimator()
	messages := []core.LLMMessage{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Hello, how are you?"},
	}
	got := e.EstimateMessages(messages)
	// system: 4 + EstimateText("You are a helpful assistant.") = 4 + (28/4+1) = 4+8 = 12
	// user: 4 + EstimateText("Hello, how are you?") = 4 + (19/4+1) = 4+5 = 9
	// total: 21
	if got != 21 {
		t.Errorf("EstimateMessages = %d, want 21", got)
	}
}

func TestDefaultEstimator_EmptyMessages(t *testing.T) {
	e := NewDefaultEstimator()
	got := e.EstimateMessages([]core.LLMMessage{})
	if got != 0 {
		t.Errorf("EstimateMessages(empty) = %d, want 0", got)
	}
}

func TestDefaultEstimator_Calibrate(t *testing.T) {
	e := NewDefaultEstimator()
	// Initial: charsPerToken = 4.0
	// If observed=100, estimated=120 (overestimate by 1.2x)
	// ratio = 120/100 = 1.2, newCharsPerToken = 4.0 * 1.2 = 4.8
	e.Calibrate(100, 120)
	got := e.EstimateText("test") // 4 / 4.8 = 0.83, int=0, +1 = 1
	if got != 1 {
		t.Errorf("After calibration, EstimateText(\"test\") = %d, want 1", got)
	}
}

func TestDefaultEstimator_CalibrateClampLow(t *testing.T) {
	e := NewDefaultEstimator()
	// Try to push below 2.0
	e.Calibrate(1000, 100) // ratio = 0.1, newCharsPerToken = 4.0 * 0.1 = 0.4 -> clamped to 2.0
	got := e.EstimateText("test") // 4 / 2.0 = 2, +1 = 3
	if got != 3 {
		t.Errorf("After calibration clamp low, EstimateText(\"test\") = %d, want 3", got)
	}
}

func TestDefaultEstimator_CalibrateClampHigh(t *testing.T) {
	e := NewDefaultEstimator()
	// Try to push above 8.0
	e.Calibrate(100, 1000) // ratio = 10, newCharsPerToken = 4.0 * 10 = 40 -> clamped to 8.0
	got := e.EstimateText("test") // 4 / 8.0 = 0.5, int=0, +1 = 1
	if got != 1 {
		t.Errorf("After calibration clamp high, EstimateText(\"test\") = %d, want 1", got)
	}
}

func TestDefaultEstimator_CalibrateZeroTokens(t *testing.T) {
	e := NewDefaultEstimator()
	// Should not change charsPerToken
	e.Calibrate(0, 100)
	e.Calibrate(100, 0)
	got := e.EstimateText("test")
	if got != 2 {
		t.Errorf("After zero calibration, EstimateText(\"test\") = %d, want 2", got)
	}
}

func TestDefaultEstimator_ChineseText(t *testing.T) {
	e := NewDefaultEstimator()
	// Chinese characters are typically 1 char = 1 token
	// "你好世界" = 12 bytes, 12/4 = 3, +1 = 4
	text := "你好世界"
	got := e.EstimateText(text)
	if got != 4 {
		t.Errorf("EstimateText(%q) = %d, want 4", text, got)
	}
}
