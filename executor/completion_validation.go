package executor

import (
	"fmt"
	"strings"

	"github.com/alex-chenc/agent-runtime/core"
)

type completionValidationResult struct {
	Passed bool
	Reason string
}

// validateStepCompletion treats an LLM step_result as a proposal. Tool-backed
// steps require real terminal evidence, and a failed call can only be recovered
// by explicitly citing a later successful terminal call.
func validateStepCompletion(step *core.PlanStep, action core.ReactAction, calls []core.ToolCallRecord) completionValidationResult {
	if len(calls) == 0 {
		if step != nil && (len(step.AllowedTools) > 0 || len(step.SuggestedTools) > 0) {
			return completionValidationResult{Reason: "tool-backed step has no tool-call evidence"}
		}
		return completionValidationResult{Passed: true}
	}

	callIndexes := make(map[string]int, len(calls))
	for index, call := range calls {
		callIndexes[call.CallID] = index
	}

	evidenceIndexes := make([]int, 0, len(action.Evidence))
	for _, evidence := range action.Evidence {
		if index, ok := callIndexes[strings.TrimSpace(evidence)]; ok {
			evidenceIndexes = append(evidenceIndexes, index)
		}
	}

	lastFailure := -1
	for index, call := range calls {
		if call.Status != core.ToolCallSuccess {
			lastFailure = index
		}
	}

	// New outcome-aware callers must cite a concrete call ID. Legacy callers
	// without ToolOutcome remain compatible when every call succeeded.
	outcomeAware := false
	for _, call := range calls {
		if call.Outcome != nil {
			outcomeAware = true
			break
		}
	}
	if len(evidenceIndexes) == 0 && (outcomeAware || lastFailure >= 0) {
		return completionValidationResult{Reason: "step_result must cite a successful terminal tool call ID"}
	}

	candidates := evidenceIndexes
	if len(candidates) == 0 {
		candidates = []int{len(calls) - 1}
	}

	var terminalSuccess bool
	for _, index := range candidates {
		call := calls[index]
		if call.Status != core.ToolCallSuccess {
			continue
		}
		if call.Outcome != nil {
			if !call.Outcome.Terminal ||
				(call.Outcome.OperationStatus != core.OperationSucceeded && call.Outcome.OperationStatus != core.OperationSkipped) {
				continue
			}
		}
		if index > lastFailure {
			terminalSuccess = true
			break
		}
	}
	if terminalSuccess {
		return completionValidationResult{Passed: true}
	}

	if lastFailure >= 0 {
		return completionValidationResult{Reason: fmt.Sprintf("tool call %s failed without later cited terminal recovery", calls[lastFailure].CallID)}
	}
	return completionValidationResult{Reason: "cited tool evidence is not a terminal successful outcome"}
}
