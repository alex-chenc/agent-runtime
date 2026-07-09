package core

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ===== Status Enums =====

type TaskStatus string

const (
	StatusInitializing TaskStatus = "initializing"
	StatusPlanning     TaskStatus = "planning"
	StatusPlanFailed   TaskStatus = "plan_failed"
	StatusRunning      TaskStatus = "running"
	StatusWaitingTool  TaskStatus = "waiting_tool"
	StatusAuditing     TaskStatus = "auditing"
	StatusReflecting   TaskStatus = "reflecting"
	StatusCorrecting   TaskStatus = "correcting"
	StatusSummarizing  TaskStatus = "summarizing"
	StatusCompleted    TaskStatus = "completed"
	StatusFailed       TaskStatus = "failed"
	StatusInterrupted  TaskStatus = "interrupted"
	StatusLimited      TaskStatus = "limited"
)

type StepStatus string

const (
	StepPending     StepStatus = "pending"
	StepRunning     StepStatus = "running"
	StepWaitingTool StepStatus = "waiting_tool"
	StepCompleted   StepStatus = "completed"
	StepFailed      StepStatus = "failed"
	StepSkipped     StepStatus = "skipped"
	StepRetrying    StepStatus = "retrying"
	StepReplaced    StepStatus = "replaced"
	StepInvalidated StepStatus = "invalidated"
)

type ExitReason string

const (
	ExitNormalCompleted         ExitReason = "normal_completed"
	ExitUserInterrupted         ExitReason = "user_interrupted"
	ExitTaskTimeout             ExitReason = "task_timeout"
	ExitMaxTotalTurns           ExitReason = "max_total_turns"
	ExitMaxToolCalls            ExitReason = "max_tool_calls"
	ExitMaxToolFailures         ExitReason = "max_tool_failures"
	ExitMaxModelFailures        ExitReason = "max_model_failures"
	ExitMaxParseFailures        ExitReason = "max_parse_failures"
	ExitNoProgress              ExitReason = "no_progress"
	ExitPlanGenerationFailed    ExitReason = "plan_generation_failed"
	ExitPlanValidationFailed    ExitReason = "plan_validation_failed"
	ExitAuditUnrecoverable      ExitReason = "audit_unrecoverable"
	ExitReflectionUnrecoverable ExitReason = "reflection_unrecoverable"
	ExitToolUnavailable         ExitReason = "tool_unavailable"
	ExitModelUnavailable        ExitReason = "model_unavailable"
	ExitSystemError             ExitReason = "system_error"
	ExitContextOverflow         ExitReason = "context_overflow"
)

type RiskLevel string

const (
	RiskReadOnly  RiskLevel = "read_only"
	RiskLow       RiskLevel = "low"
	RiskHigh      RiskLevel = "high"
	RiskDangerous RiskLevel = "dangerous"
)

type ReactActionType string

const (
	ActionToolCall          ReactActionType = "tool_call"
	ActionStepResult        ReactActionType = "step_result"
	ActionRequestExperience ReactActionType = "request_experience"
	ActionNeedUserInput     ReactActionType = "need_user_input"
	ActionFailStep          ReactActionType = "fail_step"
)

type AuditDecision string

const (
	AuditContinue          AuditDecision = "continue"
	AuditMinorAdjustment   AuditDecision = "minor_adjustment"
	AuditCorrectPlan       AuditDecision = "correct_plan"
	AuditRequestExperience AuditDecision = "request_experience"
	AuditSummarizeNow      AuditDecision = "summarize_now"
	AuditFail              AuditDecision = "fail"
)

type ReflectionDecision string

const (
	ReflectRetryStep         ReflectionDecision = "retry_step"
	ReflectSkipStep          ReflectionDecision = "skip_step"
	ReflectCorrectPlan       ReflectionDecision = "correct_plan"
	ReflectRequestExperience ReflectionDecision = "request_experience"
	ReflectSummarizeNow      ReflectionDecision = "summarize_now"
	ReflectFail              ReflectionDecision = "fail"
)

type CorrectionActionType string

const (
	CorrectionAddStep      CorrectionActionType = "add_step"
	CorrectionSkipStep     CorrectionActionType = "skip_step"
	CorrectionReplaceStep  CorrectionActionType = "replace_step"
	CorrectionSplitStep    CorrectionActionType = "split_step"
	CorrectionMergeSteps   CorrectionActionType = "merge_steps"
	CorrectionReorderSteps CorrectionActionType = "reorder_steps"
	CorrectionReplaceTool  CorrectionActionType = "replace_tool"
	CorrectionReduceScope  CorrectionActionType = "reduce_scope"
	CorrectionSummarizeNow CorrectionActionType = "summarize_now"
)

type HookEventType string

const (
	HookTaskStarted              HookEventType = "task_started"
	HookExperienceLoaded         HookEventType = "experience_loaded"
	HookPlanCreated              HookEventType = "plan_created"
	HookStepStarted              HookEventType = "step_started"
	HookModelCallStarted         HookEventType = "model_call_started"
	HookModelCallFinished        HookEventType = "model_call_finished"
	HookToolCallStarted          HookEventType = "tool_call_started"
	HookToolCallFinished         HookEventType = "tool_call_finished"
	HookStepCompleted            HookEventType = "step_completed"
	HookStepFailed               HookEventType = "step_failed"
	HookAuditStarted             HookEventType = "audit_started"
	HookAuditFinished            HookEventType = "audit_finished"
	HookReflectionStarted        HookEventType = "reflection_started"
	HookReflectionFinished       HookEventType = "reflection_finished"
	HookCorrectionApplied        HookEventType = "correction_applied"
	HookStepRetrying             HookEventType = "step_retrying"
	HookStepSkipped              HookEventType = "step_skipped"
	HookConfigChanged            HookEventType = "config_changed"
	HookTaskInterrupted          HookEventType = "task_interrupted"
	HookTaskFinished             HookEventType = "task_finished"
	HookContextBudgetChecked     HookEventType = "context_budget_checked"
	HookContextCompressed        HookEventType = "context_compressed"
	HookContextCompressionFailed HookEventType = "context_compression_failed"
)

