package agentruntime

import "github.com/alex-chenc/agent-runtime/core"

// Re-export all enum types from core for backward-compatible public API.

type TaskStatus = core.TaskStatus
type StepStatus = core.StepStatus
type ExitReason = core.ExitReason
type RiskLevel = core.RiskLevel
type ReactActionType = core.ReactActionType
type AuditDecision = core.AuditDecision
type ReflectionDecision = core.ReflectionDecision
type CorrectionActionType = core.CorrectionActionType
type HookEventType = core.HookEventType
type LLMPurpose = core.LLMPurpose
type ToolCallStatus = core.ToolCallStatus
type ToolPolicyDecision = core.ToolPolicyDecision
type ErrorKind = core.ErrorKind

// Re-export all constants.
const (
	StatusInitializing = core.StatusInitializing
	StatusPlanning     = core.StatusPlanning
	StatusPlanFailed   = core.StatusPlanFailed
	StatusRunning      = core.StatusRunning
	StatusWaitingTool  = core.StatusWaitingTool
	StatusAuditing     = core.StatusAuditing
	StatusReflecting   = core.StatusReflecting
	StatusCorrecting   = core.StatusCorrecting
	StatusSummarizing  = core.StatusSummarizing
	StatusCompleted    = core.StatusCompleted
	StatusFailed       = core.StatusFailed
	StatusInterrupted  = core.StatusInterrupted
	StatusLimited      = core.StatusLimited
)

const (
	StepPending     = core.StepPending
	StepRunning     = core.StepRunning
	StepWaitingTool = core.StepWaitingTool
	StepCompleted   = core.StepCompleted
	StepFailed      = core.StepFailed
	StepSkipped     = core.StepSkipped
	StepRetrying    = core.StepRetrying
	StepReplaced    = core.StepReplaced
	StepInvalidated = core.StepInvalidated
)

const (
	ExitNormalCompleted         = core.ExitNormalCompleted
	ExitUserInterrupted         = core.ExitUserInterrupted
	ExitTaskTimeout             = core.ExitTaskTimeout
	ExitMaxTotalTurns           = core.ExitMaxTotalTurns
	ExitMaxToolCalls            = core.ExitMaxToolCalls
	ExitMaxToolFailures         = core.ExitMaxToolFailures
	ExitMaxModelFailures        = core.ExitMaxModelFailures
	ExitMaxParseFailures        = core.ExitMaxParseFailures
	ExitNoProgress              = core.ExitNoProgress
	ExitPlanGenerationFailed    = core.ExitPlanGenerationFailed
	ExitPlanValidationFailed    = core.ExitPlanValidationFailed
	ExitAuditUnrecoverable      = core.ExitAuditUnrecoverable
	ExitReflectionUnrecoverable = core.ExitReflectionUnrecoverable
	ExitToolUnavailable         = core.ExitToolUnavailable
	ExitModelUnavailable        = core.ExitModelUnavailable
	ExitSystemError             = core.ExitSystemError
	ExitContextOverflow         = core.ExitContextOverflow
)

const (
	RiskReadOnly  = core.RiskReadOnly
	RiskLow       = core.RiskLow
	RiskHigh      = core.RiskHigh
	RiskDangerous = core.RiskDangerous
)

const (
	ActionToolCall          = core.ActionToolCall
	ActionStepResult        = core.ActionStepResult
	ActionRequestExperience = core.ActionRequestExperience
	ActionNeedUserInput     = core.ActionNeedUserInput
	ActionFailStep          = core.ActionFailStep
)

const (
	AuditContinue          = core.AuditContinue
	AuditMinorAdjustment   = core.AuditMinorAdjustment
	AuditCorrectPlan       = core.AuditCorrectPlan
	AuditRequestExperience = core.AuditRequestExperience
	AuditSummarizeNow      = core.AuditSummarizeNow
	AuditFail              = core.AuditFail
)

const (
	ReflectRetryStep         = core.ReflectRetryStep
	ReflectSkipStep          = core.ReflectSkipStep
	ReflectCorrectPlan       = core.ReflectCorrectPlan
	ReflectRequestExperience = core.ReflectRequestExperience
	ReflectSummarizeNow      = core.ReflectSummarizeNow
	ReflectFail              = core.ReflectFail
)

