package contextbudget

import (
	"strings"
	"testing"

	"github.com/chenchen511/agent-runtime/core"
)

func TestStepFolder_NoSteps(t *testing.T) {
	sf := NewStepFolder()

	messages := []core.LLMMessage{
		{Role: "system", Content: "You are a security analyst."},
		{Role: "user", Content: "Analyze this host."},
	}

	steps := []core.StepExecution{}
	currentStepID := ""

	result := sf.FoldSteps(messages, steps, currentStepID)

	if len(result) != 2 {
		t.Errorf("Expected 2 messages (no folding needed), got %d", len(result))
	}
	if result[0].Role != "system" {
		t.Errorf("First message should be system, got %s", result[0].Role)
	}
}

func TestStepFolder_SingleStep_NoFolding(t *testing.T) {
	sf := NewStepFolder()

	messages := []core.LLMMessage{
		{Role: "system", Content: "You are a security analyst."},
		{Role: "user", Content: "Analyze this host."},
		{Role: "assistant", Content: "I'll analyze the alert process."},
		{Role: "tool", Content: `{"tool_name": "GetProcessTree", "pid": 1234}`},
	}

	steps := []core.StepExecution{
		{
			StepID: "step_1",
			Status: core.StepCompleted,
			Result: "Confirmed suspicious process",
		},
	}
	currentStepID := "step_1"

	result := sf.FoldSteps(messages, steps, currentStepID)

	// Only 1 step, current step - no folding
	if len(result) != 4 {
		t.Errorf("Expected 4 messages (single step, no folding), got %d", len(result))
	}
}

func TestStepFolder_MultipleSteps_FoldsOlder(t *testing.T) {
	sf := NewStepFolder()

	messages := []core.LLMMessage{
		{Role: "system", Content: "You are a security analyst."},
		{Role: "user", Content: "Analyze this host."},
		{Role: "assistant", Content: "Step 1 analysis."},
		{Role: "tool", Content: `{"tool_name": "GetProcessTree", "result": "data"}`},
		{Role: "assistant", Content: "Step 2 analysis."},
		{Role: "tool", Content: `{"tool_name": "GetNetworkConnections", "result": "data"}`},
	}

	steps := []core.StepExecution{
		{
			StepID:   "step_1",
			Status:   core.StepCompleted,
			Result:   "Confirmed suspicious process bash spawned by sshd",
			Evidence: []string{"pid=1234", "ppid=889", "commandline=bash"},
		},
		{
			StepID:   "step_2",
			Status:   core.StepCompleted,
			Result:   "Found outbound connection to 1.2.3.4:4444",
			Evidence: []string{"remote=1.2.3.4:4444", "status=ESTABLISHED"},
		},
	}
	currentStepID := "step_2"

	result := sf.FoldSteps(messages, steps, currentStepID)

	// Should have: system, user, folded_summary, current_step_messages
	if len(result) < 3 {
		t.Errorf("Expected at least 3 messages, got %d", len(result))
	}

	// First should be system
	if result[0].Role != "system" {
		t.Errorf("First message should be system, got %s", result[0].Role)
	}

	// Second should be user
	if result[1].Role != "user" {
		t.Errorf("Second message should be user, got %s", result[1].Role)
	}

	// Third should be the folded summary
	if result[2].Role != "user" {
		t.Errorf("Third message should be folded summary (user role), got %s", result[2].Role)
	}
	if !strings.Contains(result[2].Content, "step_1") {
		t.Error("Folded summary should contain step_1")
	}
	if !strings.Contains(result[2].Content, "completed") {
		t.Error("Folded summary should contain step status")
	}
	if !strings.Contains(result[2].Content, "Confirmed suspicious process") {
		t.Error("Folded summary should contain step result")
	}
}

func TestStepFolder_PreservesCurrentStepMessages(t *testing.T) {
	sf := NewStepFolder()

	messages := []core.LLMMessage{
		{Role: "system", Content: "You are a security analyst."},
		{Role: "user", Content: "Analyze this host."},
		{Role: "assistant", Content: "Step 1 done."},
		{Role: "assistant", Content: "Step 2 current analysis."},
		{Role: "tool", Content: "Step 2 tool result."},
	}

	steps := []core.StepExecution{
		{
			StepID: "step_1",
			Status: core.StepCompleted,
			Result: "Old result",
		},
		{
			StepID: "step_2",
			Status: core.StepRunning,
		},
	}
	currentStepID := "step_2"

	result := sf.FoldSteps(messages, steps, currentStepID)

	// Should preserve current step's messages
	foundCurrentStepMsg := false
	for _, msg := range result {
		if strings.Contains(msg.Content, "Step 2 current analysis") {
			foundCurrentStepMsg = true
		}
	}
	if !foundCurrentStepMsg {
		t.Error("Current step messages should be preserved")
	}
}

