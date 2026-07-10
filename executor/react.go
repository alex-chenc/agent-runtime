package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/alex-chenc/agent-runtime/contextbudget"
	"github.com/alex-chenc/agent-runtime/core"
	"github.com/alex-chenc/agent-runtime/internal/ids"
	"github.com/alex-chenc/agent-runtime/internal/limiter"
	"github.com/alex-chenc/agent-runtime/internal/textutil"
)

// ReActExecutor executes plan steps using the ReAct loop.
type ReActExecutor struct {
	llmClient            core.LLMClient
	toolGW               ToolCaller
	experienceProvider   core.ExperienceProvider
	idGen                core.IDGenerator
	provider             core.PromptProvider
	config               core.RuntimeConfig
	hookMgr              HookEmitter
	compressor           *contextbudget.Compressor
	toolResultCache      map[string]terminalToolResult
	toolResultCacheMu    sync.Mutex
	capabilityEvidence   map[string]core.ToolOutcome
	capabilityEvidenceMu sync.Mutex
}

// HookEmitter is the interface for emitting hook events from the executor.
type HookEmitter interface {
	EmitAsync(ctx context.Context, event core.HookEvent)
}

// ToolCaller is the interface for calling tools (simplified from ToolGateway).
type ToolCaller interface {
	CallValidated(ctx context.Context, req core.ToolRequest) (core.ToolResponse, error)
	Cancel(ctx context.Context, taskID string, callID string) error
}

// NewReActExecutor creates a new ReAct executor.
func NewReActExecutor(
	client core.LLMClient,
	tools ToolCaller,
	experienceProvider core.ExperienceProvider,
	idGen core.IDGenerator,
	provider core.PromptProvider,
	config core.RuntimeConfig,
	hookMgr HookEmitter,
	compressor *contextbudget.Compressor,
) *ReActExecutor {
	if idGen == nil {
		idGen = ids.Generator{}
	}
	return &ReActExecutor{
		llmClient:          client,
		toolGW:             tools,
		experienceProvider: experienceProvider,
		idGen:              idGen,
		provider:           provider,
		config:             config,
		hookMgr:            hookMgr,
		compressor:         compressor,
		toolResultCache:    make(map[string]terminalToolResult),
		capabilityEvidence: make(map[string]core.ToolOutcome),
	}
}

// StepResult contains the outcome of executing a single step.
type StepResult struct {
	StepID          string
	Status          core.StepStatus
	Result          string
	Evidence        []string
	Turns           []core.ReactTurn
	ToolCalls       []core.ToolCallRecord
	ExperienceUsage []core.ExperienceUsage
	Errors          []core.RuntimeError
	Error           *core.RuntimeError
	StartedAt       time.Time
	EndedAt         time.Time
}