const (
	CorrectionAddStep      = core.CorrectionAddStep
	CorrectionSkipStep     = core.CorrectionSkipStep
	CorrectionReplaceStep  = core.CorrectionReplaceStep
	CorrectionSplitStep    = core.CorrectionSplitStep
	CorrectionMergeSteps   = core.CorrectionMergeSteps
	CorrectionReorderSteps = core.CorrectionReorderSteps
	CorrectionReplaceTool  = core.CorrectionReplaceTool
	CorrectionReduceScope  = core.CorrectionReduceScope
	CorrectionSummarizeNow = core.CorrectionSummarizeNow
)

const (
	HookTaskStarted        = core.HookTaskStarted
	HookExperienceLoaded   = core.HookExperienceLoaded
	HookPlanCreated        = core.HookPlanCreated
	HookStepStarted        = core.HookStepStarted
	HookModelCallStarted   = core.HookModelCallStarted
	HookModelCallFinished  = core.HookModelCallFinished
	HookToolCallStarted    = core.HookToolCallStarted
	HookToolCallFinished   = core.HookToolCallFinished
	HookStepCompleted      = core.HookStepCompleted
	HookStepFailed         = core.HookStepFailed
	HookAuditStarted       = core.HookAuditStarted
	HookAuditFinished      = core.HookAuditFinished
	HookReflectionStarted  = core.HookReflectionStarted
	HookReflectionFinished = core.HookReflectionFinished
	HookCorrectionApplied  = core.HookCorrectionApplied
	HookStepRetrying       = core.HookStepRetrying
	HookStepSkipped        = core.HookStepSkipped
	HookConfigChanged            = core.HookConfigChanged
	HookTaskInterrupted          = core.HookTaskInterrupted
	HookTaskFinished             = core.HookTaskFinished
	HookContextBudgetChecked     = core.HookContextBudgetChecked
	HookContextCompressed        = core.HookContextCompressed
	HookContextCompressionFailed = core.HookContextCompressionFailed
)

const (
	PurposePlan      = core.PurposePlan
	PurposeReact     = core.PurposeReact
	PurposeAudit     = core.PurposeAudit
	PurposeReflect   = core.PurposeReflect
	PurposeCorrect   = core.PurposeCorrect
	PurposeSummarize = core.PurposeSummarize
	PurposeCompress  = core.PurposeCompress
)

const (
	ToolCallSuccess   = core.ToolCallSuccess
	ToolCallFailed    = core.ToolCallFailed
	ToolCallTimeout   = core.ToolCallTimeout
	ToolCallCancelled = core.ToolCallCancelled
)

const (
	PolicyAllow           = core.PolicyAllow
	PolicyDeny            = core.PolicyDeny
	PolicyRequireApproval = core.PolicyRequireApproval
	PolicyReplaceTool     = core.PolicyReplaceTool
)

// Re-export ErrorKind constants.
const (
	ErrConfig           = core.ErrConfig
	ErrPlanGeneration   = core.ErrPlanGeneration
	ErrPlanValidation   = core.ErrPlanValidation
	ErrModelCall        = core.ErrModelCall
	ErrModelParse       = core.ErrModelParse
	ErrToolNotFound     = core.ErrToolNotFound
	ErrToolPolicyDenied = core.ErrToolPolicyDenied
	ErrToolCall         = core.ErrToolCall
	ErrToolTimeout      = core.ErrToolTimeout
	ErrExperience       = core.ErrExperience
	ErrAudit            = core.ErrAudit
	ErrCorrection       = core.ErrCorrection
	ErrInterrupt        = core.ErrInterrupt
	ErrSystem           = core.ErrSystem
)

// Re-export data types that were in types.go.
type ModelCallRecord = core.ModelCallRecord
type ConfigChange = core.ConfigChange
type TimelineEvent = core.TimelineEvent

// Context budget types
type CompressionStrategy = core.CompressionStrategy
type CompressionRecord = core.CompressionRecord
type ContextBudgetSnapshot = core.ContextBudgetSnapshot
