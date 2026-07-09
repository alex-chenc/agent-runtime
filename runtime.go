package agentruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/alex-chenc/agent-runtime/apperr"
	"github.com/alex-chenc/agent-runtime/audit"
	"github.com/alex-chenc/agent-runtime/contextbudget"
	"github.com/alex-chenc/agent-runtime/core"
	"github.com/alex-chenc/agent-runtime/correction"
	"github.com/alex-chenc/agent-runtime/executor"
	"github.com/alex-chenc/agent-runtime/exit"
	"github.com/alex-chenc/agent-runtime/hook"
	"github.com/alex-chenc/agent-runtime/internal/clock"
	"github.com/alex-chenc/agent-runtime/internal/ids"
	"github.com/alex-chenc/agent-runtime/internal/limiter"
	"github.com/alex-chenc/agent-runtime/internal/textutil"
	"github.com/alex-chenc/agent-runtime/plan"
	"github.com/alex-chenc/agent-runtime/planner"
	"github.com/alex-chenc/agent-runtime/reflection"
	"github.com/alex-chenc/agent-runtime/task"
	"github.com/alex-chenc/agent-runtime/tool"
)

// TaskRouter 任务路由器接口（可选组件）
type TaskRouter interface {
	Route(ctx context.Context, input RouteInput) (*RouteResult, error)
}

const defaultDirectReplySystemPrompt = "You are the Aegis security assistant. Reply concisely in the same language as the user's request."

// RouteInput 路由输入
type RouteInput struct {
	TaskID      string
	UserMessage string
	Tools       []ToolDescriptor
	MaxSteps    int
}

// Runtime is the main entry point for executing agent tasks.
type Runtime struct {
	llmClient          LLMClient
	toolGateway        ToolGateway
	tools              []ToolDescriptor
	config             RuntimeConfig
	experienceProvider ExperienceProvider
	hookSinks          []HookSink
	promptProvider     PromptProvider
	toolPolicy         ToolPolicy
	clock              Clock
	idGen              IDGenerator
	router             TaskRouter // 可选：智能任务路由器

	mu            sync.Mutex
	activeTasks   map[string]*task.Context
	activeCancels map[string]context.CancelFunc
}

// New creates a new Runtime with the given options.
func New(opts ...Option) (*Runtime, error) {
	r := &Runtime{
		config:        DefaultConfig(),
		activeTasks:   make(map[string]*task.Context),
		activeCancels: make(map[string]context.CancelFunc),
	}
	for _, opt := range opts {
		if err := opt(r); err != nil {
			return nil, fmt.Errorf("runtime: option error: %w", err)
		}
	}
	if r.clock == nil {
		r.clock = clock.RealClock{}
	}
	if r.idGen == nil {
		r.idGen = ids.Generator{}
	}
	return r, nil
}