// RunStep executes a single plan step using the ReAct loop.
func (e *ReActExecutor) RunStep(ctx context.Context, taskCtx *StepContext, step *core.PlanStep) *StepResult {
	result := &StepResult{
		StepID:    step.StepID,
		Status:    core.StepRunning,
		StartedAt: time.Now(),
	}

	limit := &limiter.Limiter{}
	var turns []core.ReactTurn
	failedToolSignatures := make(map[string]int)
	reasoningTurns := 0
	asyncPollStreak := 0
	var pendingAsyncPoll *asyncPollState

reactLoop:
	for turnIdx := 0; reasoningTurns < e.config.MaxStepReactTurns; turnIdx++ {
		if delay := asyncPollBackoff(e.config, asyncPollStreak); delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				result.Status = core.StepFailed
				result.Error = &core.RuntimeError{
					ErrorID:    e.idGen.Generate(),
					Kind:       core.ErrInterrupt,
					Stage:      "async_wait",
					TaskID:     taskCtx.TaskID,
					StepID:     step.StepID,
					Message:    "context cancelled while waiting for a non-terminal operation",
					OccurredAt: time.Now(),
				}
				result.Errors = append(result.Errors, *result.Error)
				break reactLoop
			case <-timer.C:
			}
		}

		// Check context
		if ctx.Err() != nil {
			result.Status = core.StepFailed
			result.Error = &core.RuntimeError{
				ErrorID:    e.idGen.Generate(),
				Kind:       core.ErrInterrupt,
				Stage:      "react",
				TaskID:     taskCtx.TaskID,
				StepID:     step.StepID,
				Message:    "context cancelled during step execution",
				OccurredAt: time.Now(),
			}
			result.Errors = append(result.Errors, *result.Error)
			break
		}

		turn := core.ReactTurn{
			TurnIndex: turnIdx,
			StartedAt: time.Now(),
		}

		var action core.ReactAction
		autoPolling := pendingAsyncPoll != nil
		if autoPolling {
			action = pendingAsyncPoll.Action
			action.Summary = "Poll the pending asynchronous operation."
		} else {
			// Call the model only when business reasoning is needed. Once the
			// model has selected an idempotent read-only status tool, Runtime
			// owns the repetitive polling until that operation becomes terminal.
			callID := e.idGen.Generate()
			turn.ModelCallID = callID

			resp, err := e.callLLM(ctx, taskCtx, step, turns, callID)
			if err != nil {
				parseErr := &core.RuntimeError{
					ErrorID:     e.idGen.Generate(),
					Kind:        core.ErrModelCall,
					Stage:       "react",
					TaskID:      taskCtx.TaskID,
					StepID:      step.StepID,
					ModelCallID: callID,
					Message:     fmt.Sprintf("LLM call failed: %v", err),
					Recoverable: true,
					OccurredAt:  time.Now(),
				}
				turn.ParseError = parseErr
				turn.EndedAt = time.Now()
				turns = append(turns, turn)
				result.Errors = append(result.Errors, *parseErr)
				limit.IncrParseFailures()
				if limit.ExceedsParseFailures(e.config.MaxParseFailures) {
					result.Status = core.StepFailed
					result.Error = parseErr
					break
				}
				reasoningTurns++
				asyncPollStreak = 0
				continue
			}

			// Parse action
			action, err = ParseAction(resp.Content)
			if err != nil {
				parseErr := &core.RuntimeError{
					ErrorID:     e.idGen.Generate(),
					Kind:        core.ErrModelParse,
					Stage:       "react",
					TaskID:      taskCtx.TaskID,
					StepID:      step.StepID,
					ModelCallID: callID,
					Message:     fmt.Sprintf("parse action: %v", err),
					Recoverable: true,
					OccurredAt:  time.Now(),
				}
				turn.ParseError = parseErr
				turn.EndedAt = time.Now()
				turns = append(turns, turn)
				result.Errors = append(result.Errors, *parseErr)
				limit.IncrParseFailures()
				if limit.ExceedsParseFailures(e.config.MaxParseFailures) {
					result.Status = core.StepFailed
					result.Error = parseErr
					break
				}
				reasoningTurns++
				asyncPollStreak = 0
				continue
			}

			limit.ResetParseFailures()
		}
		consumeReasoningTurn := true
		if action.Type == core.ActionToolCall {
			action.ToolArgs = mergeBoundToolArgs(action.ToolArgs, step.ToolArgs)
		}
		turn.Action = action

		switch action.Type {
		case core.ActionToolCall:
			if limit.ExceedsToolCalls(e.config.MaxToolCallsPerStep) {
				result.Status = core.StepFailed
				result.Error = &core.RuntimeError{
					ErrorID:    e.idGen.Generate(),
					Kind:       core.ErrToolCall,
					Stage:      "react",
					TaskID:     taskCtx.TaskID,
					StepID:     step.StepID,
					Message:    "max tool calls per step exceeded",
					OccurredAt: time.Now(),
				}
				result.Errors = append(result.Errors, *result.Error)
				turn.EndedAt = time.Now()
				turns = append(turns, turn)
				break
			}

			execution := toolExecutionOptions{}
			if autoPolling {
				pendingAsyncPoll.Attempt++
				execution = toolExecutionOptions{
					CallID:      pendingAsyncPoll.CallID,
					SilentHooks: true,
					Context: map[string]string{
						"agent_runtime_async_poll":   "true",
						"agent_runtime_poll_call_id": pendingAsyncPoll.CallID,
						"agent_runtime_poll_attempt": fmt.Sprintf("%d", pendingAsyncPoll.Attempt),
					},
				}
			}
			obs, record, toolErr := e.executeTool(ctx, taskCtx, step, action, execution)
			turn.Observation = obs
			turn.ToolCallID = obs.CallID
			result.ToolCalls = append(result.ToolCalls, record)
			if toolErr != nil {
				result.Errors = append(result.Errors, *toolErr)
				signature := action.ToolName + ":" + textutil.SummarizeJSON(action.ToolArgs, 2000)
				failedToolSignatures[signature]++
				if failedToolSignatures[signature] >= 2 {
					result.Status = core.StepFailed
					result.Error = &core.RuntimeError{
						ErrorID:     e.idGen.Generate(),
						Kind:        core.ErrToolCall,
						Stage:       "tool",
						TaskID:      taskCtx.TaskID,
						StepID:      step.StepID,
						ToolCallID:  obs.CallID,
						Message:     "repeated identical failed tool call; stopping step",
						Recoverable: false,
						OccurredAt:  time.Now(),
					}
					result.Errors = append(result.Errors, *result.Error)
				}
			}
			limit.IncrToolCalls()
			if obs.Status == core.ToolCallFailed {
				limit.IncrToolFailures()
			}
			if isNonTerminalAsyncObservation(obs) {
				consumeReasoningTurn = false
				asyncPollStreak++
				if pendingAsyncPoll == nil && e.canAutoPoll(action.ToolName) {
					pendingAsyncPoll = &asyncPollState{
						Action: action,
						CallID: obs.CallID,
					}
				}
			} else {
				asyncPollStreak = 0
				pendingAsyncPoll = nil
			}
			turn.EndedAt = time.Now()
			turns = append(turns, turn)

		case core.ActionStepResult:
			asyncPollStreak = 0
			pendingAsyncPoll = nil
			validation := validateStepCompletion(step, action, result.ToolCalls)
			if !validation.Passed {
				validationErr := core.RuntimeError{
					ErrorID:     e.idGen.Generate(),
					Kind:        core.ErrToolCall,
					Stage:       "completion_validation",
					TaskID:      taskCtx.TaskID,
					StepID:      step.StepID,
					Message:     validation.Reason,
					Recoverable: true,
					OccurredAt:  time.Now(),
				}
				result.Errors = append(result.Errors, validationErr)
				turn.ProgressSummary = "Step completion rejected: " + validation.Reason
			} else {
				result.Status = core.StepCompleted
				result.Result = action.StepResult
				result.Evidence = action.Evidence
			}
			turn.EndedAt = time.Now()
			turns = append(turns, turn)

		case core.ActionRequestExperience:
			asyncPollStreak = 0
			pendingAsyncPoll = nil
			usage, progress, expErr := e.fetchExperience(ctx, taskCtx, step, action)
			result.ExperienceUsage = append(result.ExperienceUsage, usage...)
			turn.ProgressSummary = progress
			if expErr != nil {
				result.Errors = append(result.Errors, *expErr)
			}
			turn.EndedAt = time.Now()
			turns = append(turns, turn)

		case core.ActionFailStep:
			asyncPollStreak = 0
			pendingAsyncPoll = nil
			result.Status = core.StepFailed
			result.Error = &core.RuntimeError{
				ErrorID:    e.idGen.Generate(),
				Kind:       core.ErrSystem,
				Stage:      "react",
				TaskID:     taskCtx.TaskID,
				StepID:     step.StepID,
				Message:    action.FailureReason,
				OccurredAt: time.Now(),
			}
			result.Errors = append(result.Errors, *result.Error)
			turn.EndedAt = time.Now()
			turns = append(turns, turn)

		default:
			asyncPollStreak = 0
			pendingAsyncPoll = nil
			// Other action types -- treat as no progress for MVP
			turn.EndedAt = time.Now()
			turns = append(turns, turn)
		}

		if consumeReasoningTurn {
			reasoningTurns++
		}
		if result.Status == core.StepCompleted || result.Status == core.StepFailed {
			break
		}
	}

	// If we exhausted all turns without completion
	if result.Status == core.StepRunning {
		result.Status = core.StepFailed
		result.Error = &core.RuntimeError{
			ErrorID:    e.idGen.Generate(),
			Kind:       core.ErrModelParse,
			Stage:      "react",
			TaskID:     taskCtx.TaskID,
			StepID:     step.StepID,
			Message:    "max ReAct turns exceeded without step completion",
			OccurredAt: time.Now(),
		}
		result.Errors = append(result.Errors, *result.Error)
	}

	result.Turns = turns
	result.EndedAt = time.Now()
	return result
}

