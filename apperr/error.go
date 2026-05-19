package apperr

import (
	"time"

	"github.com/alex-chenc/agent-runtime/core"
)

// New creates a new RuntimeError with the given parameters.
func New(kind core.ErrorKind, stage, taskID, stepID, message string) core.RuntimeError {
	return core.RuntimeError{
		ErrorID:    "",
		Kind:       kind,
		Stage:      stage,
		TaskID:     taskID,
		StepID:     stepID,
		Message:    message,
		OccurredAt: time.Now(),
	}
}

// Recoverable creates a new recoverable RuntimeError.
func Recoverable(kind core.ErrorKind, stage, taskID, stepID, message string) core.RuntimeError {
	e := New(kind, stage, taskID, stepID, message)
	e.Recoverable = true
	return e
}

// WithCause adds a cause to the error.
func WithCause(e core.RuntimeError, cause string) core.RuntimeError {
	e.Cause = cause
	return e
}

// WithAction adds an action taken to the error.
func WithAction(e core.RuntimeError, action string) core.RuntimeError {
	e.ActionTaken = action
	return e
}

// WithToolCallID adds a tool call ID to the error.
func WithToolCallID(e core.RuntimeError, callID string) core.RuntimeError {
	e.ToolCallID = callID
	return e
}

// WithModelCallID adds a model call ID to the error.
func WithModelCallID(e core.RuntimeError, callID string) core.RuntimeError {
	e.ModelCallID = callID
	return e
}