// Run executes a task and returns the structured result.
func (r *Runtime) Run(ctx context.Context, input TaskInput) (*TaskResult, error) {
	if r.llmClient == nil {
		return nil, fmt.Errorf("runtime: LLMClient is required")
	}
	if len(r.tools) > 0 && r.toolGateway == nil {
		return nil, fmt.Errorf("runtime: ToolGateway is required when tools are registered")
	}
	if input.UserInput == "" {
		return nil, fmt.Errorf("runtime: UserInput is required")
	}

	cfg := r.config
	if input.ConfigPatch != nil {
		cfg = cfg.ApplyPatch(*input.ConfigPatch)
	}
	providedPlan := input.InitialPlan != nil
	if providedPlan {
		// A caller-provided plan is authoritative. Reflection may retry or skip
		// a step, but the runtime must not replace or append plan steps.
		cfg.EnableCorrection = false
		cfg.AllowDynamicNewSteps = false
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("runtime: invalid config: %w", err)
	}

	taskID := input.TaskID
	if taskID == "" {
		taskID = r.idGen.Generate()
	}

	taskCtx := &task.Context{
		TaskID:       taskID,
		Input:        input,
		Config:       cfg,
		Status:       StatusInitializing,
		ToolSnapshot: r.tools,
		StartedAt:    r.clock.Now(),
	}

	limiters := &limiter.Limiter{}
	errMgr := apperr.NewManager(r.idGen)
	exitCtrl := exit.NewController(cfg)

	ctx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()
	if cfg.TaskTimeout > 0 {
		var cancelTimeout context.CancelFunc
		ctx, cancelTimeout = context.WithTimeout(ctx, cfg.TaskTimeout)
		defer cancelTimeout()
	}

	hookMgr := hook.NewManager(r.hookSinks, r.idGen, cfg.HookTimeout)

	recordingLLM := &recordingLLMClient{
		next:    r.llmClient,
		taskCtx: taskCtx,
		limits:  limiters,
		idGen:   r.idGen,
		clock:   r.clock,
		hookMgr: hookMgr,
	}

	r.mu.Lock()
	r.activeTasks[taskID] = taskCtx
	r.activeCancels[taskID] = cancelRun
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.activeTasks, taskID)
		delete(r.activeCancels, taskID)
		r.mu.Unlock()
	}()

	toolRegistry, err := tool.NewRegistry(r.tools)
	if err != nil {
		return nil, fmt.Errorf("runtime: tool registry: %w", err)
	}

	var toolGW *tool.GatewayWrapper
	if r.toolGateway != nil {
		policy := r.toolPolicy
		if policy == nil {
			policy = &tool.DefaultPolicy{
				AllowHighRisk:  cfg.AllowHighRiskTools,
				AllowDangerous: cfg.AllowDangerousTools,
				DisabledTools:  cfg.DisabledTools,
			}
		}
		toolGW = tool.NewGatewayWrapper(r.toolGateway, toolRegistry, policy, r.idGen, cfg.ToolTimeout)
	}
	planMgr := plan.NewManager()
	planValidator := plan.NewValidator(cfg.MaxPlanSteps, r.tools, cfg.DisabledTools)
	auditPolicy := audit.NewPolicy(cfg.AuditEveryNSteps, cfg.AuditEveryNTurns)

	var reflector *reflection.Reflector
	if cfg.EnableReflection {
		reflector = reflection.New(recordingLLM, r.idGen)
	}
	var auditor *audit.Auditor
	if cfg.EnableAudit {
		auditor = audit.New(recordingLLM, r.idGen)
	}
	var corrector *correction.Corrector
	if cfg.EnableCorrection {
		corrector = correction.New(recordingLLM, r.idGen)
	}

	// Create context compressor if enabled
	var compressor *contextbudget.Compressor
	if cfg.EnableContextCompress {
		estimator := contextbudget.NewDefaultEstimator()
		compressor = contextbudget.NewCompressor(cfg, estimator, recordingLLM)
	}

	// Preflight context budget check
	if compressor != nil {
		preflightMessages := []LLMMessage{
			{Role: "system", Content: "preflight estimate"},
			{Role: "user", Content: input.UserInput},
		}
		snap := compressor.GetBudgetSnapshot(preflightMessages)
		taskCtx.BatchUpdate(func() {
			taskCtx.ContextBudget = &snap
		})
		hookMgr.EmitType(ctx, HookContextBudgetChecked, taskID, taskCtx.Snapshot())

		if snap.ContextRatio > 1.0 {
			taskCtx.BatchUpdate(func() {
				taskCtx.Status = StatusFailed
				taskCtx.ExitReason = ExitContextOverflow
				taskCtx.EndedAt = r.clock.Now()
			})
			return r.buildResult(taskCtx, nil, nil, limiters, errMgr), nil
		}
	}

	hookMgr.EmitType(ctx, HookTaskStarted, taskID, taskCtx.Snapshot())

	taskCtx.SetStatus(StatusPlanning)

	experienceSummary := ""
	if cfg.EnableExperience {
		items := append([]ExperienceItem{}, input.InitialExperience...)
		if r.experienceProvider != nil {
			resp, err := r.experienceProvider.Fetch(ctx, ExperienceRequest{
				TaskID:   taskID,
				Query:    input.UserInput,
				MaxItems: cfg.MaxPlanSteps,
			})
			if err != nil {
				expErr := apperr.New(ErrExperience, "experience", taskID, "", err.Error())
				expErr.ErrorID = r.idGen.Generate()
				errMgr.Add(expErr)
				taskCtx.BatchUpdate(func() {
					taskCtx.Errors = append(taskCtx.Errors, expErr)
				})
			} else {
				items = append(items, resp.Items...)
			}
		}
		if len(items) > 0 {
			experienceSummary = summarizeExperience(items)
			expSnap := taskCtx.BatchUpdate(func() {
				for _, item := range items {
					taskCtx.ExperienceUsage = append(taskCtx.ExperienceUsage, ExperienceUsage{
						ItemID:    item.ID,
						UsedAt:    "planning",
						Helpful:   true,
						Timestamp: r.clock.Now(),
					})
				}
			})
			hookMgr.EmitType(ctx, HookExperienceLoaded, taskID, expSnap)
		}
	}

	p := planner.New(recordingLLM, r.idGen, r.promptProvider)
	planInput := planner.PlanInput{
		TaskID:         taskID,
		UserInput:      input.UserInput,
		UserContext:    input.UserContext,
		Tools:          r.tools,
		MaxSteps:       cfg.MaxPlanSteps,
		Experience:     experienceSummary,
		DisabledTools:  cfg.DisabledTools,
		AllowHighRisk:  cfg.AllowHighRiskTools,
		AllowDangerous: cfg.AllowDangerousTools,
	}

	// 任务路由：Router 或 Assess
	var initialPlan *core.Plan
	var composedPrompt string

	if providedPlan {
		initialPlan = prepareProvidedPlan(input.InitialPlan, r.idGen, r.clock.Now())
	} else if r.router != nil {
		// 使用 Router 进行智能路由
		routeResult, routeErr := r.router.Route(ctx, RouteInput{
			TaskID:      taskID,
			UserMessage: input.UserInput,
			Tools:       r.tools,
			MaxSteps:    cfg.MaxPlanSteps,
		})

		if routeErr == nil && routeResult.Action == core.ActionDirectReply {
			// 问候/闲聊：直接回复
			return r.handleDirectReply(ctx, input, taskCtx, routeResult.ComposedPrompt, hookMgr, limiters, errMgr), nil
		}

		if routeErr == nil && routeResult.ComposedPrompt != "" {
			composedPrompt = routeResult.ComposedPrompt
		}

		if routeErr == nil && routeResult.Action == core.ActionSimpleCall {
			// 简单调用：跳过计划
			initialPlan = p.GenerateNoPlan(planInput)
		} else {
			// 完整流程：生成计划
			var planErr error
			initialPlan, planErr = p.Generate(ctx, planInput)
			if planErr != nil {
				exitReason := ExitPlanGenerationFailed
				if reason, ok := r.exitReasonFromContext(ctx, taskCtx); ok {
					exitReason = reason
				}
				taskCtx.BatchUpdate(func() {
					taskCtx.Status = StatusPlanFailed
					taskCtx.ExitReason = exitReason
					taskCtx.EndedAt = r.clock.Now()
					taskCtx.Counters = countersFromLimiter(limiters)
				})
				errMgr.Add(apperr.New(ErrPlanGeneration, "planner", taskID, "", planErr.Error()))
				return r.buildResult(taskCtx, nil, nil, limiters, errMgr), nil
			}
		}
	} else {
		// 无 Router：使用原有 Assess 流程
		assess, assessErr := p.Assess(ctx, planInput)
		if assessErr == nil && !assess.NeedsPlan {
			initialPlan = p.GenerateNoPlan(planInput)
		} else {
			var planErr error
			initialPlan, planErr = p.Generate(ctx, planInput)
			if planErr != nil {
				exitReason := ExitPlanGenerationFailed
				if reason, ok := r.exitReasonFromContext(ctx, taskCtx); ok {
					exitReason = reason
				}
				taskCtx.BatchUpdate(func() {
					taskCtx.Status = StatusPlanFailed
					taskCtx.ExitReason = exitReason
					taskCtx.EndedAt = r.clock.Now()
					taskCtx.Counters = countersFromLimiter(limiters)
				})
				errMgr.Add(apperr.New(ErrPlanGeneration, "planner", taskID, "", planErr.Error()))
				return r.buildResult(taskCtx, nil, nil, limiters, errMgr), nil
			}
		}
	}

	// 如果有 Router 拼接的提示词，包装 PromptProvider
	effectiveProvider := r.promptProvider
	if composedPrompt != "" {
		effectiveProvider = &composedPromptProvider{
			base:     r.promptProvider,
			composed: composedPrompt,
		}
	}

	// 验证并设置计划（NeedsPlan=false 的情况已在上面处理）
	if initialPlan.NeedsPlan {
		validation := planValidator.Validate(initialPlan)
		if !validation.Valid {
			errMsg := ""
			for _, e := range validation.Errors {
				errMsg += e + "; "
			}
			taskCtx.BatchUpdate(func() {
				taskCtx.Status = StatusPlanFailed
				taskCtx.ExitReason = ExitPlanValidationFailed
				taskCtx.EndedAt = r.clock.Now()
				taskCtx.Counters = countersFromLimiter(limiters)
			})
			errMgr.Add(apperr.New(ErrPlanValidation, "planner", taskID, "", errMsg))
			return r.buildResult(taskCtx, initialPlan, nil, limiters, errMgr), nil
		}

		planMgr.SetInitialPlan(initialPlan)
		taskCtx.BatchUpdate(func() {
			taskCtx.InitialPlan = initialPlan
			taskCtx.CurrentPlan = initialPlan
			taskCtx.Status = StatusRunning
		})
		hookMgr.EmitType(ctx, HookPlanCreated, taskID, taskCtx.Snapshot())
	} else {
		// 简单任务：直接设置为运行状态
		planMgr.SetInitialPlan(initialPlan)
		taskCtx.SetStatus(StatusRunning)
	}

	completedSteps := 0
	for {
		currentCfg := taskCtx.ConfigSnapshot()
		exitCtrl = exit.NewController(currentCfg)

		if reason, ok := r.exitReasonFromContext(ctx, taskCtx); ok {
			taskCtx.BatchUpdate(func() {
				taskCtx.ExitReason = reason
				taskCtx.Counters = countersFromLimiter(limiters)
			})
			break
		}

		if decision := exitCtrl.Check(taskCtx.IsInterrupted(), limiters); decision.ShouldExit {
			taskCtx.BatchUpdate(func() {
				taskCtx.ExitReason = decision.Reason
				taskCtx.Counters = countersFromLimiter(limiters)
			})
			break
		}

		nextStep := planMgr.NextExecutableStep()
		if nextStep == nil {
			taskCtx.BatchUpdate(func() {
				if taskCtx.ExitReason == "" {
					taskCtx.ExitReason = ExitNormalCompleted
				}
				taskCtx.Counters = countersFromLimiter(limiters)
			})
			break
		}

		nextStep.Status = StepRunning
		snap := taskCtx.BatchUpdate(func() {
			taskCtx.CurrentStepID = nextStep.StepID
			taskCtx.Status = StatusRunning
		})
		hookMgr.EmitAsync(ctx, HookEvent{TaskID: taskID, Type: HookStepStarted, StepID: nextStep.StepID, Snapshot: snap})

		stepCtx := &executor.StepContext{
			TaskID:        taskID,
			UserInput:     input.UserInput,
			PlanGoal:      initialPlan.Goal,
			Metadata:      input.Metadata,
			PreviousSteps: copyStepExecutions(taskCtx),
		}

		reactExec := executor.NewReActExecutor(recordingLLM, toolGW, r.experienceProvider, r.idGen, effectiveProvider, currentCfg, hookMgr, compressor)
		stepResult := reactExec.RunStep(ctx, stepCtx, nextStep)

		exec := StepExecution{
			StepID:     stepResult.StepID,
			Attempt:    nextStep.RetryCount + 1,
			Status:     stepResult.Status,
			StartedAt:  stepResult.StartedAt,
			EndedAt:    stepResult.EndedAt,
			ReactTurns: stepResult.Turns,
			Result:     stepResult.Result,
			Evidence:   stepResult.Evidence,
			Error:      stepResult.Error,
		}

		nextStep.Status = stepResult.Status
		for _, turn := range stepResult.Turns {
			limiters.IncrTotalTurns()
			if turn.Observation != nil {
				limiters.IncrToolCalls()
				if turn.Observation.Status == ToolCallFailed || turn.Observation.Status == ToolCallTimeout || turn.Observation.Status == ToolCallCancelled {
					limiters.IncrToolFailures()
				}
			}
			if turn.ParseError != nil {
				limiters.IncrParseFailures()
			} else {
				limiters.ResetParseFailures()
			}
		}
		for _, stepErr := range stepResult.Errors {
			errMgr.Add(stepErr)
		}

		stepSnap := taskCtx.BatchUpdate(func() {
			taskCtx.Steps = append(taskCtx.Steps, exec)
			taskCtx.ToolCalls = append(taskCtx.ToolCalls, stepResult.ToolCalls...)
			taskCtx.ExperienceUsage = append(taskCtx.ExperienceUsage, stepResult.ExperienceUsage...)
			taskCtx.Errors = append(taskCtx.Errors, stepResult.Errors...)
			taskCtx.Counters = countersFromLimiter(limiters)
		})
		if stepResult.Status == StepCompleted {
			completedSteps++
			hookMgr.EmitAsync(ctx, HookEvent{TaskID: taskID, Type: HookStepCompleted, StepID: nextStep.StepID, Snapshot: stepSnap})
		} else {
			hookMgr.EmitAsync(ctx, HookEvent{TaskID: taskID, Type: HookStepFailed, StepID: nextStep.StepID, Snapshot: stepSnap})
		}

		if stepResult.Status == StepFailed && stepResult.Error != nil && currentCfg.EnableReflection && reflector != nil {
			if !limiters.ExceedsReflections(currentCfg.MaxReflections) {
				taskCtx.SetStatus(StatusReflecting)
				hookMgr.EmitType(ctx, HookReflectionStarted, taskID, taskCtx.Snapshot())
				refl, err := reflector.Reflect(ctx, reflection.ReflectInput{
					TaskID:     taskID,
					StepID:     nextStep.StepID,
					Trigger:    "step_failed",
					Error:      *stepResult.Error,
					StepResult: stepResult.Result,
					PlanGoal:   initialPlan.Goal,
				})
				if err == nil && refl != nil {
					limiters.IncrReflections()
					reflSnap := taskCtx.BatchUpdate(func() {
						taskCtx.Reflections = append(taskCtx.Reflections, *refl)
						taskCtx.Counters = countersFromLimiter(limiters)
					})
					hookMgr.EmitType(ctx, HookReflectionFinished, taskID, reflSnap)

					// Act on the reflection recommendation
					handled := r.handleReflectionRecommendation(
						ctx, taskID, refl, nextStep, planMgr, corrector, limiters,
						currentCfg, taskCtx, hookMgr, errMgr,
					)
					if handled {
						taskCtx.SetStatus(StatusRunning)
						continue
					}
				}
				taskCtx.SetStatus(StatusRunning)
			}
		}

		if currentCfg.EnableAudit && auditor != nil {
			shouldAudit := auditPolicy.ShouldAuditBySteps(completedSteps)
			if shouldAudit && !limiters.ExceedsAudits(currentCfg.MaxAudits) {
				taskCtx.SetStatus(StatusAuditing)
				auditResult, err := auditor.Audit(ctx, audit.AuditInput{
					TaskID:      taskID,
					Goal:        initialPlan.Goal,
					CurrentStep: nextStep.StepID,
					Trigger:     "step_completed",
				})
				if err == nil && auditResult != nil {
					limiters.IncrAudits()
					auditSnap := taskCtx.BatchUpdate(func() {
						taskCtx.Audits = append(taskCtx.Audits, *auditResult)
						taskCtx.Counters = countersFromLimiter(limiters)
					})
					hookMgr.EmitType(ctx, HookAuditFinished, taskID, auditSnap)

					if auditResult.ShouldExit {
						taskCtx.BatchUpdate(func() {
							taskCtx.ExitReason = auditResult.ExitReason
							taskCtx.Counters = countersFromLimiter(limiters)
						})
						break
					}

					if auditResult.Decision == AuditCorrectPlan && currentCfg.EnableCorrection && corrector != nil {
						if !limiters.ExceedsCorrections(currentCfg.MaxCorrections) {
							taskCtx.SetStatus(StatusCorrecting)
							corrResult, err := corrector.Correct(ctx, correction.CorrectionInput{
								TaskID:        taskID,
								CurrentPlan:   planMgr.CurrentPlan(),
								Trigger:       "audit",
								Hint:          auditResult.CorrectionHint,
								AllowNewSteps: currentCfg.AllowDynamicNewSteps,
								MaxSteps:      currentCfg.MaxPlanSteps,
							})
							if err == nil && corrResult != nil {
								completedIDs := completedStepIDs(planMgr.CurrentPlan())
								validation := correction.ValidateCorrection(corrResult, planMgr.CurrentPlan(), completedIDs)
								if validation.Valid {
									_ = planMgr.ApplyCorrection(corrResult)
									limiters.IncrCorrections()
									corrSnap := taskCtx.BatchUpdate(func() {
										taskCtx.Corrections = append(taskCtx.Corrections, *corrResult)
										taskCtx.Counters = countersFromLimiter(limiters)
									})
									hookMgr.EmitType(ctx, HookCorrectionApplied, taskID, corrSnap)
								} else {
									corrResult.Valid = false
									corrResult.ValidationErrors = append(corrResult.ValidationErrors, validation.Errors...)
									corrErr := apperr.New(ErrCorrection, "correction", taskID, nextStep.StepID, strings.Join(validation.Errors, "; "))
									corrErr.ErrorID = r.idGen.Generate()
									errMgr.Add(corrErr)
									corrSnap := taskCtx.BatchUpdate(func() {
										taskCtx.Corrections = append(taskCtx.Corrections, *corrResult)
										taskCtx.Errors = append(taskCtx.Errors, corrErr)
										taskCtx.Counters = countersFromLimiter(limiters)
									})
									hookMgr.EmitType(ctx, HookCorrectionApplied, taskID, corrSnap)
								}
							}
							taskCtx.SetStatus(StatusRunning)
						}
					}
				}
				taskCtx.SetStatus(StatusRunning)
			}
		}
	}

	taskCtx.BatchUpdate(func() {
		taskCtx.Status = StatusSummarizing
		taskCtx.EndedAt = r.clock.Now()
		taskCtx.Counters = countersFromLimiter(limiters)
	})
	r.generateFinalAnswer(ctx, recordingLLM, taskCtx, errMgr)
	result := r.buildResult(taskCtx, planMgr.InitialPlan(), planMgr.CurrentPlan(), limiters, errMgr)
	if err := hookMgr.Emit(ctx, HookEvent{
		TaskID:   taskID,
		Type:     HookTaskFinished,
		Payload:  result,
		Snapshot: taskCtx.Snapshot(),
	}); err != nil && cfg.FailOnHookError {
		hookErr := RuntimeError{
			ErrorID:    r.idGen.Generate(),
			Kind:       ErrSystem,
			Stage:      "hook",
			TaskID:     taskID,
			Message:    err.Error(),
			OccurredAt: r.clock.Now(),
		}
		result.Errors = append(result.Errors, hookErr)
		result.Status = StatusFailed
		result.ExitReason = ExitSystemError
		result.FinalAnswer = buildFinalAnswer(result.Status, result.ExitReason, result.StepExecutions, result.Errors)
	}

	return result, nil
}