func (e *ReActExecutor) callLLM(ctx context.Context, taskCtx *StepContext, step *core.PlanStep, prevTurns []core.ReactTurn, callID string) (core.LLMResponse, error) {
	messages := e.buildReactMessages(ctx, taskCtx, step, prevTurns)

	// Apply context compression if enabled
	if e.compressor != nil {
		compressed, records, err := e.compressor.Compress(messages, nil, step.StepID)
		if err == nil && len(records) > 0 {
			messages = compressed
			// Emit compression events via hook
			if e.hookMgr != nil {
				for _, rec := range records {
					e.hookMgr.EmitAsync(ctx, core.HookEvent{
						Type:      core.HookContextCompressed,
						TaskID:    taskCtx.TaskID,
						StepID:    step.StepID,
						CreatedAt: time.Now(),
						Payload: map[string]interface{}{
							"strategy":      string(rec.Strategy),
							"trigger_ratio": rec.TriggerRatio,
							"before_tokens": rec.BeforeTokens,
							"after_tokens":  rec.AfterTokens,
						},
					})
				}
			}
		}
	}

	// Emit budget snapshot after compression so the frontend sees updated usage
	if e.compressor != nil && e.hookMgr != nil {
		snap := e.compressor.GetBudgetSnapshot(messages)
		e.hookMgr.EmitAsync(ctx, core.HookEvent{
			Type:      core.HookContextBudgetChecked,
			TaskID:    taskCtx.TaskID,
			StepID:    step.StepID,
			CreatedAt: time.Now(),
			Snapshot: &core.TaskSnapshot{
				TaskID:        taskCtx.TaskID,
				ContextBudget: &snap,
			},
		})
	}

	timeout := e.config.ModelTimeout
	if dl, ok := ctx.Deadline(); ok {
		remaining := time.Until(dl)
		if remaining < timeout {
			timeout = remaining
		}
	}

	return e.llmClient.Complete(ctx, core.LLMRequest{
		CallID:         callID,
		TaskID:         taskCtx.TaskID,
		StepID:         step.StepID,
		Purpose:        core.PurposeReact,
		Messages:       messages,
		ResponseSchema: "react_action",
		ResponseFormat: e.reactResponseFormat(step),
		Timeout:        timeout,
	})
}