type LLMPurpose string

const (
	PurposePlan      LLMPurpose = "plan"
	PurposeReact     LLMPurpose = "react"
	PurposeAudit     LLMPurpose = "audit"
	PurposeReflect   LLMPurpose = "reflect"
	PurposeCorrect   LLMPurpose = "correct"
	PurposeSummarize LLMPurpose = "summarize"
	PurposeCompress  LLMPurpose = "compress"
	PurposeClassify  LLMPurpose = "classify" // 任务分类和提示词选择
)

// ===== Task Router Types =====

// TaskAction 路由动作
type TaskAction string

const (
	ActionDirectReply TaskAction = "direct_reply" // 直接回复（问候、闲聊）
	ActionSimpleCall  TaskAction = "simple_call"  // 简单工具调用（跳过计划）
	ActionFullPlan    TaskAction = "full_plan"    // 完整计划流程
)

// PromptFragment 提示词片段
type PromptFragment struct {
	Name        string   `json:"name"`        // 唯一名称
	Description string   `json:"description"` // 功能描述（供 LLM 选择时参考）
	Keywords    []string `json:"keywords"`    // 匹配关键词
	Priority    int      `json:"priority"`    // 优先级（数字越大越靠前）
	Content     string   `json:"content"`     // 提示词内容
}

// TaskClassification LLM 语义分析结果
type TaskClassification struct {
	TaskType   string     `json:"task_type"`  // greeting, query, analysis, investigation, remediation
	Intent     string     `json:"intent"`     // 用户意图描述
	Complexity string     `json:"complexity"` // simple, moderate, complex
	Action     TaskAction `json:"action"`     // 推荐动作
	Fragments  []string   `json:"fragments"`  // 选中的片段名称列表
	Reason     string     `json:"reason"`     // 判断原因
}

// RouteResult 路由结果
type RouteResult struct {
	Action            TaskAction          // 推荐动作
	Classification    *TaskClassification // LLM 分类结果（规则匹配时为 nil）
	SelectedFragments []string            // 选中的片段名称列表
	ComposedPrompt    string              // 拼接后的完整提示词
}

// CompressionStrategy identifies the type of context compression applied.
type CompressionStrategy string

const (
	StrategyToolResults     CompressionStrategy = "tool_results"
	StrategyHistoricalSteps CompressionStrategy = "historical_steps"
	StrategyLLMPriorTurns   CompressionStrategy = "llm_prior_turns"
	StrategyPreflight       CompressionStrategy = "preflight"
	StrategyEmergency       CompressionStrategy = "emergency"
)

type ToolCallStatus string

const (
	ToolCallSuccess   ToolCallStatus = "success"
	ToolCallFailed    ToolCallStatus = "failed"
	ToolCallTimeout   ToolCallStatus = "timeout"
	ToolCallCancelled ToolCallStatus = "cancelled"
)

type ToolPolicyDecision string

const (
	PolicyAllow           ToolPolicyDecision = "allow"
	PolicyDeny            ToolPolicyDecision = "deny"
	PolicyRequireApproval ToolPolicyDecision = "require_approval"
	PolicyReplaceTool     ToolPolicyDecision = "replace_tool"
)

type ErrorKind string

const (
	ErrConfig           ErrorKind = "config_error"
	ErrPlanGeneration   ErrorKind = "plan_generation_error"
	ErrPlanValidation   ErrorKind = "plan_validation_error"
	ErrModelCall        ErrorKind = "model_call_error"
	ErrModelParse       ErrorKind = "model_parse_error"
	ErrToolNotFound     ErrorKind = "tool_not_found"
	ErrToolPolicyDenied ErrorKind = "tool_policy_denied"
	ErrToolCall         ErrorKind = "tool_call_error"
	ErrToolTimeout      ErrorKind = "tool_timeout"
	ErrExperience       ErrorKind = "experience_error"
	ErrAudit            ErrorKind = "audit_error"
	ErrCorrection       ErrorKind = "correction_error"
	ErrInterrupt        ErrorKind = "interrupt"
	ErrSystem           ErrorKind = "system_error"
)

// ===== Core Data Structures =====

type ToolDescriptor struct {
	Name             string         `json:"name"`
	Description      string         `json:"description"`
	ArgsSchema       map[string]any `json:"args_schema,omitempty"`
	ResultSchema     map[string]any `json:"result_schema,omitempty"`
	RiskLevel        RiskLevel      `json:"risk_level"`
	AutoCallable     bool           `json:"auto_callable"`
	RequiresApproval bool           `json:"requires_approval"`
	DefaultTimeout   time.Duration  `json:"default_timeout"`
	Idempotent       bool           `json:"idempotent"`
	TypicalFailures  []string       `json:"typical_failures,omitempty"`
	Tags             []string       `json:"tags,omitempty"`
}

