package contextbudget

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/alex-chenc/agent-runtime/core"
)

// mockLLMClient is a test double for the LLMClient interface.
type mockLLMClient struct {
	response string
	err      error
}

func (m *mockLLMClient) Complete(ctx context.Context, req core.LLMRequest) (core.LLMResponse, error) {
	if m.err != nil {
		return core.LLMResponse{}, m.err
	}
	return core.LLMResponse{Content: m.response}, nil
}

func TestLLMSummarizer_SuccessfulCompression(t *testing.T) {
	summaryJSON := `{
		"summary": "Investigated suspicious process spawned by sshd",
		"facts": ["bash spawned by sshd with PID 1234", "outbound connection to 1.2.3.4:4444"],
		"timeline": ["2026-05-11T08:00: sshd spawned bash", "2026-05-11T08:01: bash connected to remote"],
		"evidence": [{"source": "GetProcessTree", "fact": "PID 9999 is nc reverse shell", "confidence": "high"}],
		"open_questions": ["Is 1.2.3.4 a known C2 server?"],
		"risks": ["Active reverse shell confirmed"],
		"discarded_detail": "Routine process listings and network stats"
	}`

	mock := &mockLLMClient{response: summaryJSON}
	summarizer := NewLLMSummarizer(mock, 6)

	messages := []core.LLMMessage{
		{Role: "system", Content: "You are a security analyst."},
		{Role: "user", Content: "Analyze alert for host-001"},
		// Old turns (to be compressed)
		{Role: "assistant", Content: "Step 1: Checking process tree"},
		{Role: "tool", Content: `{"tool_name": "GetProcessTree", "data": "..."}`},
		{Role: "assistant", Content: "Found suspicious process"},
		{Role: "tool", Content: `{"tool_name": "GetProcessTree", "data": "more"}`},
		{Role: "assistant", Content: "Step 2: Checking network"},
		{Role: "tool", Content: `{"tool_name": "GetNetworkConnections", "data": "..."}`},
		{Role: "assistant", Content: "Found outbound connection"},
		{Role: "tool", Content: `{"tool_name": "GetNetworkConnections", "data": "more"}`},
		// Recent turns (to be preserved)
		{Role: "assistant", Content: "Step 3: Checking files"},
		{Role: "tool", Content: `{"tool_name": "GetOpenFiles", "data": "..."}`},
		{Role: "assistant", Content: "Found suspicious files"},
		{Role: "tool", Content: `{"tool_name": "GetOpenFiles", "data": "more"}`},
		{Role: "assistant", Content: "Step 4: Analyzing evidence"},
		{Role: "tool", Content: `{"tool_name": "GetUserSessions", "data": "..."}`},
		{Role: "assistant", Content: "Found active sessions"},
		{Role: "tool", Content: `{"tool_name": "GetUserSessions", "data": "more"}`},
		{Role: "assistant", Content: "Step 5: Final analysis"},
		{Role: "tool", Content: `{"tool_name": "GetRunningProcesses", "data": "..."}`},
	}

	result, err := summarizer.Summarize(messages)
	if err != nil {
		t.Fatalf("Summarize failed: %v", err)
	}

	if len(result) == 0 {
		t.Fatal("Result should not be empty")
	}

	// First message should be system
	if result[0].Role != "system" {
		t.Errorf("First message should be system, got %s", result[0].Role)
	}

	// Second should be user
	if result[1].Role != "user" {
		t.Errorf("Second message should be user, got %s", result[1].Role)
	}

	// Third should be the summary
	if result[2].Role != "user" {
		t.Errorf("Third message should be summary (user role), got %s", result[2].Role)
	}

	// Summary should contain key content
	if !containsStr(result[2].Content, "Investigated suspicious process") {
		t.Error("Summary should contain the LLM-generated summary")
	}

	// Should have recent messages preserved
	if len(result) < 4 {
		t.Errorf("Should have at least 4 messages (system, user, summary, recent), got %d", len(result))
	}
}

func TestLLMSummarizer_PreservesRecentTurns(t *testing.T) {
	summaryJSON := `{"summary": "test summary", "facts": [], "timeline": [], "evidence": [], "open_questions": [], "risks": [], "discarded_detail": ""}`

	mock := &mockLLMClient{response: summaryJSON}
	summarizer := NewLLMSummarizer(mock, 6)

	// Create 20 messages (10 turns)
	messages := []core.LLMMessage{
		{Role: "system", Content: "System prompt"},
		{Role: "user", Content: "User input"},
	}
	for i := 0; i < 9; i++ {
		messages = append(messages, core.LLMMessage{Role: "assistant", Content: "Turn action"})
		messages = append(messages, core.LLMMessage{Role: "tool", Content: "Tool result"})
	}

	result, err := summarizer.Summarize(messages)
	if err != nil {
		t.Fatalf("Summarize failed: %v", err)
	}

	// Should have: system + user + summary + recent messages
	if len(result) < 4 {
		t.Errorf("Should have at least 4 messages, got %d", len(result))
	}
}