func (e *ReActExecutor) buildReactMessages(ctx context.Context, taskCtx *StepContext, step *core.PlanStep, prevTurns []core.ReactTurn) []core.LLMMessage {
	var system string
	if e.provider != nil {
		bundle, err := e.provider.Build(ctx, core.PromptRequest{
			TaskID:  taskCtx.TaskID,
			StepID:  step.StepID,
			Purpose: core.PurposeReact,
		})
		if err == nil && bundle.SystemPrompt != "" {
			system = bundle.SystemPrompt
		}
	}
	if system == "" {
		system = fmt.Sprintf(`You are an AI agent executing a step in a plan.
You must respond with a JSON object:
{
  "action": "tool_call|step_result|request_experience|fail_step",
  "summary": "brief description of what you're doing",
  "tool_call": {"tool_name": "...", "reason": "...", "args": {...}},
  "step_result": {"result": "...", "evidence": ["..."], "confidence": "low|medium|high"},
  "experience_request": {"query": "...", "reason": "..."},
  "failure": {"reason": "...", "recoverable": true}
}

Completion rules:
- A successful tool call can still be non-terminal. operation_status=accepted or running never proves that the business operation completed.
- When a step uses tools, cite the exact successful terminal call_id values in step_result.evidence.
- After a tool failure, retry with corrected arguments, use an allowed alternative, request input, or fail the step. Never report a failed call as completed.`)
	}

	// Append step context
	system += fmt.Sprintf(`

## Current Step
- Title: %s
- Objective: %s
- Expected output: %s`, step.Title, step.Objective, step.ExpectedOutput)
	if len(step.AllowedTools) > 0 {
		system += fmt.Sprintf("\n- Allowed tools for this step (strict): %s", strings.Join(step.AllowedTools, ", "))
	}
	if len(step.ToolArgs) > 0 {
		system += fmt.Sprintf("\n- Caller-bound arguments (authoritative): %s", textutil.SummarizeJSON(step.ToolArgs, 2000))
	}

	messages := []core.LLMMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: fmt.Sprintf("Task: %s", taskCtx.UserInput)},
	}
	if len(taskCtx.PreviousSteps) > 0 {
		var previous strings.Builder
		previous.WriteString("Completed prior steps and evidence:")
		for _, prior := range taskCtx.PreviousSteps {
			previous.WriteString(fmt.Sprintf("\n- %s [%s]", prior.StepID, prior.Status))
			if prior.Result != "" {
				previous.WriteString(": ")
				previous.WriteString(textutil.Truncate(prior.Result, 1500))
			}
			for _, turn := range prior.ReactTurns {
				if turn.Observation == nil {
					continue
				}
				previous.WriteString("\n")
				previous.WriteString(formatObservationForModel(turn.Observation))
			}
		}
		messages = append(messages, core.LLMMessage{Role: "user", Content: previous.String()})
	}

	// Add previous turns
	for _, turn := range prevTurns {
		if turn.Action.Summary != "" {
			actionJSON := textutil.SummarizeJSON(turn.Action, 500)
			messages = append(messages, core.LLMMessage{
				Role:    "assistant",
				Content: actionJSON,
			})
		}
		if turn.Observation != nil {
			messages = append(messages, core.LLMMessage{
				Role:    "user",
				Content: formatObservationForModel(turn.Observation),
			})
		}
		if turn.ProgressSummary != "" {
			messages = append(messages, core.LLMMessage{
				Role:    "user",
				Content: turn.ProgressSummary,
			})
		}
	}

	return messages
}

func (e *ReActExecutor) fetchExperience(ctx context.Context, taskCtx *StepContext, step *core.PlanStep, action core.ReactAction) ([]core.ExperienceUsage, string, *core.RuntimeError) {
	if e.experienceProvider == nil {
		err := &core.RuntimeError{
			ErrorID:     e.idGen.Generate(),
			Kind:        core.ErrExperience,
			Stage:       "react",
			TaskID:      taskCtx.TaskID,
			StepID:      step.StepID,
			Message:     "experience provider is not configured",
			Recoverable: true,
			OccurredAt:  time.Now(),
		}
		return nil, "Experience request could not be fulfilled: provider is not configured.", err
	}

	resp, err := e.experienceProvider.Fetch(ctx, core.ExperienceRequest{
		TaskID:   taskCtx.TaskID,
		Query:    action.ExperienceQuery,
		MaxItems: 3,
	})
	if err != nil {
		runtimeErr := &core.RuntimeError{
			ErrorID:     e.idGen.Generate(),
			Kind:        core.ErrExperience,
			Stage:       "react",
			TaskID:      taskCtx.TaskID,
			StepID:      step.StepID,
			Message:     err.Error(),
			Recoverable: true,
			OccurredAt:  time.Now(),
		}
		return nil, "Experience request failed: " + err.Error(), runtimeErr
	}

	usage := make([]core.ExperienceUsage, 0, len(resp.Items))
	var summary strings.Builder
	for _, item := range resp.Items {
		usage = append(usage, core.ExperienceUsage{
			ItemID:    item.ID,
			UsedAt:    "react",
			Helpful:   true,
			Timestamp: time.Now(),
		})
		if item.Summary == "" && item.Content == "" {
			continue
		}
		if summary.Len() > 0 {
			summary.WriteString("\n")
		}
		if item.ID != "" {
			summary.WriteString("- ")
			summary.WriteString(item.ID)
			summary.WriteString(": ")
		} else {
			summary.WriteString("- ")
		}
		if item.Summary != "" {
			summary.WriteString(item.Summary)
		} else {
			summary.WriteString(item.Content)
		}
	}
	if summary.Len() == 0 {
		return usage, "Experience request returned no relevant items.", nil
	}
	return usage, "Relevant experience:\n" + textutil.Truncate(summary.String(), 2000), nil
}