type Plan struct {
	PlanID      string     `json:"plan_id"`
	Version     int        `json:"version"`
	Goal        string     `json:"goal"`
	NeedsPlan   bool       `json:"needs_plan"`          // LLM 预评估：是否需要分步计划
	EstSteps    int        `json:"est_steps,omitempty"` // LLM 预评估：估计步骤数
	Assumptions []string   `json:"assumptions,omitempty"`
	Steps       []PlanStep `json:"steps"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type PlanStep struct {
	StepID         string   `json:"step_id"`
	Title          string   `json:"title"`
	Objective      string   `json:"objective"`
	ExpectedOutput string   `json:"expected_output"`
	SuggestedTools []string `json:"suggested_tools,omitempty"`
	// AllowedTools is a strict per-step tool allowlist supplied by the caller.
	// When empty, SuggestedTools keeps its legacy hint-only behavior.
	AllowedTools []string `json:"allowed_tools,omitempty"`
	// ToolArgs contains caller-bound arguments. These values are authoritative
	// and override same-name arguments proposed by the model.
	ToolArgs     map[string]any `json:"tool_args,omitempty"`
	Dependencies []string       `json:"dependencies,omitempty"`
	Status       StepStatus     `json:"status"`
	RetryCount   int            `json:"retry_count"`
	CreatedBy    string         `json:"created_by"`
	ChangeReason string         `json:"change_reason,omitempty"`
	RiskLevel    RiskLevel      `json:"risk_level"`
}

type TaskSnapshot struct {
	TaskID          string                 `json:"task_id"`
	UserInput       string                 `json:"user_input"`
	Status          TaskStatus             `json:"status"`
	ExitReason      ExitReason             `json:"exit_reason"`
	CurrentPlan     *Plan                  `json:"current_plan,omitempty"`
	CurrentStepID   string                 `json:"current_step_id,omitempty"`
	CompletedSteps  int                    `json:"completed_steps"`
	FailedSteps     int                    `json:"failed_steps"`
	TotalToolCalls  int                    `json:"total_tool_calls"`
	TotalModelCalls int                    `json:"total_model_calls"`
	RecentErrors    []RuntimeError         `json:"recent_errors,omitempty"`
	ContextBudget   *ContextBudgetSnapshot `json:"context_budget,omitempty"`
	StartedAt       time.Time              `json:"started_at"`
	Metadata        map[string]string      `json:"metadata,omitempty"`
}

type RuntimeError struct {
	ErrorID     string    `json:"error_id"`
	Kind        ErrorKind `json:"kind"`
	Stage       string    `json:"stage"`
	TaskID      string    `json:"task_id"`
	StepID      string    `json:"step_id,omitempty"`
	ToolCallID  string    `json:"tool_call_id,omitempty"`
	ModelCallID string    `json:"model_call_id,omitempty"`
	Message     string    `json:"message"`
	Recoverable bool      `json:"recoverable"`
	Cause       string    `json:"cause,omitempty"`
	ActionTaken string    `json:"action_taken,omitempty"`
	OccurredAt  time.Time `json:"occurred_at"`
}

type ExitDecision struct {
	ShouldExit bool       `json:"should_exit"`
	Reason     ExitReason `json:"reason,omitempty"`
	Message    string     `json:"message,omitempty"`
}

type ModelCallRecord struct {
	CallID           string        `json:"call_id"`
	TaskID           string        `json:"task_id"`
	StepID           string        `json:"step_id,omitempty"`
	Purpose          LLMPurpose    `json:"purpose"`
	InputSummary     string        `json:"input_summary"`
	OutputSummary    string        `json:"output_summary"`
	Schema           string        `json:"schema,omitempty"`
	Model            string        `json:"model,omitempty"`
	PromptTokens     int           `json:"prompt_tokens,omitempty"`
	CompletionTokens int           `json:"completion_tokens,omitempty"`
	TokensUsed       int           `json:"tokens_used,omitempty"`
	Latency          time.Duration `json:"latency"`
	Error            string        `json:"error,omitempty"`
	OccurredAt       time.Time     `json:"occurred_at"`
}

type ConfigChange struct {
	Field      string    `json:"field"`
	OldValue   string    `json:"old_value"`
	NewValue   string    `json:"new_value"`
	OccurredAt time.Time `json:"occurred_at"`
}

type TimelineEvent struct {
	EventID    string    `json:"event_id"`
	StepID     string    `json:"step_id,omitempty"`
	EventType  string    `json:"event_type"`
	Summary    string    `json:"summary"`
	OccurredAt time.Time `json:"occurred_at"`
}

type StepExecution struct {
	StepID        string        `json:"step_id"`
	Attempt       int           `json:"attempt"`
	Status        StepStatus    `json:"status"`
	StartedAt     time.Time     `json:"started_at"`
	EndedAt       time.Time     `json:"ended_at"`
	ReactTurns    []ReactTurn   `json:"react_turns"`
	Result        string        `json:"result,omitempty"`
	Evidence      []string      `json:"evidence,omitempty"`
	Error         *RuntimeError `json:"error,omitempty"`
	NoProgressHit bool          `json:"no_progress_hit"`
}

type ReactTurn struct {
	TurnIndex       int           `json:"turn_index"`
	ModelCallID     string        `json:"model_call_id"`
	Action          ReactAction   `json:"action"`
	ToolCallID      string        `json:"tool_call_id,omitempty"`
	Observation     *Observation  `json:"observation,omitempty"`
	ParseError      *RuntimeError `json:"parse_error,omitempty"`
	ProgressSummary string        `json:"progress_summary,omitempty"`
	StartedAt       time.Time     `json:"started_at"`
	EndedAt         time.Time     `json:"ended_at"`
}

type ReactAction struct {
	Type              ReactActionType `json:"type"`
	Summary           string          `json:"summary"`
	ToolName          string          `json:"tool_name,omitempty"`
	ToolArgs          map[string]any  `json:"tool_args,omitempty"`
	StepResult        string          `json:"step_result,omitempty"`
	Evidence          []string        `json:"evidence,omitempty"`
	Confidence        string          `json:"confidence,omitempty"`
	NeedsExperience   bool            `json:"needs_experience,omitempty"`
	ExperienceQuery   string          `json:"experience_query,omitempty"`
	NeedsUserInput    bool            `json:"needs_user_input,omitempty"`
	UserInputQuestion string          `json:"user_input_question,omitempty"`
	FailureReason     string          `json:"failure_reason,omitempty"`
	Recoverable       *bool           `json:"recoverable,omitempty"`
}

type Observation struct {
	ToolName string         `json:"tool_name"`
	CallID   string         `json:"call_id"`
	Status   ToolCallStatus `json:"status"`
	Content  string         `json:"content"`
	Summary  string         `json:"summary"`
	Error    string         `json:"error,omitempty"`
	Duration time.Duration  `json:"duration"`
}

type ToolCallRecord struct {
	CallID          string         `json:"call_id"`
	TaskID          string         `json:"task_id"`
	StepID          string         `json:"step_id"`
	ToolName        string         `json:"tool_name"`
	Reason          string         `json:"reason"`
	ArgsSummary     string         `json:"args_summary"`
	Status          ToolCallStatus `json:"status"`
	ResultSummary   string         `json:"result_summary"`
	ErrorMessage    string         `json:"error_message,omitempty"`
	ValidationStage string         `json:"validation_stage,omitempty"`
	RiskLevel       RiskLevel      `json:"risk_level"`
	StartedAt       time.Time      `json:"started_at"`
	EndedAt         time.Time      `json:"ended_at"`
}

type ToolValidationStage string

const (
	ToolValidationDescriptor  ToolValidationStage = "descriptor"
	ToolValidationPreparation ToolValidationStage = "preparation"
	ToolValidationArguments   ToolValidationStage = "arguments"
	ToolValidationPolicy      ToolValidationStage = "policy"
	ToolValidationStepScope   ToolValidationStage = "step_tool_scope"
)

// ToolCallValidationError identifies failures that happen before the concrete
// ToolGateway is reached, allowing callers to persist the real failure stage.
type ToolCallValidationError struct {
	Stage    ToolValidationStage `json:"stage"`
	ToolName string              `json:"tool_name,omitempty"`
	Message  string              `json:"message"`
}

func (e *ToolCallValidationError) Error() string {
	if e == nil {
		return ""
	}
	if e.ToolName == "" {
		return fmt.Sprintf("tool validation (%s): %s", e.Stage, e.Message)
	}
	return fmt.Sprintf("tool validation (%s) for %q: %s", e.Stage, e.ToolName, e.Message)
}

type ExperienceUsage struct {
	ItemID    string    `json:"item_id"`
	UsedAt    string    `json:"used_at_stage"`
	Helpful   bool      `json:"helpful"`
	Timestamp time.Time `json:"timestamp"`
}

type ReflectionResult struct {
	ReflectionID    string             `json:"reflection_id"`
	Trigger         string             `json:"trigger"`
	RootCause       string             `json:"root_cause"`
	Impact          string             `json:"impact"`
	Recoverable     bool               `json:"recoverable"`
	Recommendation  ReflectionDecision `json:"recommendation"`
	DisableTools    []string           `json:"disable_tools,omitempty"`
	CorrectionHint  string             `json:"correction_hint,omitempty"`
	ExperienceQuery string             `json:"experience_query,omitempty"`
	ReusableLesson  string             `json:"reusable_lesson,omitempty"`
	CreatedAt       time.Time          `json:"created_at"`
}

type AuditResult struct {
	AuditID        string        `json:"audit_id"`
	Trigger        string        `json:"trigger"`
	Drifted        bool          `json:"drifted"`
	RiskLevel      RiskLevel     `json:"risk_level"`
	Findings       []string      `json:"findings,omitempty"`
	Decision       AuditDecision `json:"decision"`
	CorrectionHint string        `json:"correction_hint,omitempty"`
	NeedExperience bool          `json:"need_experience"`
	NeedUserInput  bool          `json:"need_user_input"`
	ShouldExit     bool          `json:"should_exit"`
	ExitReason     ExitReason    `json:"exit_reason,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
}

