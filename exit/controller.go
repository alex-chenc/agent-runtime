package exit

import (
	"github.com/alex-chenc/agent-runtime/core"
	"github.com/alex-chenc/agent-runtime/internal/limiter"
)

// Controller checks exit conditions at key points in the task lifecycle.
type Controller struct {
	config core.RuntimeConfig
}

// NewController creates a new exit controller.
func NewController(config core.RuntimeConfig) *Controller {
	return &Controller{config: config}
}

// Check evaluates all exit conditions and returns a decision.
func (c *Controller) Check(interrupted bool, limits *limiter.Limiter) core.ExitDecision {
	if interrupted {
		return core.ExitDecision{
			ShouldExit: true,
			Reason:     core.ExitUserInterrupted,
			Message:    "task was interrupted by user",
		}
	}
	if limits.ExceedsTotalTurns(c.config.MaxTotalTurns) {
		return core.ExitDecision{
			ShouldExit: true,
			Reason:     core.ExitMaxTotalTurns,
			Message:    "max total turns exceeded",
		}
	}
	if limits.ExceedsToolCalls(c.config.MaxToolCalls) {
		return core.ExitDecision{
			ShouldExit: true,
			Reason:     core.ExitMaxToolCalls,
			Message:    "max tool calls exceeded",
		}
	}
	if limits.ExceedsToolFailures(c.config.MaxToolFailures) {
		return core.ExitDecision{
			ShouldExit: true,
			Reason:     core.ExitMaxToolFailures,
			Message:    "max tool failures exceeded",
		}
	}
	if limits.ExceedsModelFailures(c.config.MaxModelFailures) {
		return core.ExitDecision{
			ShouldExit: true,
			Reason:     core.ExitMaxModelFailures,
			Message:    "max model failures exceeded",
		}
	}
	if limits.ExceedsParseFailures(c.config.MaxParseFailures) {
		return core.ExitDecision{
			ShouldExit: true,
			Reason:     core.ExitMaxParseFailures,
			Message:    "max parse failures exceeded",
		}
	}
	return core.ExitDecision{ShouldExit: false}
}

// CheckStep checks step-level exit conditions.
func (c *Controller) CheckStep(toolCallsInStep int) core.ExitDecision {
	if toolCallsInStep >= c.config.MaxToolCallsPerStep {
		return core.ExitDecision{
			ShouldExit: true,
			Reason:     core.ExitMaxToolCalls,
			Message:    "max tool calls per step exceeded",
		}
	}
	return core.ExitDecision{ShouldExit: false}
}