type asyncPollState struct {
	Action  core.ReactAction
	CallID  string
	Attempt int
}

type toolExecutionOptions struct {
	CallID      string
	SilentHooks bool
	Context     map[string]string
}

// terminalToolResult preserves evidence for a completed non-idempotent call
// within this Runtime instance. A model can revisit a later plan step and
// propose the same write again; replaying it would create duplicate tasks or
// side effects even though the first terminal result is already available.
type terminalToolResult struct {
	Response core.ToolResponse
	Record   core.ToolCallRecord
}

func (e *ReActExecutor) executeTool(
	ctx context.Context,
	taskCtx *StepContext,
	step *core.PlanStep,
	action core.ReactAction,
	options toolExecutionOptions,
) (*core.Observation, core.ToolCallRecord, *core.RuntimeError) {
	callID := options.CallID
	if callID == "" {
		callID = e.idGen.Generate()
	}
	if cached, ok := e.cachedTerminalToolResult(taskCtx.TaskID, action, options); ok {
		return cachedObservation(cached), cached.Record, nil
	}
	start := time.Now()
	record := core.ToolCallRecord{
		CallID:      callID,
		TaskID:      taskCtx.TaskID,
		StepID:      step.StepID,
		ToolName:    action.ToolName,
		Reason:      action.Summary,
		ArgsSummary: textutil.SummarizeJSON(action.ToolArgs, 1000),
		RiskLevel:   step.RiskLevel,
		StartedAt:   start,
	}
	if prerequisiteErr := e.validateToolPrerequisites(action.ToolName); prerequisiteErr != "" {
		record.Status = core.ToolCallFailed
		record.ErrorMessage = prerequisiteErr
		record.ValidationStage = string(core.ToolValidationPolicy)
		record.EndedAt = time.Now()
		obs := &core.Observation{
			ToolName: action.ToolName,
			CallID:   callID,
			Status:   core.ToolCallFailed,
			Error:    prerequisiteErr,
			Summary:  prerequisiteErr,
			Duration: time.Since(start),
		}
		return obs, record, &core.RuntimeError{
			ErrorID:     e.idGen.Generate(),
			Kind:        core.ErrToolPolicyDenied,
			Stage:       "tool_prerequisite",
			TaskID:      taskCtx.TaskID,
			StepID:      step.StepID,
			ToolCallID:  callID,
			Message:     prerequisiteErr,
			Recoverable: true,
			OccurredAt:  start,
		}
	}

	// Emit HookToolCallStarted
	if e.hookMgr != nil && !options.SilentHooks {
		e.hookMgr.EmitAsync(ctx, core.HookEvent{
			TaskID: taskCtx.TaskID,
			StepID: step.StepID,
			Type:   core.HookToolCallStarted,
			Payload: map[string]interface{}{
				"tool_name":    action.ToolName,
				"call_id":      callID,
				"args_summary": textutil.SummarizeJSON(action.ToolArgs, 1000),
				"reason":       action.Summary,
			},
		})
	}

	if e.toolGW == nil {
		errMsg := "tool gateway is not configured"
		obs := &core.Observation{
			ToolName: action.ToolName,
			CallID:   callID,
			Status:   core.ToolCallFailed,
			Error:    errMsg,
			Summary:  fmt.Sprintf("Tool %s failed: %s", action.ToolName, errMsg),
			Duration: time.Since(start),
		}
		record.Status = core.ToolCallFailed
		record.ErrorMessage = errMsg
		record.EndedAt = time.Now()
		// Emit HookToolCallFinished for gateway-not-configured error
		if e.hookMgr != nil && !options.SilentHooks {
			e.hookMgr.EmitAsync(ctx, core.HookEvent{
				TaskID: taskCtx.TaskID,
				StepID: step.StepID,
				Type:   core.HookToolCallFinished,
				Payload: map[string]interface{}{
					"call_id":       callID,
					"tool_name":     action.ToolName,
					"status":        string(core.ToolCallFailed),
					"error_message": errMsg,
					"duration_ms":   time.Since(start).Milliseconds(),
				},
			})
		}
		return obs, record, &core.RuntimeError{
			ErrorID:     e.idGen.Generate(),
			Kind:        core.ErrToolCall,
			Stage:       "tool",
			TaskID:      taskCtx.TaskID,
			StepID:      step.StepID,
			ToolCallID:  callID,
			Message:     errMsg,
			Recoverable: true,
			OccurredAt:  start,
		}
	}

	if len(step.AllowedTools) > 0 && !containsTool(step.AllowedTools, action.ToolName) {
		validationErr := &core.ToolCallValidationError{
			Stage:    core.ToolValidationStepScope,
			ToolName: action.ToolName,
			Message:  fmt.Sprintf("tool is outside step allowlist: %s", strings.Join(step.AllowedTools, ", ")),
		}
		record.Status = core.ToolCallFailed
		record.ErrorMessage = validationErr.Error()
		record.ValidationStage = string(validationErr.Stage)
		record.EndedAt = time.Now()
		obs := &core.Observation{
			ToolName: action.ToolName,
			CallID:   callID,
			Status:   core.ToolCallFailed,
			Error:    validationErr.Error(),
			Summary:  validationErr.Error(),
			Duration: time.Since(start),
		}
		if e.hookMgr != nil && !options.SilentHooks {
			e.hookMgr.EmitAsync(ctx, core.HookEvent{
				TaskID: taskCtx.TaskID,
				StepID: step.StepID,
				Type:   core.HookToolCallFinished,
				Payload: map[string]interface{}{
					"call_id":          callID,
					"tool_name":        action.ToolName,
					"status":           string(core.ToolCallFailed),
					"error_message":    validationErr.Error(),
					"validation_stage": string(validationErr.Stage),
					"duration_ms":      time.Since(start).Milliseconds(),
				},
			})
		}
		return obs, record, &core.RuntimeError{
			ErrorID:     e.idGen.Generate(),
			Kind:        core.ErrToolCall,
			Stage:       "tool",
			TaskID:      taskCtx.TaskID,
			StepID:      step.StepID,
			ToolCallID:  callID,
			Message:     validationErr.Error(),
			Recoverable: true,
			OccurredAt:  start,
		}
	}

	resp, err := e.toolGW.CallValidated(ctx, core.ToolRequest{
		CallID:   callID,
		TaskID:   taskCtx.TaskID,
		StepID:   step.StepID,
		ToolName: action.ToolName,
		Reason:   action.Summary,
		Args:     action.ToolArgs,
		Timeout:  e.config.ToolTimeout,
		Context:  options.Context,
	})

	obs := &core.Observation{
		ToolName: action.ToolName,
		CallID:   callID,
		Duration: time.Since(start),
	}

	if err != nil {
		validationStage := validationStageFromError(err)
		record.Status = core.ToolCallFailed
		record.ErrorMessage = err.Error()
		record.ValidationStage = validationStage
		record.EndedAt = time.Now()
		obs.Status = core.ToolCallFailed
		obs.Error = err.Error()
		obs.Summary = fmt.Sprintf("Tool %s failed: %v", action.ToolName, err)
		// Emit HookToolCallFinished for CallValidated error
		if e.hookMgr != nil && !options.SilentHooks {
			e.hookMgr.EmitAsync(ctx, core.HookEvent{
				TaskID: taskCtx.TaskID,
				StepID: step.StepID,
				Type:   core.HookToolCallFinished,
				Payload: map[string]interface{}{
					"call_id":          callID,
					"tool_name":        action.ToolName,
					"status":           string(core.ToolCallFailed),
					"error_message":    err.Error(),
					"validation_stage": validationStage,
					"duration_ms":      time.Since(start).Milliseconds(),
				},
			})
		}
		return obs, record, &core.RuntimeError{
			ErrorID:     e.idGen.Generate(),
			Kind:        core.ErrToolCall,
			Stage:       "tool",
			TaskID:      taskCtx.TaskID,
			StepID:      step.StepID,
			ToolCallID:  callID,
			Message:     err.Error(),
			Recoverable: true,
			OccurredAt:  start,
		}
	}

	obs.Status = resp.Status
	obs.Content = resp.Content
	obs.Summary = resp.Summary
	obs.Outcome = resp.Outcome
	if resp.ErrorMessage != "" {
		obs.Error = resp.ErrorMessage
	}
	record.Status = resp.Status
	if record.Status == "" {
		record.Status = core.ToolCallSuccess
		obs.Status = core.ToolCallSuccess
	}
	record.ResultSummary = resp.Summary
	record.ErrorMessage = resp.ErrorMessage
	record.Outcome = resp.Outcome
	e.rememberCapabilityEvidence(resp.Outcome)
	if !resp.StartedAt.IsZero() {
		record.StartedAt = resp.StartedAt
	}
	if resp.EndedAt.IsZero() {
		record.EndedAt = time.Now()
	} else {
		record.EndedAt = resp.EndedAt
	}
	if record.Status == core.ToolCallFailed || record.Status == core.ToolCallTimeout || record.Status == core.ToolCallCancelled {
		kind := core.ErrToolCall
		if record.Status == core.ToolCallTimeout {
			kind = core.ErrToolTimeout
		}
		// Emit HookToolCallFinished for failed tool calls
		if e.hookMgr != nil && !options.SilentHooks {
			e.hookMgr.EmitAsync(ctx, core.HookEvent{
				TaskID: taskCtx.TaskID,
				StepID: step.StepID,
				Type:   core.HookToolCallFinished,
				Payload: map[string]interface{}{
					"call_id":        callID,
					"tool_name":      action.ToolName,
					"status":         string(record.Status),
					"error_message":  firstNonEmpty(resp.ErrorMessage, resp.Summary),
					"result_summary": resp.Summary,
					"duration_ms":    time.Since(start).Milliseconds(),
				},
			})
		}
		return obs, record, &core.RuntimeError{
			ErrorID:     e.idGen.Generate(),
			Kind:        kind,
			Stage:       "tool",
			TaskID:      taskCtx.TaskID,
			StepID:      step.StepID,
			ToolCallID:  callID,
			Message:     firstNonEmpty(resp.ErrorMessage, resp.Summary),
			Recoverable: true,
			OccurredAt:  start,
		}
	}

	// Emit HookToolCallFinished for successful tool calls
	if e.hookMgr != nil && !options.SilentHooks {
		e.hookMgr.EmitAsync(ctx, core.HookEvent{
			TaskID: taskCtx.TaskID,
			StepID: step.StepID,
			Type:   core.HookToolCallFinished,
			Payload: map[string]interface{}{
				"call_id":          callID,
				"tool_name":        action.ToolName,
				"status":           string(record.Status),
				"result_summary":   resp.Summary,
				"operation_status": operationStatus(resp.Outcome),
				"terminal":         operationTerminal(resp.Outcome),
				"duration_ms":      time.Since(start).Milliseconds(),
			},
		})
	}
	e.storeTerminalToolResult(taskCtx.TaskID, action, options, resp, record)

	return obs, record, nil
}

