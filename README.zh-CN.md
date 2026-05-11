# Agent Runtime

[English](README.md) | 简体中文

Agent Runtime 是一个用 Go 编写的、可嵌入式的 AI Agent 运行时库。它负责把用户输入转换为结构化执行计划，按步骤执行 ReAct 循环，调用外部工具，记录模型调用和工具调用轨迹，并最终返回完整的 `TaskResult`。

这个项目不是一个独立服务，而是一个可被业务系统集成的核心运行时。业务侧需要自行提供 LLM 适配器、工具执行适配器、Hook、Prompt、经验库等外部能力。

## 当前定位

公共入口位于根包：

```go
rt, err := agentruntime.New(...)
result, err := rt.Run(ctx, agentruntime.TaskInput{...})
```

设计上参考了 LangGraph 和 AutoGen 的部分思想：

- 类 LangGraph 的任务状态管理：任务上下文、计划版本、步骤状态、退出原因、中断控制、运行时配置变更。
- 类 AutoGen 的多角色协作：Planner、ReAct Executor、Auditor、Reflector、Corrector 通过结构化模型输出和工具调用协同完成任务。

当前代码没有依赖 LangGraph 或 AutoGen，也没有复制它们的代码，整体实现为原生 Go 版本。

## 能力范围

- 根据 `TaskInput.UserInput` 生成执行计划
- 校验计划步骤数量、依赖关系、风险等级、工具可用性
- 每个步骤使用 ReAct 循环执行
- 支持工具调用、观察结果、模型输出解析、进度判断、经验请求
- 支持工具注册、参数 Schema 校验、风险策略控制
- 支持失败后的 Reflection、周期性 Audit、计划 Correction
- 支持最终 LLM 总结，并在模型失败时提供本地兜底总结
- 支持 `Runtime.Interrupt` 中断正在运行的任务
- 支持 `Runtime.UpdateConfig` 修改运行中任务的配置
- 支持 Hook 事件，用于日志、指标、追踪、UI 进度和持久化
- 返回结构化 `TaskResult`，包含计划、步骤执行、模型调用、工具调用、错误、审计、反思、修正、经验使用、配置变更和指标

## 不包含的能力

- 不提供 HTTP 服务、CLI、数据库、队列、Web UI
- 不内置任何 LLM 厂商客户端
- 不内置生产级工具执行器
- 不提供分布式调度或持久化恢复机制
- 不提供人工审批界面，但工具策略可以返回需要审批的决策

这些能力应由集成 Agent Runtime 的上层应用实现。

## 运行流程

```text
TaskInput
  -> 加载初始经验和经验库内容
  -> 调用 LLM 生成计划
  -> 校验计划
  -> 按步骤执行 ReAct 循环
  -> 在策略允许时通过 ToolGateway 调用工具
  -> 必要时请求更多经验
  -> 按配置执行 Audit、Reflection、Correction
  -> 生成最终总结
  -> 返回 TaskResult
```

## 安装与模块

通过 `go get` 安装：

```bash
go get github.com/chenchen511/agent-runtime
```

当前模块路径为：

```text
module github.com/chenchen511/agent-runtime
```

## 快速开始

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

其中 `myLLMClient` 需要实现 `agentruntime.LLMClient`，`myToolGateway` 需要实现 `agentruntime.ToolGateway`。

## 核心 API

### 创建 Runtime

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

必填项：

- `WithLLMClient`：所有任务都需要模型客户端。
- `WithToolGateway`：注册了工具时必须提供工具网关。

可选项：

- `WithTools`：注册可用工具描述。
- `WithConfig`：覆盖 `DefaultConfig()`。
- `WithExperienceProvider`：开启规划阶段和执行阶段的经验检索。
- `WithHooks`：注册运行时生命周期事件接收器。
- `WithPromptProvider`：自定义 Prompt。
- `WithToolPolicy`：替换默认工具风险策略。
- `WithClock`、`WithIDGenerator`：主要用于测试。

### 运行任务

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

`UserInput` 必填。`TaskID` 可选，不传时由运行时自动生成。

### 中断任务

```go
err := rt.Interrupt(taskID, "user cancelled")
```

`Interrupt` 会把任务标记为中断，并取消当前上下文。它只对当前进程内正在运行的任务有效。

### 修改运行中配置

```go
maxTurns := 20
err := rt.UpdateConfig(taskID, agentruntime.ConfigPatch{
	MaxTotalTurns: &maxTurns,
})
```

配置变更会应用到正在运行的任务上下文，并记录到 `TaskResult.ConfigChanges`。如果 Patch 非法，例如把限制值调低到已消耗计数以下，运行时会拒绝变更。

## 适配器接口

### LLMClient

```go
type LLMClient interface {
	Complete(ctx context.Context, req agentruntime.LLMRequest) (agentruntime.LLMResponse, error)
}
```

