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
	HookTaskStarted        HookEventType = "task_started"
	HookExperienceLoaded   HookEventType = "experience_loaded"
	HookPlanCreated        HookEventType = "plan_created"
	HookStepStarted        HookEventType = "step_started"
	HookModelCallStarted   HookEventType = "model_call_started"
	HookModelCallFinished  HookEventType = "model_call_finished"
	HookToolCallStarted    HookEventType = "tool_call_started"
	HookToolCallFinished   HookEventType = "tool_call_finished"
	HookStepCompleted      HookEventType = "step_completed"
	HookStepFailed         HookEventType = "step_failed"
	HookAuditStarted       HookEventType = "audit_started"
	HookAuditFinished      HookEventType = "audit_finished"
	HookReflectionStarted  HookEventType = "reflection_started"
	HookReflectionFinished HookEventType = "reflection_finished"
	HookCorrectionApplied  HookEventType = "correction_applied"
	HookStepRetrying       HookEventType = "step_retrying"
	HookStepSkipped        HookEventType = "step_skipped"
	HookConfigChanged      HookEventType = "config_changed"
	HookTaskInterrupted    HookEventType = "task_interrupted"
	HookTaskFinished       HookEventType = "task_finished"
)

type LLMPurpose string

const (
	PurposePlan      LLMPurpose = "plan"
	PurposeReact     LLMPurpose = "react"
	PurposeAudit     LLMPurpose = "audit"
	PurposeReflect   LLMPurpose = "reflect"
	PurposeCorrect   LLMPurpose = "correct"
	PurposeSummarize LLMPurpose = "summarize"
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
	Assumptions []string   `json:"assumptions,omitempty"`
	Steps       []PlanStep `json:"steps"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type PlanStep struct {
	StepID         string     `json:"step_id"`
	Title          string     `json:"title"`
	Objective      string     `json:"objective"`
	ExpectedOutput string     `json:"expected_output"`
	SuggestedTools []string   `json:"suggested_tools,omitempty"`
	Dependencies   []string   `json:"dependencies,omitempty"`
	Status         StepStatus `json:"status"`
	RetryCount     int        `json:"retry_count"`
	CreatedBy      string     `json:"created_by"`
	ChangeReason   string     `json:"change_reason,omitempty"`
	RiskLevel      RiskLevel  `json:"risk_level"`
}

type TaskSnapshot struct {
	TaskID          string            `json:"task_id"`
	UserInput       string            `json:"user_input"`
	Status          TaskStatus        `json:"status"`
	ExitReason      ExitReason        `json:"exit_reason"`
	CurrentPlan     *Plan             `json:"current_plan,omitempty"`
	CurrentStepID   string            `json:"current_step_id,omitempty"`
	CompletedSteps  int               `json:"completed_steps"`
	FailedSteps     int               `json:"failed_steps"`
	TotalToolCalls  int               `json:"total_tool_calls"`
	TotalModelCalls int               `json:"total_model_calls"`
	RecentErrors    []RuntimeError    `json:"recent_errors,omitempty"`
	StartedAt       time.Time         `json:"started_at"`
	Metadata        map[string]string `json:"metadata,omitempty"`
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
	CallID        string        `json:"call_id"`
	TaskID        string        `json:"task_id"`
	StepID        string        `json:"step_id,omitempty"`
	Purpose       LLMPurpose    `json:"purpose"`
	InputSummary  string        `json:"input_summary"`
	OutputSummary string        `json:"output_summary"`
	Schema        string        `json:"schema,omitempty"`
	Model         string        `json:"model,omitempty"`
	TokensUsed    int           `json:"tokens_used,omitempty"`
	Latency       time.Duration `json:"latency"`
	Error         string        `json:"error,omitempty"`
	OccurredAt    time.Time     `json:"occurred_at"`
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
	CallID        string         `json:"call_id"`
	TaskID        string         `json:"task_id"`
	StepID        string         `json:"step_id"`
	ToolName      string         `json:"tool_name"`
	Reason        string         `json:"reason"`
	ArgsSummary   string         `json:"args_summary"`
	Status        ToolCallStatus `json:"status"`
	ResultSummary string         `json:"result_summary"`
	ErrorMessage  string         `json:"error_message,omitempty"`
	RiskLevel     RiskLevel      `json:"risk_level"`
	StartedAt     time.Time      `json:"started_at"`
	EndedAt       time.Time      `json:"ended_at"`
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

type RuntimeMetrics struct {
	TotalTurns         int           `json:"total_turns"`
	TotalToolCalls     int           `json:"total_tool_calls"`
	TotalModelCalls    int           `json:"total_model_calls"`
	TotalToolFailures  int           `json:"total_tool_failures"`
	TotalModelFailures int           `json:"total_model_failures"`
	TotalParseFailures int           `json:"total_parse_failures"`
	TotalAudits        int           `json:"total_audits"`
	TotalReflections   int           `json:"total_reflections"`
	TotalCorrections   int           `json:"total_corrections"`
	TotalDuration      time.Duration `json:"total_duration"`
	ModelCallDuration  time.Duration `json:"model_call_duration"`
	ToolCallDuration   time.Duration `json:"tool_call_duration"`
}

type TaskResult struct {
	TaskID          string             `json:"task_id"`
	Status          TaskStatus         `json:"status"`
	ExitReason      ExitReason         `json:"exit_reason"`
	FinalAnswer     string             `json:"final_answer"`
	Completion      CompletionSummary  `json:"completion"`
	InitialPlan     *Plan              `json:"initial_plan,omitempty"`
	FinalPlan       *Plan              `json:"final_plan,omitempty"`
	StepExecutions  []StepExecution    `json:"step_executions"`
	ToolCalls       []ToolCallRecord   `json:"tool_calls"`
	ModelCalls      []ModelCallRecord  `json:"model_calls"`
	Errors          []RuntimeError     `json:"errors"`
	Reflections     []ReflectionResult `json:"reflections"`
	Audits          []AuditResult      `json:"audits"`
	Corrections     []CorrectionResult `json:"corrections"`
	ExperienceUsage []ExperienceUsage  `json:"experience_usage,omitempty"`
	ConfigChanges   []ConfigChange     `json:"config_changes,omitempty"`
	Metrics         RuntimeMetrics     `json:"metrics"`
	StartedAt       time.Time          `json:"started_at"`
	EndedAt         time.Time          `json:"ended_at"`
	Metadata        map[string]string  `json:"metadata,omitempty"`
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
	return nil
}

type ConfigPatch struct {
	MaxTotalTurns       *int           `json:"max_total_turns,omitempty"`
	MaxPlanSteps        *int           `json:"max_plan_steps,omitempty"`
	MaxStepReactTurns   *int           `json:"max_step_react_turns,omitempty"`
	MaxToolCalls        *int           `json:"max_tool_calls,omitempty"`
	MaxToolCallsPerStep *int           `json:"max_tool_calls_per_step,omitempty"`
	TaskTimeout         *time.Duration `json:"task_timeout,omitempty"`
	ModelTimeout        *time.Duration `json:"model_timeout,omitempty"`
	ToolTimeout         *time.Duration `json:"tool_timeout,omitempty"`
	EnableReflection    *bool          `json:"enable_reflection,omitempty"`
	EnableAudit         *bool          `json:"enable_audit,omitempty"`
	EnableCorrection    *bool          `json:"enable_correction,omitempty"`
	MaxStepRetries      *int           `json:"max_step_retries,omitempty"`
	DisabledTools       []string       `json:"disabled_tools,omitempty"`
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