func (e *ReActExecutor) validateToolPrerequisites(toolName string) string {
	descriptor, ok := e.toolDescriptor(toolName)
	if !ok || len(descriptor.Prerequisites) == 0 {
		return ""
	}
	e.capabilityEvidenceMu.Lock()
	defer e.capabilityEvidenceMu.Unlock()
	for _, prerequisite := range descriptor.Prerequisites {
		capability := strings.TrimSpace(prerequisite.Capability)
		outcome, found := e.capabilityEvidence[capability]
		switch prerequisite.Condition {
		case core.PrerequisiteCapabilityEmptyResult:
			if !found || !outcome.Terminal || outcome.OperationStatus != core.OperationSucceeded || len(outcome.Facts) != 0 {
				return fmt.Sprintf("tool prerequisite not met: %s requires a terminal empty result from capability %s", prerequisite.Condition, capability)
			}
		}
	}
	return ""
}

func (e *ReActExecutor) rememberCapabilityEvidence(outcome *core.ToolOutcome) {
	if e == nil || outcome == nil || strings.TrimSpace(outcome.Capability) == "" || !outcome.Terminal || outcome.OperationStatus != core.OperationSucceeded {
		return
	}
	e.capabilityEvidenceMu.Lock()
	e.capabilityEvidence[outcome.Capability] = *outcome
	e.capabilityEvidenceMu.Unlock()
}

