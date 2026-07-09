package agentruntime

import (
	"github.com/alex-chenc/agent-runtime/core"
	"github.com/alex-chenc/agent-runtime/task"
)

// Re-export all result/data types from core.

type TaskResult = core.TaskResult
type CompletionSummary = core.CompletionSummary
type RuntimeMetrics = core.RuntimeMetrics
type StepExecution = core.StepExecution
type ReactTurn = core.ReactTurn
type ReactAction = core.ReactAction
type Observation = core.Observation
type ToolCallRecord = core.ToolCallRecord
type ToolOutcome = core.ToolOutcome
type ExperienceUsage = core.ExperienceUsage
type RuntimeError = core.RuntimeError
type ExitDecision = core.ExitDecision
type ReflectionResult = core.ReflectionResult
type AuditResult = core.AuditResult
type CorrectionResult = core.CorrectionResult
type CorrectionAction = core.CorrectionAction
type Plan = core.Plan
type PlanStep = core.PlanStep
type TaskSnapshot = core.TaskSnapshot
type TaskInput = task.TaskInput