type CorrectionResult struct {
	CorrectionID     string             `json:"correction_id"`
	Trigger          string             `json:"trigger"`
	FromPlanVersion  int                `json:"from_plan_version"`
	ToPlanVersion    int                `json:"to_plan_version"`
	Actions          []CorrectionAction `json:"actions"`
	Reason           string             `json:"reason"`
	Valid            bool               `json:"valid"`
	ValidationErrors []string           `json:"validation_errors,omitempty"`
	CreatedAt        time.Time          `json:"created_at"`
}

type CorrectionAction struct {
	Type         CorrectionActionType `json:"type"`
	StepID       string               `json:"step_id,omitempty"`
	NewStepID    string               `json:"new_step_id,omitempty"`
	TargetStepID string               `json:"target_step_id,omitempty"`
	Reason       string               `json:"reason"`
}

type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ResponseFormat specifies the desired output format for LLM calls.
// Used to request structured JSON output from providers that support it
// (e.g., DashScope/Bailian, OpenAI).
type ResponseFormat struct {
	Type       string                `json:"type"`
	JSONSchema *ResponseFormatSchema `json:"json_schema,omitempty"`
}

// ResponseFormatSchema defines a JSON Schema for structured output.
type ResponseFormatSchema struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Schema      json.RawMessage `json:"schema,omitempty"`
	Strict      bool            `json:"strict,omitempty"`
}

type LLMUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ExperienceItem struct {
	ID       string         `json:"id"`
	Summary  string         `json:"summary"`
	Content  string         `json:"content"`
	Tags     []string       `json:"tags,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type HookEvent struct {
	EventID   string        `json:"event_id"`
	TaskID    string        `json:"task_id"`
	StepID    string        `json:"step_id,omitempty"`
	Type      HookEventType `json:"type"`
	Payload   any           `json:"payload,omitempty"`
	Snapshot  *TaskSnapshot `json:"snapshot,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
}

type CompletionSummary struct {
	CompletedSteps  int      `json:"completed_steps"`
	FailedSteps     int      `json:"failed_steps"`
	SkippedSteps    int      `json:"skipped_steps"`
	ToolCalls       int      `json:"tool_calls"`
	ModelCalls      int      `json:"model_calls"`
	KeyFindings     []string `json:"key_findings,omitempty"`
	UnfinishedItems []string `json:"unfinished_items,omitempty"`
	Risks           []string `json:"risks,omitempty"`
	Recommendations []string `json:"recommendations,omitempty"`
}

// CompressionRecord documents a single compression event during task execution.
type CompressionRecord struct {
	CompressionID string              `json:"compression_id"`
	TaskID        string              `json:"task_id"`
	StepID        string              `json:"step_id,omitempty"`
	Strategy      CompressionStrategy `json:"strategy"`
	TriggerRatio  float64             `json:"trigger_ratio"`
	BeforeTokens  int                 `json:"before_tokens"`
	AfterTokens   int                 `json:"after_tokens"`
	CompressedRef []string            `json:"compressed_ref,omitempty"`
	Summary       string              `json:"summary"`
	ModelCallID   string              `json:"model_call_id,omitempty"`
	CreatedAt     time.Time           `json:"created_at"`
}

// ContextBudgetSnapshot captures the context budget state at a point in time.
type ContextBudgetSnapshot struct {
	MaxContextTokens      int     `json:"max_context_tokens"`
	ReservedOutputTokens  int     `json:"reserved_output_tokens"`
	EstimatedPromptTokens int     `json:"estimated_prompt_tokens"`
	ContextRatio          float64 `json:"context_ratio"`
	PromptTokensObserved  int     `json:"prompt_tokens_observed"`
	CompletionTokens      int     `json:"completion_tokens"`
	TotalTokens           int     `json:"total_tokens"`
	CompressionCount      int     `json:"compression_count"`
}

type RuntimeMetrics struct {
	TotalTurns            int           `json:"total_turns"`
	TotalToolCalls        int           `json:"total_tool_calls"`
	TotalModelCalls       int           `json:"total_model_calls"`
	TotalToolFailures     int           `json:"total_tool_failures"`
	TotalModelFailures    int           `json:"total_model_failures"`
	TotalParseFailures    int           `json:"total_parse_failures"`
	TotalAudits           int           `json:"total_audits"`
	TotalReflections      int           `json:"total_reflections"`
	TotalCorrections      int           `json:"total_corrections"`
	TotalPromptTokens     int           `json:"total_prompt_tokens"`
	TotalCompletionTokens int           `json:"total_completion_tokens"`
	TotalDuration         time.Duration `json:"total_duration"`
	ModelCallDuration     time.Duration `json:"model_call_duration"`
	ToolCallDuration      time.Duration `json:"tool_call_duration"`
}

type TaskResult struct {
	TaskID             string                 `json:"task_id"`
	Status             TaskStatus             `json:"status"`
	ExitReason         ExitReason             `json:"exit_reason"`
	FinalAnswer        string                 `json:"final_answer"`
	Completion         CompletionSummary      `json:"completion"`
	InitialPlan        *Plan                  `json:"initial_plan,omitempty"`
	FinalPlan          *Plan                  `json:"final_plan,omitempty"`
	StepExecutions     []StepExecution        `json:"step_executions"`
	ToolCalls          []ToolCallRecord       `json:"tool_calls"`
	ModelCalls         []ModelCallRecord      `json:"model_calls"`
	Errors             []RuntimeError         `json:"errors"`
	Reflections        []ReflectionResult     `json:"reflections"`
	Audits             []AuditResult          `json:"audits"`
	Corrections        []CorrectionResult     `json:"corrections"`
	ExperienceUsage    []ExperienceUsage      `json:"experience_usage,omitempty"`
	ConfigChanges      []ConfigChange         `json:"config_changes,omitempty"`
	CompressionRecords []CompressionRecord    `json:"compression_records,omitempty"`
	ContextBudget      *ContextBudgetSnapshot `json:"context_budget,omitempty"`
	Metrics            RuntimeMetrics         `json:"metrics"`
	StartedAt          time.Time              `json:"started_at"`
	EndedAt            time.Time              `json:"ended_at"`
	Metadata           map[string]string      `json:"metadata,omitempty"`
}