func (e *ReActExecutor) toolDescriptor(toolName string) (core.ToolDescriptor, bool) {
	provider, ok := e.toolGW.(toolDescriptorProvider)
	if !ok {
		return core.ToolDescriptor{}, false
	}
	for _, descriptor := range provider.ToolDescriptors() {
		if descriptor.Name == toolName {
			return descriptor, true
		}
	}
	return core.ToolDescriptor{}, false
}

func (e *ReActExecutor) cachedTerminalToolResult(taskID string, action core.ReactAction, options toolExecutionOptions) (terminalToolResult, bool) {
	if e == nil || !e.shouldMemoizeTool(action.ToolName) || options.CallID != "" {
		return terminalToolResult{}, false
	}
	key, ok := toolResultCacheKey(taskID, action.ToolName, action.ToolArgs)
	if !ok {
		return terminalToolResult{}, false
	}
	e.toolResultCacheMu.Lock()
	defer e.toolResultCacheMu.Unlock()
	cached, ok := e.toolResultCache[key]
	return cached, ok
}

func (e *ReActExecutor) storeTerminalToolResult(taskID string, action core.ReactAction, options toolExecutionOptions, response core.ToolResponse, record core.ToolCallRecord) {
	if e == nil || !e.shouldMemoizeTool(action.ToolName) || options.CallID != "" || !terminalSuccessfulResponse(response) {
		return
	}
	key, ok := toolResultCacheKey(taskID, action.ToolName, action.ToolArgs)
	if !ok {
		return
	}
	e.toolResultCacheMu.Lock()
	e.toolResultCache[key] = terminalToolResult{Response: response, Record: record}
	e.toolResultCacheMu.Unlock()
}