// Interrupt signals a running task to stop.
func (r *Runtime) Interrupt(taskID string, reason string) error {
	r.mu.Lock()
	taskCtx, ok := r.activeTasks[taskID]
	cancel := r.activeCancels[taskID]
	r.mu.Unlock()
	if !ok {
		return fmt.Errorf("runtime: task %q not found or not running", taskID)
	}
	taskCtx.SetInterrupted(reason)
	if cancel != nil {
		cancel()
	}
	return nil
}

// UpdateConfig applies a configuration patch to a running task.
func (r *Runtime) UpdateConfig(taskID string, patch ConfigPatch) error {
	r.mu.Lock()
	taskCtx, ok := r.activeTasks[taskID]
	r.mu.Unlock()
	if !ok {
		return fmt.Errorf("runtime: task %q not found or not running", taskID)
	}

	taskCtx.Lock()
	defer taskCtx.Unlock()

	oldCfg := taskCtx.Config
	newCfg := oldCfg.ApplyPatch(patch)
	if err := newCfg.Validate(); err != nil {
		return fmt.Errorf("runtime: invalid config patch: %w", err)
	}
	if patch.MaxTotalTurns != nil && *patch.MaxTotalTurns < taskCtx.Counters.TotalTurns {
		return fmt.Errorf("runtime: MaxTotalTurns cannot be lower than current total turns %d", taskCtx.Counters.TotalTurns)
	}
	if patch.MaxToolCalls != nil && *patch.MaxToolCalls < taskCtx.Counters.ToolCalls {
		return fmt.Errorf("runtime: MaxToolCalls cannot be lower than current tool calls %d", taskCtx.Counters.ToolCalls)
	}
	taskCtx.Config = newCfg
	now := time.Now()

	if patch.MaxTotalTurns != nil && oldCfg.MaxTotalTurns != *patch.MaxTotalTurns {
		taskCtx.ConfigChanges = append(taskCtx.ConfigChanges, ConfigChange{
			Field: "max_total_turns", OldValue: fmt.Sprintf("%d", oldCfg.MaxTotalTurns),
			NewValue: fmt.Sprintf("%d", *patch.MaxTotalTurns), OccurredAt: now,
		})
	}
	if patch.MaxPlanSteps != nil && oldCfg.MaxPlanSteps != *patch.MaxPlanSteps {
		taskCtx.ConfigChanges = append(taskCtx.ConfigChanges, ConfigChange{
			Field: "max_plan_steps", OldValue: fmt.Sprintf("%d", oldCfg.MaxPlanSteps),
			NewValue: fmt.Sprintf("%d", *patch.MaxPlanSteps), OccurredAt: now,
		})
	}
	if patch.MaxStepReactTurns != nil && oldCfg.MaxStepReactTurns != *patch.MaxStepReactTurns {
		taskCtx.ConfigChanges = append(taskCtx.ConfigChanges, ConfigChange{
			Field: "max_step_react_turns", OldValue: fmt.Sprintf("%d", oldCfg.MaxStepReactTurns),
			NewValue: fmt.Sprintf("%d", *patch.MaxStepReactTurns), OccurredAt: now,
		})
	}
	if patch.MaxToolCalls != nil && oldCfg.MaxToolCalls != *patch.MaxToolCalls {
		taskCtx.ConfigChanges = append(taskCtx.ConfigChanges, ConfigChange{
			Field: "max_tool_calls", OldValue: fmt.Sprintf("%d", oldCfg.MaxToolCalls),
			NewValue: fmt.Sprintf("%d", *patch.MaxToolCalls), OccurredAt: now,
		})
	}
	if patch.EnableReflection != nil && oldCfg.EnableReflection != *patch.EnableReflection {
		taskCtx.ConfigChanges = append(taskCtx.ConfigChanges, ConfigChange{
			Field: "enable_reflection", OldValue: fmt.Sprintf("%t", oldCfg.EnableReflection),
			NewValue: fmt.Sprintf("%t", *patch.EnableReflection), OccurredAt: now,
		})
	}
	if patch.EnableAudit != nil && oldCfg.EnableAudit != *patch.EnableAudit {
		taskCtx.ConfigChanges = append(taskCtx.ConfigChanges, ConfigChange{
			Field: "enable_audit", OldValue: fmt.Sprintf("%t", oldCfg.EnableAudit),
			NewValue: fmt.Sprintf("%t", *patch.EnableAudit), OccurredAt: now,
		})
	}
	if patch.EnableCorrection != nil && oldCfg.EnableCorrection != *patch.EnableCorrection {
		taskCtx.ConfigChanges = append(taskCtx.ConfigChanges, ConfigChange{
			Field: "enable_correction", OldValue: fmt.Sprintf("%t", oldCfg.EnableCorrection),
			NewValue: fmt.Sprintf("%t", *patch.EnableCorrection), OccurredAt: now,
		})
	}
	return nil
}