func (tr *TaskResult) ToJSON() ([]byte, error) {
	return json.MarshalIndent(tr, "", "  ")
}

// ===== Configuration =====

type RuntimeConfig struct {
	MaxTotalTurns         int           `json:"max_total_turns"`
	MaxPlanSteps          int           `json:"max_plan_steps"`
	MaxStepReactTurns     int           `json:"max_step_react_turns"`
	MaxToolCalls          int           `json:"max_tool_calls"`
	MaxToolCallsPerStep   int           `json:"max_tool_calls_per_step"`
	MaxToolFailures       int           `json:"max_tool_failures"`
	MaxModelFailures      int           `json:"max_model_failures"`
	MaxParseFailures      int           `json:"max_parse_failures"`
	MaxNoProgressTurns    int           `json:"max_no_progress_turns"`
	TaskTimeout           time.Duration `json:"task_timeout"`
	ModelTimeout          time.Duration `json:"model_timeout"`
	ToolTimeout           time.Duration `json:"tool_timeout"`
	HookTimeout           time.Duration `json:"hook_timeout"`
	EnableReflection      bool          `json:"enable_reflection"`
	EnableAudit           bool          `json:"enable_audit"`
	EnableCorrection      bool          `json:"enable_correction"`
	EnableExperience      bool          `json:"enable_experience"`
	AuditEveryNSteps      int           `json:"audit_every_n_steps"`
	AuditEveryNTurns      int           `json:"audit_every_n_turns"`
	MaxAudits             int           `json:"max_audits"`
	MaxCorrections        int           `json:"max_corrections"`
	MaxReflections        int           `json:"max_reflections"`
	MaxStepRetries        int           `json:"max_step_retries"`
	AllowDynamicNewSteps  bool          `json:"allow_dynamic_new_steps"`
	AllowSkipFailedStep   bool          `json:"allow_skip_failed_step"`
	AllowBestEffortAnswer bool          `json:"allow_best_effort_answer"`
	AllowHighRiskTools    bool          `json:"allow_high_risk_tools"`
	AllowDangerousTools   bool          `json:"allow_dangerous_tools"`
	DisabledTools         []string      `json:"disabled_tools,omitempty"`
	FailOnHookError       bool          `json:"fail_on_hook_error"`

	// Context budget and compression settings
	MaxContextTokens      int     `json:"max_context_tokens"`
	ReservedOutputTokens  int     `json:"reserved_output_tokens"`
	EnableContextCompress bool    `json:"enable_context_compress"`
	ToolCompressRatio     float64 `json:"tool_compress_ratio"`
	StepCompressRatio     float64 `json:"step_compress_ratio"`
	LLMCompressRatio      float64 `json:"llm_compress_ratio"`
	CompressTargetRatio   float64 `json:"compress_target_ratio"`
	RecentTurnsToKeep     int     `json:"recent_turns_to_keep"`
}

func DefaultConfig() RuntimeConfig {
	return RuntimeConfig{
		MaxTotalTurns: 40, MaxPlanSteps: 8, MaxStepReactTurns: 6,
		MaxToolCalls: 20, MaxToolCallsPerStep: 4, MaxToolFailures: 5,
		MaxModelFailures: 5, MaxParseFailures: 3, MaxNoProgressTurns: 3,
		TaskTimeout: 10 * time.Minute, ModelTimeout: 60 * time.Second,
		ToolTimeout: 60 * time.Second, HookTimeout: 10 * time.Second,
		EnableReflection: true, EnableAudit: true, EnableCorrection: true,
		EnableExperience: true, AuditEveryNSteps: 3, AuditEveryNTurns: 0,
		MaxAudits: 5, MaxCorrections: 3, MaxReflections: 5, MaxStepRetries: 2,
		AllowDynamicNewSteps: true, AllowSkipFailedStep: true,
		AllowBestEffortAnswer: true,
		MaxContextTokens:      256000, ReservedOutputTokens: 8192,
		EnableContextCompress: true, ToolCompressRatio: 0.70,
		StepCompressRatio: 0.80, LLMCompressRatio: 0.95,
		CompressTargetRatio: 0.60, RecentTurnsToKeep: 6,
	}
}

