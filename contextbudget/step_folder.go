package contextbudget

import (
	"fmt"
	"strings"

	"github.com/alex-chenc/agent-runtime/core"
)

// StepFolder folds non-current steps into compact summaries.
// Preserves system prompt, user input, and current step's full messages.
type StepFolder struct{}

// NewStepFolder creates a new StepFolder.
func NewStepFolder() *StepFolder {
	return &StepFolder{}
}

// FoldSteps folds non-current completed steps into a single summary message.
// Returns a new message list where:
// - System prompt and user input are preserved at the start
// - All previous (non-current) steps are collapsed into one summary message
// - Current step's messages are preserved in full
func (sf *StepFolder) FoldSteps(messages []core.LLMMessage, steps []core.StepExecution, currentStepID string) []core.LLMMessage {
	if len(steps) <= 1 {
		return messages
	}

	// Find which steps to fold (all except current)
	var foldable []core.StepExecution
	for _, step := range steps {
		if step.StepID != currentStepID {
			foldable = append(foldable, step)
		}
	}

	if len(foldable) == 0 {
		return messages
	}

	// Build the folded summary
	summary := sf.buildFoldedSummary(foldable)

	// Reconstruct messages: system, user, folded summary, current step messages
	var result []core.LLMMessage

	// Find the boundary between setup (system+user) and step messages
	setupEnd := 0
	for i, msg := range messages {
		if msg.Role == "system" || msg.Role == "user" {
			setupEnd = i + 1
		} else {
			break
		}
	}

	// Keep system and user messages
	result = append(result, messages[:setupEnd]...)

	// Add folded summary as a user message
	result = append(result, core.LLMMessage{
		Role:    "user",
		Content: summary,
	})

	// Find messages belonging to the current step
	currentStepMessages := sf.findCurrentStepMessages(messages, steps, currentStepID, setupEnd)
	result = append(result, currentStepMessages...)

	return result
}

// buildFoldedSummary creates a text summary of folded steps.
func (sf *StepFolder) buildFoldedSummary(steps []core.StepExecution) string {
	var sb strings.Builder
	sb.WriteString("Previous completed steps:\n")

	for _, step := range steps {
		status := string(step.Status)
		sb.WriteString(fmt.Sprintf("- %s [%s]", step.StepID, status))

		if step.Result != "" {
			sb.WriteString(fmt.Sprintf(": %s", step.Result))
		}
		sb.WriteString("\n")

		// Evidence
		if len(step.Evidence) > 0 {
			sb.WriteString(fmt.Sprintf("  Evidence: %s\n", strings.Join(step.Evidence, ", ")))
		}

		// Error
		if step.Error != nil {
			sb.WriteString(fmt.Sprintf("  Error: %s\n", step.Error.Message))
		}

		// Tool call summary from ReactTurns
		toolNames := sf.extractToolNames(step.ReactTurns)
		if len(toolNames) > 0 {
			sb.WriteString(fmt.Sprintf("  Tools used: %s\n", strings.Join(toolNames, ", ")))
		}
	}

	return sb.String()
}

// extractToolNames gets unique tool names from ReactTurns.
func (sf *StepFolder) extractToolNames(turns []core.ReactTurn) []string {
	seen := make(map[string]bool)
	var names []string
	for _, turn := range turns {
		name := turn.Action.ToolName
		if name != "" && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	return names
}

// findCurrentStepMessages finds messages belonging to the current step.
// Heuristic: keep the last N messages that correspond to the current step's turns.
func (sf *StepFolder) findCurrentStepMessages(messages []core.LLMMessage, steps []core.StepExecution, currentStepID string, setupEnd int) []core.LLMMessage {
	if len(messages) <= setupEnd {
		return nil
	}

	afterSetup := messages[setupEnd:]

	// Find the current step to get its turn count
	for _, step := range steps {
		if step.StepID == currentStepID {
			// Each turn = assistant + tool = 2 messages
			turnCount := len(step.ReactTurns)
			if turnCount <= 0 {
				turnCount = 1 // at least 1 turn
			}
			keepMessages := turnCount * 2
			if keepMessages > len(afterSetup) {
				keepMessages = len(afterSetup)
			}
			return afterSetup[len(afterSetup)-keepMessages:]
		}
	}

	// Fallback: keep last 2 messages
	if len(afterSetup) > 2 {
		return afterSetup[len(afterSetup)-2:]
	}
	return afterSetup
}