func (r *Runtime) buildResult(
	taskCtx *task.Context,
	initialPlan, currentPlan *Plan,
	limiters *limiter.Limiter,
	errMgr *apperr.Manager,
) *TaskResult {
	taskCtx.Lock()
	defer taskCtx.Unlock()

	status := taskCtx.Status
	if status == StatusRunning || status == StatusSummarizing {
		if taskCtx.ExitReason == ExitNormalCompleted {
			status = StatusCompleted
		} else if taskCtx.ExitReason == ExitUserInterrupted {
			status = StatusInterrupted
		} else if taskCtx.ExitReason != "" {
			status = StatusLimited
		}
	}

	completed, failed, skipped := 0, 0, 0
	for _, s := range taskCtx.Steps {
		switch s.Status {
		case StepCompleted:
			completed++
		case StepFailed:
			failed++
		case StepSkipped:
			skipped++
		}
	}
	errors := mergeErrors(taskCtx.Errors, errMgr.Errors())
	finalAnswer := taskCtx.FinalAnswer
	if finalAnswer == "" {
		finalAnswer = buildFinalAnswer(status, taskCtx.ExitReason, taskCtx.Steps, errors)
	}

	return &TaskResult{
		TaskID:      taskCtx.TaskID,
		Status:      status,
		ExitReason:  taskCtx.ExitReason,
		FinalAnswer: finalAnswer,
		Completion: CompletionSummary{
			CompletedSteps: completed,
			FailedSteps:    failed,
			SkippedSteps:   skipped,
			ToolCalls:      limiters.ToolCalls(),
			ModelCalls:     limiters.ModelCalls(),
		},
		InitialPlan:        initialPlan,
		FinalPlan:          currentPlan,
		StepExecutions:     taskCtx.Steps,
		ToolCalls:          taskCtx.ToolCalls,
		ModelCalls:         taskCtx.ModelCalls,
		Errors:             errors,
		Reflections:        taskCtx.Reflections,
		Audits:             taskCtx.Audits,
		Corrections:        taskCtx.Corrections,
		ExperienceUsage:    taskCtx.ExperienceUsage,
		ConfigChanges:      taskCtx.ConfigChanges,
		CompressionRecords: taskCtx.CompressionRecords,
		ContextBudget:      taskCtx.ContextBudget,
		Metrics: RuntimeMetrics{
			TotalTurns:            limiters.TotalTurns(),
			TotalToolCalls:        limiters.ToolCalls(),
			TotalModelCalls:       limiters.ModelCalls(),
			TotalToolFailures:     limiters.ToolFailures(),
			TotalModelFailures:    limiters.ModelFailures(),
			TotalParseFailures:    limiters.ParseFailures(),
			TotalAudits:           limiters.Audits(),
			TotalReflections:      limiters.Reflections(),
			TotalCorrections:      limiters.Corrections(),
			TotalPromptTokens:     taskCtx.TotalPromptTokens,
			TotalCompletionTokens: taskCtx.TotalCompletionTokens,
			TotalDuration:         taskCtx.EndedAt.Sub(taskCtx.StartedAt),
		},
		StartedAt: taskCtx.StartedAt,
		EndedAt:   taskCtx.EndedAt,
		Metadata:  taskCtx.Input.Metadata,
	}
}