func (c RuntimeConfig) Validate() error {
	if c.MaxTotalTurns < 1 {
		return fmt.Errorf("config: MaxTotalTurns must be >= 1, got %d", c.MaxTotalTurns)
	}
	if c.MaxPlanSteps < 1 {
		return fmt.Errorf("config: MaxPlanSteps must be >= 1, got %d", c.MaxPlanSteps)
	}
	if c.MaxStepReactTurns < 1 {
		return fmt.Errorf("config: MaxStepReactTurns must be >= 1, got %d", c.MaxStepReactTurns)
	}
	if c.MaxToolCalls < 1 {
		return fmt.Errorf("config: MaxToolCalls must be >= 1, got %d", c.MaxToolCalls)
	}
	if c.MaxToolCallsPerStep < 1 {
		return fmt.Errorf("config: MaxToolCallsPerStep must be >= 1, got %d", c.MaxToolCallsPerStep)
	}
	if c.MaxToolFailures < 1 {
		return fmt.Errorf("config: MaxToolFailures must be >= 1, got %d", c.MaxToolFailures)
	}
	if c.MaxModelFailures < 1 {
		return fmt.Errorf("config: MaxModelFailures must be >= 1, got %d", c.MaxModelFailures)
	}
	if c.MaxParseFailures < 1 {
		return fmt.Errorf("config: MaxParseFailures must be >= 1, got %d", c.MaxParseFailures)
	}
	if c.MaxNoProgressTurns < 1 {
		return fmt.Errorf("config: MaxNoProgressTurns must be >= 1, got %d", c.MaxNoProgressTurns)
	}
	if c.TaskTimeout <= 0 {
		return fmt.Errorf("config: TaskTimeout must be > 0")
	}
	if c.ModelTimeout <= 0 {
		return fmt.Errorf("config: ModelTimeout must be > 0")
	}
	if c.ToolTimeout <= 0 {
		return fmt.Errorf("config: ToolTimeout must be > 0")
	}
	if c.HookTimeout <= 0 {
		return fmt.Errorf("config: HookTimeout must be > 0")
	}
	if c.EnableAudit && c.MaxAudits < 1 {
		return fmt.Errorf("config: MaxAudits must be >= 1 when audit is enabled")
	}
	if c.EnableCorrection && c.MaxCorrections < 1 {
		return fmt.Errorf("config: MaxCorrections must be >= 1 when correction is enabled")
	}
	if c.EnableReflection && c.MaxReflections < 1 {
		return fmt.Errorf("config: MaxReflections must be >= 1 when reflection is enabled")
	}
	if c.AllowDangerousTools && !c.AllowHighRiskTools {
		return fmt.Errorf("config: AllowDangerousTools requires AllowHighRiskTools")
	}
	if c.MaxStepRetries < 0 {
		return fmt.Errorf("config: MaxStepRetries must be >= 0, got %d", c.MaxStepRetries)
	}
	// Context budget validation
	if c.MaxContextTokens > 0 {
		if c.ReservedOutputTokens < 0 {
			return fmt.Errorf("config: ReservedOutputTokens must be >= 0, got %d", c.ReservedOutputTokens)
		}
		if c.ToolCompressRatio < 0 || c.ToolCompressRatio > 1 {
			return fmt.Errorf("config: ToolCompressRatio must be in [0,1], got %f", c.ToolCompressRatio)
		}
		if c.StepCompressRatio < 0 || c.StepCompressRatio > 1 {
			return fmt.Errorf("config: StepCompressRatio must be in [0,1], got %f", c.StepCompressRatio)
		}
		if c.LLMCompressRatio < 0 || c.LLMCompressRatio > 1 {
			return fmt.Errorf("config: LLMCompressRatio must be in [0,1], got %f", c.LLMCompressRatio)
		}
		if c.CompressTargetRatio < 0 || c.CompressTargetRatio > 1 {
			return fmt.Errorf("config: CompressTargetRatio must be in [0,1], got %f", c.CompressTargetRatio)
		}
		if c.RecentTurnsToKeep < 0 {
			return fmt.Errorf("config: RecentTurnsToKeep must be >= 0, got %d", c.RecentTurnsToKeep)
		}
	}
	return nil
}

type ConfigPatch struct {
	MaxTotalTurns         *int           `json:"max_total_turns,omitempty"`
	MaxPlanSteps          *int           `json:"max_plan_steps,omitempty"`
	MaxStepReactTurns     *int           `json:"max_step_react_turns,omitempty"`
	MaxToolCalls          *int           `json:"max_tool_calls,omitempty"`
	MaxToolCallsPerStep   *int           `json:"max_tool_calls_per_step,omitempty"`
	TaskTimeout           *time.Duration `json:"task_timeout,omitempty"`
	ModelTimeout          *time.Duration `json:"model_timeout,omitempty"`
	ToolTimeout           *time.Duration `json:"tool_timeout,omitempty"`
	EnableReflection      *bool          `json:"enable_reflection,omitempty"`
	EnableAudit           *bool          `json:"enable_audit,omitempty"`
	EnableCorrection      *bool          `json:"enable_correction,omitempty"`
	MaxStepRetries        *int           `json:"max_step_retries,omitempty"`
	DisabledTools         []string       `json:"disabled_tools,omitempty"`
	MaxContextTokens      *int           `json:"max_context_tokens,omitempty"`
	ReservedOutputTokens  *int           `json:"reserved_output_tokens,omitempty"`
	EnableContextCompress *bool          `json:"enable_context_compress,omitempty"`
	ToolCompressRatio     *float64       `json:"tool_compress_ratio,omitempty"`
	StepCompressRatio     *float64       `json:"step_compress_ratio,omitempty"`
	LLMCompressRatio      *float64       `json:"llm_compress_ratio,omitempty"`
	CompressTargetRatio   *float64       `json:"compress_target_ratio,omitempty"`
	RecentTurnsToKeep     *int           `json:"recent_turns_to_keep,omitempty"`
}

