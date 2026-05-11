# Agent Runtime

English | [简体中文](README.zh-CN.md)

Agent Runtime is a Go library for running structured, tool-using AI agent tasks.
It turns one user request into a validated plan, executes each step with a ReAct
loop, records model/tool/error traces, and returns a single structured
`TaskResult`.

The project is designed as an embeddable runtime, not as a standalone service.
Callers provide the model adapter, tool adapter, hooks, prompts, and optional
experience store.

## Current Status

This repository contains the core runtime implementation and tests. The public
entry point is the root package:

```go
rt, err := agentruntime.New(...)
result, err := rt.Run(ctx, agentruntime.TaskInput{...})
```

The runtime borrows architectural ideas from LangGraph and AutoGen:

- LangGraph-like task state: task context, plan versions, step state, exits,
  interrupts, and runtime configuration patches.
- AutoGen-like collaboration: planner, ReAct executor, auditor, reflector, and
  corrector cooperate through structured model messages and tool calls.

There is no LangGraph or AutoGen dependency, and the implementation is native
Go code.

## What It Provides

- Plan generation from `TaskInput.UserInput`
- Plan validation against step limits, dependencies, risk levels, and registered
  tools
- Per-step ReAct execution with tool calls, observations, model parse handling,
  progress tracking, and experience requests
- Tool registry, JSON-like argument schema validation, and risk-aware tool
  policy checks
- Reflection after failures, periodic audit, and plan correction
- Final LLM summary with local fallback
- Task interruption through `Runtime.Interrupt`
- Runtime config updates through `Runtime.UpdateConfig`
- Hook events for observability and integration
- Structured result output including plans, steps, model calls, tool calls,
  errors, audits, reflections, corrections, experience usage, config changes,
  and metrics

## What It Does Not Provide

- No HTTP server, CLI, database, queue, or web UI
- No built-in LLM vendor client
- No built-in production tool executor
- No distributed scheduler or durable resume mechanism
- No human approval UI, although tool policies can return approval decisions

Those pieces should be implemented by the application that embeds this runtime.

## Runtime Flow

```text
TaskInput
  -> load initial/provider experience
  -> generate plan with LLM
  -> validate plan
  -> execute each plan step with ReAct
  -> call tools through ToolGateway when allowed
  -> request more experience when needed
  -> audit, reflect, and correct when configured
  -> summarize
  -> TaskResult
```

## Installation

Install it with:

```bash
go get github.com/chenchen511/agent-runtime
```

The module path is:

```text
module github.com/chenchen511/agent-runtime
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"time"

	agentruntime "github.com/chenchen511/agent-runtime"
)

func main() {
	ctx := context.Background()

	rt, err := agentruntime.New(
		agentruntime.WithLLMClient(myLLMClient),
		agentruntime.WithToolGateway(myToolGateway),
		agentruntime.WithTools([]agentruntime.ToolDescriptor{
			{
				Name:         "search_docs",
				Description:  "Search internal documentation.",
				RiskLevel:    agentruntime.RiskReadOnly,
				AutoCallable: true,
				DefaultTimeout: 30 * time.Second,
				ArgsSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{"type": "string"},
					},
					"required":             []any{"query"},
					"additionalProperties": false,
				},
			},
		}),
	)
	if err != nil {
		panic(err)
	}

	result, err := rt.Run(ctx, agentruntime.TaskInput{
		UserInput: "Find the relevant design notes and summarize the next steps.",
		Metadata: map[string]string{"source": "example"},
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(result.Status, result.ExitReason)
	fmt.Println(result.FinalAnswer)
}
```

`myLLMClient` must implement `agentruntime.LLMClient`. `myToolGateway` must
implement `agentruntime.ToolGateway`.

## Main API

### Runtime Construction

```go
rt, err := agentruntime.New(
	agentruntime.WithLLMClient(llm),
	agentruntime.WithToolGateway(tools),
	agentruntime.WithTools(descriptors),
	agentruntime.WithConfig(cfg),
	agentruntime.WithExperienceProvider(experience),
	agentruntime.WithHooks(hookSink),
	agentruntime.WithPromptProvider(prompts),
	agentruntime.WithToolPolicy(policy),
)
```

Required:

- `WithLLMClient` is required for all tasks.
- `WithToolGateway` is required when tools are registered.

Optional:

- `WithTools` registers available tool descriptors.
- `WithConfig` overrides `DefaultConfig()`.
- `WithExperienceProvider` enables planning-time and step-time experience
  retrieval.
- `WithHooks` emits runtime lifecycle events.
- `WithPromptProvider` customizes prompts.
- `WithToolPolicy` replaces the default risk policy.
- `WithClock` and `WithIDGenerator` are mainly for tests.

### Running a Task