type recordingLLMClient struct {
	next    LLMClient
	taskCtx *task.Context
	limits  *limiter.Limiter
	idGen   IDGenerator
	clock   Clock
	hookMgr *hook.Manager
}

func (c *recordingLLMClient) Complete(ctx context.Context, req LLMRequest) (LLMResponse, error) {
	if req.CallID == "" {
		req.CallID = c.idGen.Generate()
	}
	started := c.clock.Now()
	resp, err := c.next.Complete(ctx, req)
	ended := c.clock.Now()

	c.limits.IncrModelCalls()
	if err != nil {
		c.limits.IncrModelFailures()
	}

	record := ModelCallRecord{
		CallID:           req.CallID,
		TaskID:           req.TaskID,
		StepID:           req.StepID,
		Purpose:          req.Purpose,
		InputSummary:     summarizeMessages(req.Messages),
		OutputSummary:    textutil.Truncate(resp.Content, 8000),
		Schema:           req.ResponseSchema,
		Model:            resp.Model,
		PromptTokens:     resp.Usage.PromptTokens,
		CompletionTokens: resp.Usage.CompletionTokens,
		TokensUsed:       resp.Usage.TotalTokens,
		Latency:          ended.Sub(started),
		OccurredAt:       started,
	}
	if err != nil {
		record.Error = err.Error()
	}

	// Emit HookModelCallFinished with full response content for streaming
	if c.hookMgr != nil && err == nil && resp.Content != "" {
		c.hookMgr.EmitAsync(ctx, HookEvent{
			TaskID: req.TaskID,
			StepID: req.StepID,
			Type:   HookModelCallFinished,
			Payload: map[string]interface{}{
				"output_summary": resp.Content,
				"purpose":        string(req.Purpose),
				"model":          resp.Model,
				"tokens_used":    resp.Usage.TotalTokens,
			},
		})
	}

	c.taskCtx.BatchUpdate(func() {
		c.taskCtx.ModelCalls = append(c.taskCtx.ModelCalls, record)
		c.taskCtx.Counters = countersFromLimiter(c.limits)
		c.taskCtx.TotalPromptTokens += resp.Usage.PromptTokens
		c.taskCtx.TotalCompletionTokens += resp.Usage.CompletionTokens
		if err != nil {
			c.taskCtx.Errors = append(c.taskCtx.Errors, RuntimeError{
				ErrorID:     c.idGen.Generate(),
				Kind:        ErrModelCall,
				Stage:       string(req.Purpose),
				TaskID:      req.TaskID,
				StepID:      req.StepID,
				ModelCallID: req.CallID,
				Message:     err.Error(),
				Recoverable: true,
				OccurredAt:  started,
			})
		}
	})

	return resp, err
}