func (c RuntimeConfig) ApplyPatch(patch ConfigPatch) RuntimeConfig {
	result := c
	if patch.MaxTotalTurns != nil {
		result.MaxTotalTurns = *patch.MaxTotalTurns
	}
	if patch.MaxPlanSteps != nil {
		result.MaxPlanSteps = *patch.MaxPlanSteps
	}
	if patch.MaxStepReactTurns != nil {
		result.MaxStepReactTurns = *patch.MaxStepReactTurns
	}
	if patch.MaxToolCalls != nil {
		result.MaxToolCalls = *patch.MaxToolCalls
	}
	if patch.MaxToolCallsPerStep != nil {
		result.MaxToolCallsPerStep = *patch.MaxToolCallsPerStep
	}
	if patch.TaskTimeout != nil {
		result.TaskTimeout = *patch.TaskTimeout
	}
	if patch.ModelTimeout != nil {
		result.ModelTimeout = *patch.ModelTimeout
	}
	if patch.ToolTimeout != nil {
		result.ToolTimeout = *patch.ToolTimeout
	}
	if patch.EnableReflection != nil {
		result.EnableReflection = *patch.EnableReflection
	}
	if patch.EnableAudit != nil {
		result.EnableAudit = *patch.EnableAudit
	}
	if patch.EnableCorrection != nil {
		result.EnableCorrection = *patch.EnableCorrection
	}
	if patch.MaxStepRetries != nil {
		result.MaxStepRetries = *patch.MaxStepRetries
	}
	if patch.DisabledTools != nil {
		result.DisabledTools = patch.DisabledTools
	}
	if patch.MaxContextTokens != nil {
		result.MaxContextTokens = *patch.MaxContextTokens
	}
	if patch.ReservedOutputTokens != nil {
		result.ReservedOutputTokens = *patch.ReservedOutputTokens
	}
	if patch.EnableContextCompress != nil {
		result.EnableContextCompress = *patch.EnableContextCompress
	}
	if patch.ToolCompressRatio != nil {
		result.ToolCompressRatio = *patch.ToolCompressRatio
	}
	if patch.StepCompressRatio != nil {
		result.StepCompressRatio = *patch.StepCompressRatio
	}
	if patch.LLMCompressRatio != nil {
		result.LLMCompressRatio = *patch.LLMCompressRatio
	}
	if patch.CompressTargetRatio != nil {
		result.CompressTargetRatio = *patch.CompressTargetRatio
	}
	if patch.RecentTurnsToKeep != nil {
		result.RecentTurnsToKeep = *patch.RecentTurnsToKeep
	}
	return result
}

// ===== Interfaces =====

type LLMClient interface {
	Complete(ctx context.Context, req LLMRequest) (LLMResponse, error)
}

type LLMRequest struct {
	CallID         string            `json:"call_id,omitempty"`
	TaskID         string            `json:"task_id"`
	StepID         string            `json:"step_id,omitempty"`
	Purpose        LLMPurpose        `json:"purpose"`
	Messages       []LLMMessage      `json:"messages"`
	ResponseSchema string            `json:"response_schema,omitempty"`
	ResponseFormat *ResponseFormat   `json:"response_format,omitempty"`
	Temperature    *float32          `json:"temperature,omitempty"`
	Timeout        time.Duration     `json:"timeout"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

type LLMResponse struct {
	Content string   `json:"content"`
	Parsed  any      `json:"parsed,omitempty"`
	Model   string   `json:"model,omitempty"`
	Usage   LLMUsage `json:"usage,omitempty"`
	Raw     any      `json:"raw,omitempty"`
}

type ToolGateway interface {
	Call(ctx context.Context, req ToolRequest) (ToolResponse, error)
	Cancel(ctx context.Context, taskID string, callID string) error
}

// ToolRequestPreparer is an optional gateway capability used to resolve
// arguments derived from earlier tool results before runtime schema validation.
type ToolRequestPreparer interface {
	Prepare(ctx context.Context, req ToolRequest) (ToolRequest, error)
}

type ToolRequest struct {
	CallID     string            `json:"call_id"`
	TaskID     string            `json:"task_id"`
	StepID     string            `json:"step_id"`
	ToolName   string            `json:"tool_name"`
	Reason     string            `json:"reason"`
	Args       map[string]any    `json:"args"`
	Timeout    time.Duration     `json:"timeout"`
	RiskLevel  RiskLevel         `json:"risk_level"`
	Cancelable bool              `json:"cancelable"`
	Context    map[string]string `json:"context,omitempty"`
}

type ToolResponse struct {
	CallID       string            `json:"call_id"`
	ToolName     string            `json:"tool_name"`
	Status       ToolCallStatus    `json:"status"`
	Content      string            `json:"content"`
	Summary      string            `json:"summary"`
	ErrorMessage string            `json:"error_message,omitempty"`
	StartedAt    time.Time         `json:"started_at"`
	EndedAt      time.Time         `json:"ended_at"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

type ExperienceProvider interface {
	Fetch(ctx context.Context, req ExperienceRequest) (ExperienceResponse, error)
}

type ExperienceRequest struct {
	TaskID   string `json:"task_id"`
	Query    string `json:"query"`
	MaxItems int    `json:"max_items"`
}

type ExperienceResponse struct {
	Items []ExperienceItem `json:"items"`
}

type HookSink interface {
	Handle(ctx context.Context, event HookEvent) error
}

type PromptProvider interface {
	Build(ctx context.Context, req PromptRequest) (PromptBundle, error)
}

type PromptRequest struct {
	TaskID  string     `json:"task_id"`
	StepID  string     `json:"step_id,omitempty"`
	Purpose LLMPurpose `json:"purpose"`
}

type PromptBundle struct {
	SystemPrompt string       `json:"system_prompt"`
	Messages     []LLMMessage `json:"messages"`
}

type ToolPolicy interface {
	Evaluate(ctx context.Context, req ToolPolicyRequest) (ToolPolicyDecision, error)
}

type ToolPolicyRequest struct {
	TaskID    string         `json:"task_id"`
	StepID    string         `json:"step_id"`
	ToolName  string         `json:"tool_name"`
	Args      map[string]any `json:"args"`
	RiskLevel RiskLevel      `json:"risk_level"`
}

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	Generate() string
}
