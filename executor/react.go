package executor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chenchen511/agent-runtime/contextbudget"
	"github.com/chenchen511/agent-runtime/core"
	"github.com/chenchen511/agent-runtime/internal/ids"
	"github.com/chenchen511/agent-runtime/internal/limiter"
	"github.com/chenchen511/agent-runtime/internal/textutil"
)

// ReActExecutor executes plan steps using the ReAct loop.
type ReActExecutor struct {
	llmClient          core.LLMClient
	toolGW             ToolCaller
	experienceProvider core.ExperienceProvider
	idGen              core.IDGenerator
	provider           core.PromptProvider
	config             core.RuntimeConfig
	hookMgr            HookEmitter
	compressor         *contextbudget.Compressor
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

	for turnIdx := 0; turnIdx < e.config.MaxStepReactTurns; turnIdx++ {
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

		// Call LLM
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
			continue
		}

		// Parse action
		action, err := ParseAction(resp.Content)
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
			continue
		}

		limit.ResetParseFailures()
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

			obs, record, toolErr := e.executeTool(ctx, taskCtx, step, action)
			turn.Observation = obs
			turn.ToolCallID = obs.CallID
			result.ToolCalls = append(result.ToolCalls, record)
			if toolErr != nil {
				result.Errors = append(result.Errors, *toolErr)
			}
			limit.IncrToolCalls()
			if obs.Status == core.ToolCallFailed {
				limit.IncrToolFailures()
			}
			turn.EndedAt = time.Now()
			turns = append(turns, turn)

		case core.ActionStepResult:
			result.Status = core.StepCompleted
			result.Result = action.StepResult
			result.Evidence = action.Evidence
			turn.EndedAt = time.Now()
			turns = append(turns, turn)

		case core.ActionRequestExperience:
			usage, progress, expErr := e.fetchExperience(ctx, taskCtx, step, action)
			result.ExperienceUsage = append(result.ExperienceUsage, usage...)
			turn.ProgressSummary = progress
			if expErr != nil {
				result.Errors = append(result.Errors, *expErr)
			}
			turn.EndedAt = time.Now()
			turns = append(turns, turn)

		case core.ActionFailStep:
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
			// Other action types -- treat as no progress for MVP
			turn.EndedAt = time.Now()
			turns = append(turns, turn)
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

	timeout := e.config.ModelTimeout
	if dl, ok := ctx.Deadline(); ok {
		remaining := time.Until(dl)
		if remaining < timeout {
			timeout = remaining
		}
	}

	return e.llmClient.Complete(ctx, core.LLMRequest{
		CallID:   callID,
		TaskID:   taskCtx.TaskID,
		StepID:   step.StepID,
		Purpose:  core.PurposeReact,
		Messages: messages,
		Timeout:  timeout,
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
}`)
	}

	// Append step context
	system += fmt.Sprintf(`

## Current Step
- Title: %s
- Objective: %s
- Expected output: %s`, step.Title, step.Objective, step.ExpectedOutput)

	messages := []core.LLMMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: fmt.Sprintf("Task: %s", taskCtx.UserInput)},
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
			obsContent := textutil.Truncate(turn.Observation.Content, 2000)
			messages = append(messages, core.LLMMessage{
				Role:    "user",
				Content: fmt.Sprintf("Observation from %s: %s", turn.Observation.ToolName, obsContent),
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

func (e *ReActExecutor) executeTool(ctx context.Context, taskCtx *StepContext, step *core.PlanStep, action core.ReactAction) (*core.Observation, core.ToolCallRecord, *core.RuntimeError) {
	callID := e.idGen.Generate()
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

	// Emit HookToolCallStarted
	if e.hookMgr != nil {
		e.hookMgr.EmitAsync(ctx, core.HookEvent{
			TaskID: taskCtx.TaskID,
			StepID: step.StepID,
			Type:   core.HookToolCallStarted,
			Payload: map[string]interface{}{
				"tool_name":   action.ToolName,
				"call_id":     callID,
				"args_summary": textutil.SummarizeJSON(action.ToolArgs, 1000),
				"reason":      action.Summary,
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
		if e.hookMgr != nil {
			e.hookMgr.EmitAsync(ctx, core.HookEvent{
				TaskID: taskCtx.TaskID,
				StepID: step.StepID,
				Type:   core.HookToolCallFinished,
				Payload: map[string]interface{}{
					"call_id":        callID,
					"tool_name":      action.ToolName,
					"status":         string(core.ToolCallFailed),
					"error_message":  errMsg,
					"duration_ms":    time.Since(start).Milliseconds(),
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

	resp, err := e.toolGW.CallValidated(ctx, core.ToolRequest{
		CallID:   callID,
		TaskID:   taskCtx.TaskID,
		StepID:   step.StepID,
		ToolName: action.ToolName,
		Reason:   action.Summary,
		Args:     action.ToolArgs,
		Timeout:  e.config.ToolTimeout,
	})

	obs := &core.Observation{
		ToolName: action.ToolName,
		CallID:   callID,
		Duration: time.Since(start),
	}

	if err != nil {
		record.Status = core.ToolCallFailed
		record.ErrorMessage = err.Error()
		record.EndedAt = time.Now()
		obs.Status = core.ToolCallFailed
		obs.Error = err.Error()
		obs.Summary = fmt.Sprintf("Tool %s failed: %v", action.ToolName, err)
		// Emit HookToolCallFinished for CallValidated error
		if e.hookMgr != nil {
			e.hookMgr.EmitAsync(ctx, core.HookEvent{
				TaskID: taskCtx.TaskID,
				StepID: step.StepID,
				Type:   core.HookToolCallFinished,
				Payload: map[string]interface{}{
					"call_id":        callID,
					"tool_name":      action.ToolName,
					"status":         string(core.ToolCallFailed),
					"error_message":  err.Error(),
					"duration_ms":    time.Since(start).Milliseconds(),
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
		if e.hookMgr != nil {
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
	if e.hookMgr != nil {
		e.hookMgr.EmitAsync(ctx, core.HookEvent{
			TaskID: taskCtx.TaskID,
			StepID: step.StepID,
			Type:   core.HookToolCallFinished,
			Payload: map[string]interface{}{
				"call_id":        callID,
				"tool_name":      action.ToolName,
				"status":         string(record.Status),
				"result_summary": resp.Summary,
				"duration_ms":    time.Since(start).Milliseconds(),
			},
		})
	}

	return obs, record, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return "tool call failed"
}