运行时会通过 `req.Purpose` 标识调用目的：

- `plan`：生成计划
- `react`：执行步骤
- `audit`：审计当前执行是否偏离目标
- `reflect`：分析失败原因和恢复策略
- `correct`：生成计划修正
- `summarize`：生成最终总结

部分调用会设置 `req.ResponseSchema`，例如 `plan_generation`、`final_summary`。LLM 适配器应根据 Prompt 契约返回可解析的 JSON 内容。

### ToolGateway

```go
type ToolGateway interface {
	Call(ctx context.Context, req agentruntime.ToolRequest) (agentruntime.ToolResponse, error)
	Cancel(ctx context.Context, taskID string, callID string) error
}
```

工具网关负责真正执行工具、处理外部超时、返回简短 `Summary` 和详细 `Content`。运行时只负责决策、校验、调用、记录，不直接实现具体业务副作用。

### ExperienceProvider

```go
type ExperienceProvider interface {
	Fetch(ctx context.Context, req agentruntime.ExperienceRequest) (agentruntime.ExperienceResponse, error)
}
```

经验会在规划阶段使用，ReAct 执行过程中也可以主动请求更多经验。最终使用情况记录在 `TaskResult.ExperienceUsage`。

### HookSink

```go
type HookSink interface {
	Handle(ctx context.Context, event agentruntime.HookEvent) error
}
```

Hook 适合用于日志、指标、链路追踪、UI 进度更新和结果持久化。当 `FailOnHookError` 为 true 时，任务结束阶段的 Hook 错误可能使任务以 `system_error` 失败。

## 工具描述

通过 `ToolDescriptor` 注册工具：

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

当前参数 Schema 支持：

- object `required`
- 基础 `type`
- string、number、integer、boolean、object、array 类型检查
- `enum`
- `additionalProperties: false`
- 嵌套 object properties
- array item schemas
- 数值 `minimum` 和 `maximum`

工具风险等级：

- `read_only`
- `low`
- `high`
- `dangerous`

默认策略会拒绝高风险和危险工具，除非通过配置显式允许，或通过自定义 `ToolPolicy` 放行。

## 配置

建议从默认配置开始修改：

```go
cfg := agentruntime.DefaultConfig()
cfg.MaxTotalTurns = 40
cfg.MaxPlanSteps = 8
cfg.MaxStepReactTurns = 6
cfg.TaskTimeout = 10 * time.Minute
cfg.ModelTimeout = 60 * time.Second
cfg.ToolTimeout = 60 * time.Second
```

常用开关：

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

如果手动构造配置，传给 `WithConfig` 前建议先调用 `cfg.Validate()`。

## 返回结果

`TaskResult` 面向存储、调试、审计和 UI 展示设计，主要字段包括：

- `Status`、`ExitReason`、`FinalAnswer`
- `Completion`
- `InitialPlan`、`FinalPlan`
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
- `StartedAt`、`EndedAt`

需要 JSON 输出时可以使用：

```go
data, err := result.ToJSON()
```

## 目录结构

```text
.
├── runtime.go        # 公共运行时编排入口
├── config.go         # 公共配置别名和默认值
├── interfaces.go     # 公共适配器接口
├── result.go         # 公共结果类型别名
├── core/             # 共享领域类型
├── task/             # 可变任务上下文和计数器
├── planner/          # 计划生成与解析
├── plan/             # 计划管理与校验
├── executor/         # ReAct 步骤执行
├── tool/             # 工具注册、网关包装、策略、Schema 校验
├── audit/            # 周期性执行审计
├── reflection/       # 失败反思与恢复建议
├── correction/       # 计划修正
├── experience/       # 经验提供器辅助逻辑
├── hook/             # Hook 管理器
├── exit/             # 退出决策逻辑
└── docs/             # 设计文档和开发提示词
```

## 开发与测试

运行全部测试：

```bash
env GOCACHE=/tmp/go-cache go test ./...
```

运行竞态检测：

```bash
env GOCACHE=/tmp/go-cache go test -race ./...
```

运行静态检查：

```bash
env GOCACHE=/tmp/go-cache go vet ./...
```

查看覆盖率：

```bash
env GOCACHE=/tmp/go-cache go test -cover ./...
```

如果本机 Go 构建缓存目录可写，也可以直接使用 `go test ./...`。

## 后续开发建议

- 根包保持稳定，只暴露嵌入式集成真正需要的 API。
- 新能力优先挂到已有接口后面，避免让运行时依赖具体业务服务。
- 模型输出解析必须严格，并补充单元测试。
- 工具执行保持在运行时之外。运行时负责决策、校验、观察和记录，业务应用负责具体副作用。
- 每新增一种退出路径、策略分支、解析结构、配置变更，都应补对应测试。