func TestStepFolder_FoldedIncludesError(t *testing.T) {
	sf := NewStepFolder()

	messages := []core.LLMMessage{
		{Role: "system", Content: "System prompt."},
		{Role: "user", Content: "User input."},
		{Role: "assistant", Content: "Step 1 analysis."},
		{Role: "assistant", Content: "Step 2 analysis."},
	}

	steps := []core.StepExecution{
		{
			StepID: "step_1",
			Status: core.StepFailed,
			Error: &core.RuntimeError{
				Message: "connection timeout",
			},
		},
		{
			StepID: "step_2",
			Status: core.StepRunning,
		},
	}
	currentStepID := "step_2"

	result := sf.FoldSteps(messages, steps, currentStepID)

	// Find folded summary
	for _, msg := range result {
		if strings.Contains(msg.Content, "step_1") && strings.Contains(msg.Content, "failed") {
			if !strings.Contains(msg.Content, "connection timeout") {
				t.Error("Folded step should include error summary")
			}
			return
		}
	}
	t.Error("Should find folded step_1 with error")
}

func TestStepFolder_ToolCallSummary(t *testing.T) {
	sf := NewStepFolder()

	messages := []core.LLMMessage{
		{Role: "system", Content: "System prompt."},
		{Role: "user", Content: "User input."},
		{Role: "assistant", Content: "Step 1 analysis."},
		{Role: "tool", Content: `{"tool_name": "GetProcessTree"}`},
		{Role: "assistant", Content: "Step 2 analysis."},
	}

	steps := []core.StepExecution{
		{
			StepID: "step_1",
			Status: core.StepCompleted,
			Result: "Found suspicious process",
			ReactTurns: []core.ReactTurn{
				{Action: core.ReactAction{ToolName: "GetProcessTree"}},
				{Action: core.ReactAction{ToolName: "GetNetworkConnections"}},
			},
		},
		{
			StepID: "step_2",
			Status: core.StepRunning,
		},
	}
	currentStepID := "step_2"

	result := sf.FoldSteps(messages, steps, currentStepID)

	// Find folded summary
	for _, msg := range result {
		if strings.Contains(msg.Content, "step_1") {
			if !strings.Contains(msg.Content, "GetProcessTree") {
				t.Error("Folded step should include tool call summary")
			}
			return
		}
	}
	t.Error("Should find folded step_1")
}

func TestStepFolder_MultiplePreviousSteps(t *testing.T) {
	sf := NewStepFolder()

	messages := []core.LLMMessage{
		{Role: "system", Content: "System prompt."},
		{Role: "user", Content: "User input."},
		{Role: "assistant", Content: "Step 1."},
		{Role: "assistant", Content: "Step 2."},
		{Role: "assistant", Content: "Step 3."},
	}

	steps := []core.StepExecution{
		{StepID: "step_1", Status: core.StepCompleted, Result: "Result 1"},
		{StepID: "step_2", Status: core.StepCompleted, Result: "Result 2"},
		{StepID: "step_3", Status: core.StepRunning},
	}
	currentStepID := "step_3"

	result := sf.FoldSteps(messages, steps, currentStepID)

	// Find folded summary
	foldedContent := ""
	for _, msg := range result {
		if strings.Contains(msg.Content, "step_1") && strings.Contains(msg.Content, "step_2") {
			foldedContent = msg.Content
			break
		}
	}

	if foldedContent == "" {
		t.Fatal("Should find folded summary with both step_1 and step_2")
	}
	if !strings.Contains(foldedContent, "Result 1") {
		t.Error("Folded summary should include step_1 result")
	}
	if !strings.Contains(foldedContent, "Result 2") {
		t.Error("Folded summary should include step_2 result")
	}
}

func TestStepFolder_StepWithNoResult(t *testing.T) {
	sf := NewStepFolder()

	messages := []core.LLMMessage{
		{Role: "system", Content: "System prompt."},
		{Role: "user", Content: "User input."},
		{Role: "assistant", Content: "Step 1."},
		{Role: "assistant", Content: "Step 2."},
	}

	steps := []core.StepExecution{
		{StepID: "step_1", Status: core.StepCompleted},
		{StepID: "step_2", Status: core.StepRunning},
	}
	currentStepID := "step_2"

	result := sf.FoldSteps(messages, steps, currentStepID)

	// Should not crash, folded summary should exist
	found := false
	for _, msg := range result {
		if strings.Contains(msg.Content, "step_1") {
			found = true
		}
	}
	if !found {
		t.Error("Should find folded step_1 even with no result")
	}
}

func TestStepFolder_EvidenceFormatting(t *testing.T) {
	sf := NewStepFolder()

	messages := []core.LLMMessage{
		{Role: "system", Content: "System prompt."},
		{Role: "user", Content: "User input."},
		{Role: "assistant", Content: "Step 1."},
		{Role: "assistant", Content: "Step 2."},
	}

	steps := []core.StepExecution{
		{
			StepID:   "step_1",
			Status:   core.StepCompleted,
			Result:   "Found suspicious process",
			Evidence: []string{"pid=1234", "ppid=889", "cmdline=bash"},
		},
		{StepID: "step_2", Status: core.StepRunning},
	}
	currentStepID := "step_2"

	result := sf.FoldSteps(messages, steps, currentStepID)

	// Find folded summary
	for _, msg := range result {
		if strings.Contains(msg.Content, "step_1") {
			if !strings.Contains(msg.Content, "pid=1234") {
				t.Error("Folded step should include evidence")
			}
			return
		}
	}
	t.Error("Should find folded step_1")
}