func (e *ReActExecutor) shouldMemoizeTool(toolName string) bool {
	descriptor, ok := e.toolDescriptor(toolName)
	return ok && !descriptor.Idempotent
}

func toolResultCacheKey(taskID, toolName string, args map[string]any) (string, bool) {
	encodedArgs, err := json.Marshal(args)
	if err != nil {
		return "", false
	}
	return taskID + "\x00" + toolName + "\x00" + string(encodedArgs), true
}

func terminalSuccessfulResponse(response core.ToolResponse) bool {
	if response.Status != "" && response.Status != core.ToolCallSuccess {
		return false
	}
	if response.Outcome == nil {
		return true
	}
	return response.Outcome.Terminal && (response.Outcome.OperationStatus == core.OperationSucceeded || response.Outcome.OperationStatus == core.OperationSkipped)
}

func cachedObservation(cached terminalToolResult) *core.Observation {
	record := cached.Record
	response := cached.Response
	return &core.Observation{
		ToolName: response.ToolName,
		CallID:   record.CallID,
		Status:   record.Status,
		Content:  response.Content,
		Summary:  response.Summary,
		Outcome:  response.Outcome,
		Duration: record.EndedAt.Sub(record.StartedAt),
	}
}

func formatObservationForModel(observation *core.Observation) string {
	if observation == nil {
		return "Tool observation: unavailable"
	}
	var builder strings.Builder
	builder.WriteString("Tool observation:")
	builder.WriteString("\n- tool_name: ")
	builder.WriteString(observation.ToolName)
	builder.WriteString("\n- call_id: ")
	builder.WriteString(observation.CallID)
	builder.WriteString("\n- call_status: ")
	builder.WriteString(string(observation.Status))
	if observation.Outcome != nil {
		builder.WriteString("\n- operation_status: ")
		builder.WriteString(string(observation.Outcome.OperationStatus))
		builder.WriteString("\n- terminal: ")
		builder.WriteString(fmt.Sprintf("%t", observation.Outcome.Terminal))
		builder.WriteString("\n- outcome: ")
		builder.WriteString(textutil.SummarizeJSON(observation.Outcome, 2000))
	}
	if observation.Error != "" {
		builder.WriteString("\n- error: ")
		builder.WriteString(textutil.Truncate(observation.Error, 2000))
	}
	if observation.Summary != "" {
		builder.WriteString("\n- summary: ")
		builder.WriteString(textutil.Truncate(observation.Summary, 1000))
	}
	if observation.Content != "" {
		builder.WriteString("\n- content: ")
		builder.WriteString(textutil.Truncate(observation.Content, 2000))
	}
	return builder.String()
}

func isNonTerminalAsyncObservation(observation *core.Observation) bool {
	if observation == nil ||
		observation.Status != core.ToolCallSuccess ||
		observation.Outcome == nil ||
		observation.Outcome.Terminal {
		return false
	}
	return observation.Outcome.OperationStatus == core.OperationAccepted ||
		observation.Outcome.OperationStatus == core.OperationRunning
}

func asyncPollBackoff(config core.RuntimeConfig, streak int) time.Duration {
	if streak <= 0 || config.AsyncPollInitialBackoff <= 0 {
		return 0
	}
	delay := config.AsyncPollInitialBackoff
	const maxDuration = time.Duration(1<<63 - 1)
	for count := 1; count < streak; count++ {
		if config.AsyncPollMaxBackoff > 0 && delay >= config.AsyncPollMaxBackoff {
			return config.AsyncPollMaxBackoff
		}
		if delay > maxDuration/2 {
			delay = maxDuration
			break
		}
		delay *= 2
	}
	if config.AsyncPollMaxBackoff > 0 && delay > config.AsyncPollMaxBackoff {
		return config.AsyncPollMaxBackoff
	}
	return delay
}

func (e *ReActExecutor) canAutoPoll(toolName string) bool {
	provider, ok := e.toolGW.(toolDescriptorProvider)
	if !ok {
		return false
	}
	for _, descriptor := range provider.ToolDescriptors() {
		if descriptor.Name == toolName {
			return descriptor.RiskLevel == core.RiskReadOnly && descriptor.Idempotent
		}
	}
	return false
}

func operationStatus(outcome *core.ToolOutcome) string {
	if outcome == nil {
		return ""
	}
	return string(outcome.OperationStatus)
}

func operationTerminal(outcome *core.ToolOutcome) bool {
	return outcome != nil && outcome.Terminal
}

func mergeBoundToolArgs(modelArgs, boundArgs map[string]any) map[string]any {
	result := make(map[string]any, len(modelArgs)+len(boundArgs))
	for key, value := range modelArgs {
		result[key] = value
	}
	for key, value := range boundArgs {
		result[key] = value
	}
	return result
}

func containsTool(allowed []string, toolName string) bool {
	for _, name := range allowed {
		if name == toolName {
			return true
		}
	}
	return false
}

func validationStageFromError(err error) string {
	var validationErr *core.ToolCallValidationError
	if errors.As(err, &validationErr) {
		return string(validationErr.Stage)
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return "tool call failed"
}