```go
result, err := rt.Run(ctx, agentruntime.TaskInput{
	TaskID:    "optional-stable-id",
	UserInput: "user request",
	UserContext: map[string]any{
		"tenant": "demo",
	},
	InitialExperience: []agentruntime.ExperienceItem{},
	Metadata:          map[string]string{"request_id": "req-123"},
	ConfigPatch:       nil,
})
```

`UserInput` is required. `TaskID` is optional; the runtime generates one when it
is empty.

### Interrupting a Task

```go
err := rt.Interrupt(taskID, "user cancelled")
```

`Interrupt` marks the task as interrupted and cancels the active context. It only
works for tasks currently running in this process.

### Updating Runtime Config

```go
maxTurns := 20
err := rt.UpdateConfig(taskID, agentruntime.ConfigPatch{
	MaxTotalTurns: &maxTurns,
})
```

Config updates are applied to the active task context and recorded in
`TaskResult.ConfigChanges`. The runtime rejects invalid patches, such as lowering
limits below already consumed counters.

## Adapter Interfaces

### LLMClient

```go
type LLMClient interface {
	Complete(ctx context.Context, req agentruntime.LLMRequest) (agentruntime.LLMResponse, error)
}
```

The runtime sets `req.Purpose` to one of:

- `plan`
- `react`
- `audit`
- `reflect`
- `correct`
- `summarize`

Some calls also set `req.ResponseSchema`, for example `plan_generation` or
`final_summary`. The adapter should return JSON content that matches the prompt
contract for structured steps.

### ToolGateway

```go
type ToolGateway interface {
	Call(ctx context.Context, req agentruntime.ToolRequest) (agentruntime.ToolResponse, error)
	Cancel(ctx context.Context, taskID string, callID string) error
}
```

The gateway is responsible for executing real tools, enforcing external
timeouts if needed, and returning a concise `Summary` plus detailed `Content`.

### ExperienceProvider

```go
type ExperienceProvider interface {
	Fetch(ctx context.Context, req agentruntime.ExperienceRequest) (agentruntime.ExperienceResponse, error)
}
```

Experience is used during planning and can also be requested by the ReAct loop.
The result records usage in `TaskResult.ExperienceUsage`.

### HookSink

```go
type HookSink interface {
	Handle(ctx context.Context, event agentruntime.HookEvent) error
}
```

Hooks are useful for logs, metrics, tracing, UI progress, and persistence.
When `FailOnHookError` is true, a hook failure on task finish can mark the task
as failed with `system_error`.

## Tool Descriptors

Tools are registered with `ToolDescriptor`:

```go
agentruntime.ToolDescriptor{
	Name:             "read_file",
	Description:      "Read a text file from the workspace.",
	ArgsSchema:       map[string]any{...},
	ResultSchema:     map[string]any{...},
	RiskLevel:        agentruntime.RiskReadOnly,
	AutoCallable:     true,
	RequiresApproval: false,
	DefaultTimeout:   30 * time.Second,
	Idempotent:       true,
	TypicalFailures:  []string{"file not found", "permission denied"},
	Tags:             []string{"filesystem"},
}
```

Supported argument schema checks include:

- object `required`
- primitive `type`
- string/number/integer/boolean/object/array checks
- `enum`
- `additionalProperties: false`
- nested object properties
- array item schemas
- numeric `minimum` and `maximum`

Risk levels:

- `read_only`
- `low`
- `high`
- `dangerous`

By default, high-risk and dangerous tools are denied unless enabled in config or
allowed by a custom `ToolPolicy`.

## Configuration

Start with:

```go
cfg := agentruntime.DefaultConfig()
cfg.MaxTotalTurns = 40
cfg.MaxPlanSteps = 8
cfg.MaxStepReactTurns = 6
cfg.TaskTimeout = 10 * time.Minute
cfg.ModelTimeout = 60 * time.Second
cfg.ToolTimeout = 60 * time.Second
```

Important switches:

- `EnableReflection`
- `EnableAudit`
- `EnableCorrection`
- `EnableExperience`
- `AllowDynamicNewSteps`
- `AllowSkipFailedStep`
- `AllowBestEffortAnswer`
- `AllowHighRiskTools`
- `AllowDangerousTools`
- `DisabledTools`
- `FailOnHookError`

Always call `cfg.Validate()` if you construct config manually before passing it
to `WithConfig`.

## Result Shape

`TaskResult` is designed for storage, debugging, and UI rendering. It includes:

- `Status`, `ExitReason`, and `FinalAnswer`
- `Completion` summary
- `InitialPlan` and `FinalPlan`
- `StepExecutions`
- `ToolCalls`
- `ModelCalls`
- `Errors`
- `Reflections`
- `Audits`
- `Corrections`
- `ExperienceUsage`
- `ConfigChanges`
- `Metrics`
- `StartedAt` and `EndedAt`

Use `result.ToJSON()` when you need an indented JSON representation.