func (r *Runtime) generateFinalAnswer(ctx context.Context, llm LLMClient, taskCtx *task.Context, errMgr *apperr.Manager) {
	taskCtx.Lock()
	status := taskCtx.Status
	if taskCtx.ExitReason == ExitNormalCompleted {
		status = StatusCompleted
	} else if taskCtx.ExitReason == ExitUserInterrupted {
		status = StatusInterrupted
	} else if taskCtx.ExitReason != "" {
		status = StatusLimited
	}
	errors := mergeErrors(taskCtx.Errors, errMgr.Errors())
	fallback := buildFinalAnswer(status, taskCtx.ExitReason, taskCtx.Steps, errors)
	prompt := buildSummaryPrompt(status, taskCtx.ExitReason, taskCtx.Steps, errors)
	taskID := taskCtx.TaskID
	taskCtx.Unlock()

	summaryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), taskCtx.ConfigSnapshot().ModelTimeout)
	defer cancel()

	systemPrompt := "Generate a concise final answer using only the execution evidence provided. Do not contradict successful tool observations, report tools that were never called, or claim unexecuted work was completed. Respond as JSON: {\"final_answer\":\"...\"}."
	if r.promptProvider != nil {
		if bundle, err := r.promptProvider.Build(summaryCtx, PromptRequest{TaskID: taskID, Purpose: PurposeSummarize}); err == nil && bundle.SystemPrompt != "" {
			systemPrompt = bundle.SystemPrompt
		}
	}

	resp, err := llm.Complete(summaryCtx, LLMRequest{
		TaskID:         taskID,
		Purpose:        PurposeSummarize,
		ResponseSchema: "final_summary",
		Messages: []LLMMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: prompt},
		},
		Timeout: taskCtx.ConfigSnapshot().ModelTimeout,
	})
	finalAnswer := fallback
	if err == nil {
		if parsed := parseFinalAnswer(resp.Content); parsed != "" {
			finalAnswer = parsed
		}
	}
	taskCtx.BatchUpdate(func() {
		taskCtx.FinalAnswer = finalAnswer
	})
}

func buildSummaryPrompt(status TaskStatus, reason ExitReason, steps []StepExecution, errors []RuntimeError) string {
	var b strings.Builder
	b.WriteString("Status: ")
	b.WriteString(string(status))
	if reason != "" {
		b.WriteString("\nExit reason: ")
		b.WriteString(string(reason))
	}
	b.WriteString("\nSteps:")
	for _, step := range steps {
		b.WriteString("\n- ")
		b.WriteString(step.StepID)
		b.WriteString(" [")
		b.WriteString(string(step.Status))
		b.WriteString("]")
		if step.Result != "" {
			b.WriteString(": ")
			b.WriteString(step.Result)
		}
		for _, turn := range step.ReactTurns {
			if turn.Observation == nil {
				continue
			}
			b.WriteString("\n  - observed tool=")
			b.WriteString(turn.Observation.ToolName)
			b.WriteString(" call_id=")
			b.WriteString(turn.Observation.CallID)
			b.WriteString(" status=")
			b.WriteString(string(turn.Observation.Status))
			if turn.Observation.Content != "" {
				b.WriteString(" content=")
				b.WriteString(textutil.Truncate(turn.Observation.Content, 2000))
			}
			if turn.Observation.Error != "" {
				b.WriteString(" error=")
				b.WriteString(textutil.Truncate(turn.Observation.Error, 1000))
			}
		}
	}
	if len(errors) > 0 {
		b.WriteString("\nErrors:")
		for _, err := range errors {
			b.WriteString("\n- ")
			b.WriteString(string(err.Kind))
			b.WriteString(": ")
			b.WriteString(err.Message)
		}
	}
	return textutil.Truncate(b.String(), 16000)
}

