# CLAUDE.md

## Project Overview

AgentRuntime is a Go SDK that provides a reusable Agent execution kernel. It handles task planning, ReAct execution, tool orchestration, error reflection, periodic auditing, plan correction, exit control, and structured result generation.

**This is a library/SDK, not a service.** It has no HTTP server, no database, no UI, and no real tool execution. All external capabilities are injected via interfaces.

## Design Documents

- `agent-runtime.md` — Full design specification
- `agent-runtime.optimized.md` — Optimized design specification (primary reference)
- `development-prompt.md` — Development prompt with phased implementation guide

## Module Structure

```
agentruntime/          # Root package — public API: Runtime, New, Run, Interrupt, UpdateConfig
├── task/              # TaskContext, TaskInput, TaskSnapshot, Counters
├── plan/              # Plan, PlanStep, PlanManager, PlanValidator, PlanDiff
├── planner/           # Planner, plan generation prompt and parser
├── executor/          # ReActExecutor, StepRunner, ActionParser, Observation
├── tool/              # ToolRegistry, ToolDescriptor, ToolPolicy, gateway wrapper
├── llm/               # LLMClient interface types, messages, request/response
├── experience/        # ExperienceProvider interface types
├── reflection/        # Reflector, reflection result and prompt
├── audit/             # Auditor, audit result, policy, trigger
├── correction/        # PlanCorrector, correction result, validator
├── exit/              # ExitController, exit reasons, decisions
├── hook/              # HookManager, HookEvent, HookSink
├── apperr/            # RuntimeError, ErrorKind, ErrorManager
└── internal/          # Non-exported utilities: clock, ids, limiter, textutil
```

## Build and Test

```bash
# Run all tests
cd /code/agent-runtime && go test ./...

# Run tests with race detector
cd /code/agent-runtime && go test -race ./...

# Run a specific package's tests
cd /code/agent-runtime && go test ./plan/...

# Run a specific test
cd /code/agent-runtime && go test -run TestPlanValidator ./plan/...

# Check compilation
cd /code/agent-runtime && go build ./...
```

## Key Design Principles

1. **SDK boundary**: No HTTP, no database, no real tool execution, no model vendor binding
2. **All external via interfaces**: LLMClient, ToolGateway, ExperienceProvider, HookSink, PromptProvider, ToolPolicy
3. **Serial by default**: Steps execute serially, tool calls execute serially within steps
4. **Bounded execution**: All loops, tool calls, audits, reflections, corrections have upper limits
5. **History immutability**: Plan correction can only modify unexecuted steps; completed/failed/skipped history is preserved
6. **Structured output**: All model output goes through parser + validator; never trust raw natural language
7. **Tool safety**: Tool calls go through registry lookup, param validation, risk policy, call limits, and timeout
8. **Result on failure**: Even on failure/interrupt/limit, return a structured TaskResult with status and exit reason

## Conventions

- Package names are short and don't conflict with stdlib (`apperr` not `errors`, `task` not `context`)
- All public types and functions have doc comments
- Errors carry context: stage, kind, task ID, step ID, action taken
- Tests use fake adapters (FakeLLMClient, FakeToolGateway, RecordingHookSink)
- Never call real models or real tools in tests
- Time uses `time.Time` (RFC3339 for JSON serialization)
- IDs are generated via injectable IDGenerator (default: UUID-like strings)

## Development Phases

1. **Phase 1**: Basic skeleton — config, types, interfaces, Runtime.New
2. **Phase 2**: Plan generation — Planner, PlanValidator, PlanManager
3. **Phase 3**: ReAct + tools — ReActExecutor, ToolRegistry, ToolGateway wrapper
4. **Phase 4**: Exit, errors, results — ExitController, ErrorManager, ResultBuilder
5. **Phase 5**: Audit, reflection, correction — Reflector, Auditor, PlanCorrector
6. **Phase 6**: Tests, docs, release — Fake adapters, scenario tests, README