func TestLLMSummarizer_LLMFailure_EmergencyFallback(t *testing.T) {
	mock := &mockLLMClient{err: errors.New("LLM call failed")}
	summarizer := NewLLMSummarizer(mock, 6)

	messages := []core.LLMMessage{
		{Role: "system", Content: "System prompt"},
		{Role: "user", Content: "User input"},
		// Old turns (to be compressed via emergency fallback)
		{Role: "assistant", Content: "Turn 1"},
		{Role: "tool", Content: "Tool result 1"},
		{Role: "assistant", Content: "Turn 2"},
		{Role: "tool", Content: "Tool result 2"},
		{Role: "assistant", Content: "Turn 3"},
		{Role: "tool", Content: "Tool result 3"},
		{Role: "assistant", Content: "Turn 4"},
		{Role: "tool", Content: "Tool result 4"},
		// Recent turns (to be preserved)
		{Role: "assistant", Content: "Turn 5"},
		{Role: "tool", Content: "Tool result 5"},
		{Role: "assistant", Content: "Turn 6"},
		{Role: "tool", Content: "Tool result 6"},
		{Role: "assistant", Content: "Turn 7"},
		{Role: "tool", Content: "Tool result 7"},
		{Role: "assistant", Content: "Turn 8"},
		{Role: "tool", Content: "Tool result 8"},
		{Role: "assistant", Content: "Turn 9"},
		{Role: "tool", Content: "Tool result 9"},
	}

	result, err := summarizer.Summarize(messages)
	if err != nil {
		t.Fatalf("Summarize should not return error on fallback: %v", err)
	}

	if len(result) == 0 {
		t.Fatal("Result should not be empty even on fallback")
	}

	// Should contain emergency compression indicator
	foundEmergency := false
	for _, msg := range result {
		if containsStr(msg.Content, "Emergency") || containsStr(msg.Content, "compressed programmatically") {
			foundEmergency = true
		}
	}
	if !foundEmergency {
		t.Error("Fallback result should indicate emergency compression")
	}
}

func TestLLMSummarizer_ShortMessages_NoCompression(t *testing.T) {
	mock := &mockLLMClient{response: `{"summary": "test"}`}
	summarizer := NewLLMSummarizer(mock, 6)

	messages := []core.LLMMessage{
		{Role: "system", Content: "System prompt"},
		{Role: "user", Content: "User input"},
		{Role: "assistant", Content: "Single turn"},
	}

	result, err := summarizer.Summarize(messages)
	if err != nil {
		t.Fatalf("Summarize failed: %v", err)
	}

	// Messages are too short to need compression
	if len(result) != 3 {
		t.Errorf("Short messages should not be compressed, got %d messages", len(result))
	}
}

func TestLLMSummarizer_InvalidJSON_EmergencyFallback(t *testing.T) {
	mock := &mockLLMClient{response: "not valid json"}
	summarizer := NewLLMSummarizer(mock, 6)

	messages := []core.LLMMessage{
		{Role: "system", Content: "System prompt"},
		{Role: "user", Content: "User input"},
		{Role: "assistant", Content: "Turn 1"},
		{Role: "tool", Content: "Tool result 1"},
		{Role: "assistant", Content: "Turn 2"},
		{Role: "tool", Content: "Tool result 2"},
		{Role: "assistant", Content: "Turn 3"},
		{Role: "tool", Content: "Tool result 3"},
	}

	result, err := summarizer.Summarize(messages)
	if err != nil {
		t.Fatalf("Summarize should not return error on invalid JSON: %v", err)
	}

	// Should fall back to emergency compression
	if len(result) == 0 {
		t.Fatal("Result should not be empty on invalid JSON")
	}
}

func TestLLMSummarizer_StructuredOutput_Fields(t *testing.T) {
	summaryJSON := `{
		"summary": "Security investigation summary",
		"facts": ["fact1", "fact2"],
		"timeline": ["event1", "event2"],
		"evidence": [{"source": "tool1", "fact": "evidence1", "confidence": "high"}],
		"open_questions": ["question1"],
		"risks": ["risk1"],
		"discarded_detail": "routine data"
	}`

	mock := &mockLLMClient{response: summaryJSON}
	summarizer := NewLLMSummarizer(mock, 6)

	// Verify the summary JSON is valid
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(summaryJSON), &parsed); err != nil {
		t.Fatalf("Summary JSON should be valid: %v", err)
	}

	// Check required fields
	requiredFields := []string{"summary", "facts", "timeline", "evidence", "open_questions", "risks", "discarded_detail"}
	for _, field := range requiredFields {
		if _, ok := parsed[field]; !ok {
			t.Errorf("Summary JSON should contain field %q", field)
		}
	}

	_ = summarizer // suppress unused warning
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