func parseFinalAnswer(content string) string {
	jsonStr := textutil.ExtractJSON(content)
	if jsonStr == "" {
		return strings.TrimSpace(content)
	}
	var raw struct {
		FinalAnswer string `json:"final_answer"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return ""
	}
	if raw.FinalAnswer == "" {
		// LLM returned structured JSON (e.g. attack_graph + conclusions)
		// without a final_answer field. Preserve the full JSON so the
		// caller can extract structured data from it.
		return jsonStr
	}
	return strings.TrimSpace(raw.FinalAnswer)
}

func summarizeMessages(messages []LLMMessage) string {
	var b strings.Builder
	for i, msg := range messages {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(msg.Role)
		b.WriteString(": ")
		b.WriteString(msg.Content)
	}
	return textutil.Truncate(b.String(), 1000)
}

func summarizeExperience(items []ExperienceItem) string {
	var b strings.Builder
	for _, item := range items {
		if item.Summary == "" && item.Content == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		if item.ID != "" {
			b.WriteString("- ")
			b.WriteString(item.ID)
			b.WriteString(": ")
		} else {
			b.WriteString("- ")
		}
		if item.Summary != "" {
			b.WriteString(item.Summary)
		} else {
			b.WriteString(item.Content)
		}
	}
	return textutil.Truncate(b.String(), 4000)
}

func countersFromLimiter(l *limiter.Limiter) task.Counters {
	return task.Counters{
		ToolCalls:       l.ToolCalls(),
		ToolFailures:    l.ToolFailures(),
		ModelCalls:      l.ModelCalls(),
		ModelFailures:   l.ModelFailures(),
		ParseFailures:   l.ParseFailures(),
		NoProgressTurns: l.NoProgressTurns(),
		TotalTurns:      l.TotalTurns(),
		Audits:          l.Audits(),
		Reflections:     l.Reflections(),
		Corrections:     l.Corrections(),
	}
}

func (r *Runtime) exitReasonFromContext(ctx context.Context, taskCtx *task.Context) (ExitReason, bool) {
	if ctx.Err() == nil {
		return "", false
	}
	if taskCtx.IsInterrupted() {
		return ExitUserInterrupted, true
	}
	if ctx.Err() == context.DeadlineExceeded {
		return ExitTaskTimeout, true
	}
	return ExitUserInterrupted, true
}

func completedStepIDs(p *Plan) map[string]bool {
	completed := make(map[string]bool)
	if p == nil {
		return completed
	}
	for _, step := range p.Steps {
		if step.Status == StepCompleted || step.Status == StepFailed || step.Status == StepSkipped || step.Status == StepReplaced || step.Status == StepInvalidated {
			completed[step.StepID] = true
		}
	}
	return completed
}

func prepareProvidedPlan(source *Plan, idGen IDGenerator, now time.Time) *Plan {
	if source == nil {
		return nil
	}
	planCopy := *source
	if planCopy.PlanID == "" {
		planCopy.PlanID = idGen.Generate()
	}
	if planCopy.Version <= 0 {
		planCopy.Version = 1
	}
	planCopy.NeedsPlan = true
	planCopy.EstSteps = len(source.Steps)
	if planCopy.CreatedAt.IsZero() {
		planCopy.CreatedAt = now
	}
	planCopy.UpdatedAt = now
	planCopy.Assumptions = append([]string(nil), source.Assumptions...)
	planCopy.Steps = make([]PlanStep, len(source.Steps))
	for i := range source.Steps {
		step := source.Steps[i]
		step.SuggestedTools = append([]string(nil), source.Steps[i].SuggestedTools...)
		step.AllowedTools = append([]string(nil), source.Steps[i].AllowedTools...)
		step.Dependencies = append([]string(nil), source.Steps[i].Dependencies...)
		if source.Steps[i].ToolArgs != nil {
			step.ToolArgs = make(map[string]any, len(source.Steps[i].ToolArgs))
			for key, value := range source.Steps[i].ToolArgs {
				step.ToolArgs[key] = value
			}
		}
		if step.StepID == "" {
			step.StepID = fmt.Sprintf("step_%d", i+1)
		}
		step.Status = StepPending
		step.RetryCount = 0
		if step.CreatedBy == "" {
			step.CreatedBy = "caller"
		}
		planCopy.Steps[i] = step
	}
	return &planCopy
}

func copyStepExecutions(taskCtx *task.Context) []StepExecution {
	if taskCtx == nil {
		return nil
	}
	taskCtx.Lock()
	defer taskCtx.Unlock()
	result := make([]StepExecution, len(taskCtx.Steps))
	copy(result, taskCtx.Steps)
	return result
}

func mergeErrors(primary, secondary []RuntimeError) []RuntimeError {
	merged := make([]RuntimeError, 0, len(primary)+len(secondary))
	seen := make(map[string]bool)
	for _, err := range primary {
		merged = append(merged, err)
		if err.ErrorID != "" {
			seen[err.ErrorID] = true
		}
	}
	for _, err := range secondary {
		if err.ErrorID != "" && seen[err.ErrorID] {
			continue
		}
		merged = append(merged, err)
	}
	return merged
}

func buildFinalAnswer(status TaskStatus, reason ExitReason, steps []StepExecution, errors []RuntimeError) string {
	var b strings.Builder
	b.WriteString("Task status: ")
	b.WriteString(string(status))
	if reason != "" {
		b.WriteString("\nExit reason: ")
		b.WriteString(string(reason))
	}
	completed := 0
	for _, step := range steps {
		if step.Status != StepCompleted {
			continue
		}
		completed++
		if step.Result != "" {
			b.WriteString("\nCompleted ")
			b.WriteString(step.StepID)
			b.WriteString(": ")
			b.WriteString(step.Result)
		}
	}
	if completed == 0 {
		b.WriteString("\nNo steps completed successfully.")
	}
	if len(errors) > 0 {
		b.WriteString("\nErrors: ")
		limit := len(errors)
		if limit > 3 {
			limit = 3
		}
		for i := 0; i < limit; i++ {
			if i > 0 {
				b.WriteString("; ")
			}
			b.WriteString(errors[i].Message)
		}
	}
	return b.String()
}

// handleReflectionRecommendation acts on a reflection's recommendation.
// Returns true if the recommendation was handled and the loop should continue.
func (r *Runtime) handleReflectionRecommendation(
	ctx context.Context,
	taskID string,
	refl *ReflectionResult,
	step *PlanStep,
	planMgr *plan.Manager,
	corrector *correction.Corrector,
	limiters *limiter.Limiter,
	cfg RuntimeConfig,
	taskCtx *task.Context,
	hookMgr *hook.Manager,
	errMgr *apperr.Manager,
) bool {
	switch refl.Recommendation {
	case ReflectRetryStep:
		return r.handleRetryStep(ctx, taskID, refl, step, planMgr, limiters, cfg, taskCtx, hookMgr)
	case ReflectSkipStep:
		return r.handleSkipStep(ctx, taskID, step, planMgr, cfg, taskCtx, hookMgr)
	case ReflectCorrectPlan:
		return r.handleCorrectPlan(ctx, taskID, refl, step, planMgr, corrector, limiters, cfg, taskCtx, hookMgr, errMgr)
	case ReflectRequestExperience:
		return r.handleRequestExperience(ctx, taskID, refl, step, planMgr, limiters, cfg, taskCtx, hookMgr)
	case ReflectSummarizeNow:
		taskCtx.BatchUpdate(func() {
			taskCtx.ExitReason = ExitNormalCompleted
			taskCtx.Counters = countersFromLimiter(limiters)
		})
		return true
	case ReflectFail:
		taskCtx.BatchUpdate(func() {
			taskCtx.ExitReason = ExitReflectionUnrecoverable
			taskCtx.Counters = countersFromLimiter(limiters)
		})
		return true
	default:
		return false
	}
}

func (r *Runtime) handleRetryStep(
	ctx context.Context,
	taskID string,
	refl *ReflectionResult,
	step *PlanStep,
	planMgr *plan.Manager,
	limiters *limiter.Limiter,
	cfg RuntimeConfig,
	taskCtx *task.Context,
	hookMgr *hook.Manager,
) bool {
	if step.RetryCount >= cfg.MaxStepRetries {
		return false
	}
	if err := planMgr.ResetStepForRetry(step.StepID); err != nil {
		return false
	}
	snap := taskCtx.BatchUpdate(func() {
		taskCtx.Counters = countersFromLimiter(limiters)
	})
	hookMgr.EmitType(ctx, HookStepRetrying, taskID, snap)
	return true
}

func (r *Runtime) handleSkipStep(
	ctx context.Context,
	taskID string,
	step *PlanStep,
	planMgr *plan.Manager,
	cfg RuntimeConfig,
	taskCtx *task.Context,
	hookMgr *hook.Manager,
) bool {
	if !cfg.AllowSkipFailedStep {
		return false
	}
	if err := planMgr.UpdateStepStatus(step.StepID, StepSkipped); err != nil {
		return false
	}
	snap := taskCtx.BatchUpdate(func() {
		for i := len(taskCtx.Steps) - 1; i >= 0; i-- {
			if taskCtx.Steps[i].StepID == step.StepID {
				taskCtx.Steps[i].Status = StepSkipped
				break
			}
		}
	})
	hookMgr.EmitType(ctx, HookStepSkipped, taskID, snap)
	return true
}

func (r *Runtime) handleCorrectPlan(
	ctx context.Context,
	taskID string,
	refl *ReflectionResult,
	step *PlanStep,
	planMgr *plan.Manager,
	corrector *correction.Corrector,
	limiters *limiter.Limiter,
	cfg RuntimeConfig,
	taskCtx *task.Context,
	hookMgr *hook.Manager,
	errMgr *apperr.Manager,
) bool {
	if !cfg.EnableCorrection || corrector == nil {
		return false
	}
	if limiters.ExceedsCorrections(cfg.MaxCorrections) {
		return false
	}
	taskCtx.SetStatus(StatusCorrecting)
	corrResult, err := corrector.Correct(ctx, correction.CorrectionInput{
		TaskID:        taskID,
		CurrentPlan:   planMgr.CurrentPlan(),
		Trigger:       "reflection",
		Hint:          refl.CorrectionHint,
		AllowNewSteps: cfg.AllowDynamicNewSteps,
		MaxSteps:      cfg.MaxPlanSteps,
	})
	if err != nil || corrResult == nil {
		taskCtx.SetStatus(StatusRunning)
		return false
	}
	completedIDs := completedStepIDs(planMgr.CurrentPlan())
	validation := correction.ValidateCorrection(corrResult, planMgr.CurrentPlan(), completedIDs)
	if validation.Valid {
		_ = planMgr.ApplyCorrection(corrResult)
		limiters.IncrCorrections()
		corrSnap := taskCtx.BatchUpdate(func() {
			taskCtx.Corrections = append(taskCtx.Corrections, *corrResult)
			taskCtx.Counters = countersFromLimiter(limiters)
		})
		hookMgr.EmitType(ctx, HookCorrectionApplied, taskID, corrSnap)
	} else {
		corrResult.Valid = false
		corrResult.ValidationErrors = append(corrResult.ValidationErrors, validation.Errors...)
		corrErr := apperr.New(ErrCorrection, "correction", taskID, step.StepID, strings.Join(validation.Errors, "; "))
		corrErr.ErrorID = r.idGen.Generate()
		errMgr.Add(corrErr)
		corrSnap := taskCtx.BatchUpdate(func() {
			taskCtx.Corrections = append(taskCtx.Corrections, *corrResult)
			taskCtx.Errors = append(taskCtx.Errors, corrErr)
			taskCtx.Counters = countersFromLimiter(limiters)
		})
		hookMgr.EmitType(ctx, HookCorrectionApplied, taskID, corrSnap)
	}
	taskCtx.SetStatus(StatusRunning)
	return true
}

func (r *Runtime) handleRequestExperience(
	ctx context.Context,
	taskID string,
	refl *ReflectionResult,
	step *PlanStep,
	planMgr *plan.Manager,
	limiters *limiter.Limiter,
	cfg RuntimeConfig,
	taskCtx *task.Context,
	hookMgr *hook.Manager,
) bool {
	if !cfg.EnableExperience || r.experienceProvider == nil {
		return r.handleRetryStep(ctx, taskID, refl, step, planMgr, limiters, cfg, taskCtx, hookMgr)
	}
	query := refl.ExperienceQuery
	if query == "" {
		query = refl.RootCause
	}
	resp, err := r.experienceProvider.Fetch(ctx, ExperienceRequest{
		TaskID:   taskID,
		Query:    query,
		MaxItems: 3,
	})
	if err == nil && len(resp.Items) > 0 {
		taskCtx.BatchUpdate(func() {
			for _, item := range resp.Items {
				taskCtx.ExperienceUsage = append(taskCtx.ExperienceUsage, ExperienceUsage{
					ItemID:    item.ID,
					UsedAt:    "reflection",
					Helpful:   true,
					Timestamp: r.clock.Now(),
				})
			}
		})
	}
	return r.handleRetryStep(ctx, taskID, refl, step, planMgr, limiters, cfg, taskCtx, hookMgr)
}

// handleDirectReply 处理直接回复（问候、闲聊等不需要工具的场景）
func (r *Runtime) handleDirectReply(
	ctx context.Context,
	input TaskInput,
	taskCtx *task.Context,
	composedPrompt string,
	hookMgr *hook.Manager,
	limiters *limiter.Limiter,
	errMgr *apperr.Manager,
) *TaskResult {
	taskID := taskCtx.TaskID

	taskCtx.BatchUpdate(func() {
		taskCtx.Status = StatusRunning
	})

	// 使用拼接的提示词或默认提示词
	systemPrompt := composedPrompt
	if systemPrompt == "" {
		systemPrompt = defaultDirectReplySystemPrompt
	}

	timeout := taskCtx.ConfigSnapshot().ModelTimeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	resp, err := r.llmClient.Complete(ctx, LLMRequest{
		TaskID:  taskID,
		Purpose: PurposeReact,
		Messages: []LLMMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: input.UserInput},
		},
		Timeout: timeout,
	})

	finalAnswer := ""
	if err == nil {
		finalAnswer = resp.Content
	} else {
		errMgr.Add(apperr.New(ErrModelCall, "direct_reply", taskID, "", err.Error()))
		finalAnswer = "抱歉，我暂时无法回复。请稍后重试。"
	}

	taskCtx.BatchUpdate(func() {
		taskCtx.Status = StatusCompleted
		taskCtx.ExitReason = ExitNormalCompleted
		taskCtx.FinalAnswer = finalAnswer
		taskCtx.EndedAt = r.clock.Now()
		taskCtx.Counters = countersFromLimiter(limiters)
	})

	return r.buildResult(taskCtx, nil, nil, limiters, errMgr)
}

// composedPromptProvider 包装 PromptProvider，用 Router 拼接的提示词覆盖 React 阶段
type composedPromptProvider struct {
	base     PromptProvider
	composed string
}

func (p *composedPromptProvider) Build(ctx context.Context, req PromptRequest) (PromptBundle, error) {
	if req.Purpose == PurposeReact && p.composed != "" {
		return PromptBundle{SystemPrompt: p.composed}, nil
	}
	if p.base != nil {
		return p.base.Build(ctx, req)
	}
	return PromptBundle{}, nil
}
