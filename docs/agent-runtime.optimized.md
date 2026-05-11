# AgentRuntime SDK 程序设计文档（优化版）

版本：v0.2  
日期：2026-05-11  
适用语言：Go  
交付形态：SDK 依赖包  

## 1. 文档目标

本文档定义一个基于 Go 实现的 `AgentRuntime SDK` 的程序设计方案。它面向后续开发落地，重点说明：

- SDK 的边界、模块、包结构和公共接口。
- Agent 任务从接收到结束的完整生命周期。
- 计划生成、ReAct 执行、工具调用、审计、反思、纠偏和退出控制的协作方式。
- 关键数据结构、状态机、错误模型、Hook 事件和可测试契约。
- MVP 开发范围、实现顺序、测试策略和验收标准。

该 SDK 不提供独立服务端、不提供前端、不内置数据库、不真实执行工具。它只提供可复用的 Agent 执行内核，由调用方接入模型、工具、存储、事件推送和业务权限。

## 2. 设计定位

### 2.1 SDK 要解决的问题

Agent 运行时必须解决以下工程问题：

- 任务需要先规划再执行，避免直接进入不可控工具调用。
- Agent 不能无限循环，必须有总轮数、步骤轮数、工具次数、失败次数等限制。
- 每个计划步骤需要有明确状态、输入、输出、错误和执行记录。
- 模型输出必须可解析、可校验，不能让自然语言直接驱动危险动作。
- 工具由外部进程执行，SDK 只负责统一工具调用语义、超时、取消和结果归档。
- 执行过程需要可中断、可审计、可反思、可纠偏。
- 计划纠偏只能修改未来，不得篡改历史执行记录。
- 最终结果必须完整、结构化、可持久化。

### 2.2 SDK 不解决的问题

以下能力不属于 SDK 范围：

- HTTP Server、gRPC Server、SSE、WebSocket。
- Web UI、CLI 交互界面。
- 数据库、对象存储、向量库、RAG 系统。
- 工具真实执行环境、容器沙箱、插件市场。
- 用户认证、权限系统、多租户。
- 分布式任务调度、任务队列、任务恢复集群。
- 具体模型厂商封装。
- 业务安全审批流。

调用方可以通过 Adapter、Hook 和最终结果持久化来实现这些能力。

### 2.3 核心原则

- SDK 只做运行时内核，不绑定业务系统。
- 所有外部能力都通过接口注入。
- 默认串行执行，先保证可控性和可审计性。
- 所有状态变更必须有记录，最终结果可回放主要执行过程。
- 模型输出必须结构化解析，解析失败进入错误策略。
- 工具调用前必须经过工具注册、参数校验、风险策略和次数限制。
- 退出控制优先级高于计划执行，任何关键点都必须检查退出条件。
- 反思和纠偏是恢复机制，不允许变成新的无限循环源。

## 3. 术语定义

| 术语 | 含义 |
| --- | --- |
| Runtime | SDK 的主控对象，负责创建任务、调度步骤、检查退出条件和构造结果。 |
| Task | 一次用户任务执行实例。 |
| TaskContext | 单个任务的内存状态中心。 |
| Plan | Planner 生成的结构化执行计划。 |
| Step | Plan 中的一个可执行步骤。 |
| ReAct | 单步骤内部的分析、工具调用、观察、继续推理循环。 |
| Tool | 外部工具能力，SDK 不真实执行，只通过 ToolGateway 调用。 |
| Observation | 工具执行结果进入模型上下文后的结构化观察。 |
| Audit | 阶段性审计，用于判断执行是否偏离用户目标。 |
| Reflection | 错误反思，用于分析失败原因并给出恢复建议。 |
| Correction | 计划纠偏，用于修改后续未执行步骤。 |
| Hook | 生命周期事件回调。 |
| Final Result | 任务结束时返回给调用方的结构化结果。 |

## 4. 总体架构

### 4.1 分层架构

```text
调用方业务系统
  |  注入 LLMClient / ToolGateway / ExperienceProvider / Hook / Policy
  v
AgentRuntime SDK
  |  统一任务生命周期、状态机、限制器、错误处理、结果构造
  v
外部适配器
  |  模型服务、工具进程、历史经验系统、持久化系统、事件系统
  v
外部基础设施
```

SDK 内部模块如下：

```text
AgentRuntime SDK
├── Runtime Engine          运行时主控
├── Task Context            任务状态中心
├── Config Manager          配置和动态配置补丁
├── Planner                 初始计划生成
├── Plan Manager            计划状态维护
├── ReAct Executor          单步骤执行器
├── Tool Registry           工具描述注册表
├── Tool Gateway            外部工具通信抽象
├── LLM Adapter             大模型调用抽象
├── Experience Provider     历史经验抽象
├── Reflector               错误反思
├── Auditor                 阶段性审计
├── Plan Corrector          计划纠偏
├── Exit Controller         退出控制
├── Hook Manager            生命周期事件
├── Result Builder          最终结果构造
├── Error Manager           错误分类和处理策略
└── Limiter                 轮数、次数、时间、无进展限制
```

### 4.2 核心数据流

```text
TaskInput
  -> Runtime 初始化 TaskContext
  -> Config 校验和默认值补齐
  -> ToolRegistry 快照
  -> ExperienceProvider 加载初始经验摘要
  -> Planner 生成初始计划
  -> PlanValidator 校验计划
  -> PlanManager 保存计划
  -> Runtime 调度下一个 Step
  -> ReActExecutor 执行 Step
  -> LLMClient 产生结构化动作
  -> ToolGateway 调用外部工具
  -> Observation 写回步骤上下文
  -> Step 完成、失败或跳过
  -> PlanManager 更新计划状态
  -> Auditor 按策略审计
  -> Reflector 按错误策略反思
  -> PlanCorrector 按审计或反思结果纠偏
  -> ExitController 判断是否结束
  -> ResultBuilder 构造 TaskResult
```

### 4.3 Runtime 和 Adapter 边界

| 能力 | SDK 内部负责 | 调用方负责 |
| --- | --- | --- |
| 模型调用 | 定义 `LLMClient` 接口、构造请求、解析响应、记录摘要 | 实现具体模型请求、鉴权、重试细节、限流 |
| 工具调用 | 定义 `ToolGateway` 接口、校验工具描述、生成请求、处理超时和取消意图 | 执行真实工具、隔离权限、返回结果 |
| 历史经验 | 定义 `ExperienceProvider` 接口、记录使用情况 | 检索经验、压缩摘要、持久化经验 |
| 持久化 | 产出可序列化结果、触发 Hook | 写数据库、对象存储、日志系统 |
| 事件推送 | 定义 Hook 事件 | 推送到前端、消息队列、监控系统 |
| 权限审批 | 提供风险等级和可选 Policy 接口 | 决定是否允许高风险工具 |

## 5. 推荐包结构

MVP 建议使用以下包结构。包名尽量短，但不要与标准库 `context`、`errors` 混淆。

```text
agentruntime/
├── runtime.go                 # Runtime 入口、Run、Interrupt、UpdateConfig
├── options.go                 # Option 模式
├── config.go                  # RuntimeConfig、默认值、校验
├── interfaces.go              # 外部 Adapter 接口
├── types.go                   # 通用 ID、时间、状态枚举
├── result.go                  # TaskResult 和结果构造入口
├── version.go
│
├── task/
│   ├── context.go             # TaskContext
│   ├── input.go               # TaskInput
│   ├── snapshot.go            # 给 Hook、审计、总结使用的快照
│   └── counters.go            # 运行计数器
│
├── plan/
│   ├── plan.go                # Plan、PlanStep、Dependency
│   ├── manager.go             # PlanManager
│   ├── validator.go           # PlanValidator
│   ├── diff.go                # 纠偏前后差异
│   └── status.go
│
├── planner/
│   ├── planner.go             # Planner
│   ├── prompt.go
│   └── parser.go
│
├── executor/
│   ├── react.go               # ReActExecutor
│   ├── step_runner.go         # StepRunner
│   ├── action.go              # 模型动作类型
│   ├── observation.go
│   ├── progress.go            # 无进展判断
│   └── parser.go
│
├── tool/
│   ├── registry.go            # ToolRegistry
│   ├── descriptor.go          # ToolDescriptor
│   ├── request.go             # ToolRequest
│   ├── response.go            # ToolResponse
│   ├── policy.go              # 风险和审批策略
│   └── gateway.go             # ToolGateway 包装层
│
├── llm/
│   ├── client.go              # LLMClient 接口补充类型
│   ├── message.go
│   ├── request.go
│   ├── response.go
│   └── schema.go              # 结构化输出 schema 名称和版本
│
├── experience/
│   ├── provider.go
│   ├── request.go
│   ├── response.go
│   └── usage.go
│
├── reflection/
│   ├── reflector.go
│   ├── result.go
│   └── prompt.go
│
├── audit/
│   ├── auditor.go
│   ├── result.go
│   ├── policy.go
│   └── trigger.go
│
├── correction/
│   ├── corrector.go
│   ├── result.go
│   ├── validator.go
│   └── prompt.go
│
├── exit/
│   ├── controller.go
│   ├── reason.go
│   └── decision.go
│
├── hook/
│   ├── manager.go
│   ├── event.go
│   ├── sink.go
│   └── payload.go
│
├── apperr/
│   ├── error.go
│   ├── kind.go
│   ├── retry.go
│   └── manager.go
│
└── internal/
    ├── clock/
    ├── ids/
    ├── limiter/
    ├── queue/
    ├── schema/
    └── textutil/
```

包设计注意事项：

- `agentruntime` 根包暴露稳定 API。
- 子包暴露必要类型，但尽量避免让调用方依赖内部执行细节。
- `internal/` 只放不可被外部导入的工具能力。
- `TaskContext` 不建议直接暴露可变指针，Hook 和 Adapter 使用只读快照。

## 6. 公共 API 设计

### 6.1 Runtime 创建

```go
package agentruntime

type Runtime struct {
    // 内部字段不导出
}

func New(opts ...Option) (*Runtime, error)
```

必需 Option：

```go
func WithLLMClient(client LLMClient) Option
func WithToolGateway(gateway ToolGateway) Option
func WithTools(tools []ToolDescriptor) Option
```

可选 Option：

```go
func WithConfig(config RuntimeConfig) Option
func WithExperienceProvider(provider ExperienceProvider) Option
func WithHooks(sinks ...HookSink) Option
func WithPromptProvider(provider PromptProvider) Option
func WithToolPolicy(policy ToolPolicy) Option
func WithClock(clock Clock) Option
func WithIDGenerator(generator IDGenerator) Option
```

创建规则：

- `LLMClient` 必填。
- `ToolGateway` 在允许工具调用时必填。
- `ToolDescriptor` 可以为空，但为空时 Planner 和 ReAct 不能生成工具调用。
- `RuntimeConfig` 为空时使用默认配置。
- `Runtime` 可以复用执行多个任务，但每个任务必须创建独立 `TaskContext`。

### 6.2 执行任务

```go
func (r *Runtime) Run(ctx context.Context, input TaskInput) (*TaskResult, error)
```

返回约定：

- 业务执行失败、超限、中断时，优先返回非空 `TaskResult`，其中包含 `Status` 和 `ExitReason`。
- 只有 SDK 无法构造结果、配置非法、依赖缺失等启动级错误，才返回非空 `error`。
- 当 `error != nil` 时，调用方不应假设任务已经进入可持久化执行状态。

### 6.3 中断任务

```go
func (r *Runtime) Interrupt(taskID string, reason string) error
```

中断语义：

- 只对当前进程内正在执行的任务有效。
- 设置任务中断标记。
- 阻止新的模型调用和工具调用。
- 尝试通过 `ToolGateway.Cancel` 取消当前工具请求。
- 最终仍由 `Run` 返回 `TaskResult`。

### 6.4 动态配置更新

```go
func (r *Runtime) UpdateConfig(taskID string, patch ConfigPatch) error
```

允许动态修改：

- 超时时间。
- 最大轮数和最大工具调用次数，但不能低于已经发生的计数。
- 是否启用审计、反思、纠偏。
- 禁用工具列表。
- 是否允许继续执行。

不允许动态修改：

- `TaskID`。
- 原始用户输入。
- 已完成步骤、已失败步骤、历史工具调用。
- 已生成的审计、反思、纠偏记录。

## 7. 外部接口设计

### 7.1 LLMClient

```go
type LLMClient interface {
    Complete(ctx context.Context, req LLMRequest) (LLMResponse, error)
}
```

`LLMRequest` 字段：

| 字段 | 说明 |
| --- | --- |
| `TaskID` | 任务 ID。 |
| `StepID` | 可选，当前步骤 ID。 |
| `Purpose` | `plan`、`react`、`audit`、`reflect`、`correct`、`summarize`。 |
| `Messages` | 模型消息。 |
| `ResponseSchema` | 期望的结构化响应 schema 名称和版本。 |
| `Temperature` | 可选采样参数。 |
| `Timeout` | 调用超时。 |
| `Metadata` | 调用方透传元数据。 |

`LLMResponse` 字段：

| 字段 | 说明 |
| --- | --- |
| `Content` | 原始文本或 JSON 字符串。 |
| `Parsed` | 可选，适配器已解析后的结构。 |
| `Model` | 实际模型名称。 |
| `Usage` | token 用量摘要。 |
| `Latency` | 耗时。 |
| `Raw` | 可选原始响应摘要，不建议保存敏感全量内容。 |

SDK 约束：

- SDK 不要求暴露模型隐藏推理。
- 记录模型调用时保存输入摘要、输出摘要、schema、耗时和错误，不默认保存完整 Prompt。
- 所有模型输出必须经过目的相关 parser 校验。

### 7.2 ToolGateway

```go
type ToolGateway interface {
    Call(ctx context.Context, req ToolRequest) (ToolResponse, error)
    Cancel(ctx context.Context, taskID string, callID string) error
}
```

`ToolRequest` 字段：

| 字段 | 说明 |
| --- | --- |
| `CallID` | 工具调用唯一 ID。 |
| `TaskID` | 任务 ID。 |
| `StepID` | 步骤 ID。 |
| `ToolName` | 工具名称。 |
| `Reason` | 调用原因。 |
| `Args` | JSON 对象参数。 |
| `Timeout` | 单次调用超时。 |
| `RiskLevel` | 工具风险等级。 |
| `Cancelable` | 是否期望支持取消。 |
| `Context` | 只读上下文摘要，不传可变状态。 |

`ToolResponse` 字段：

| 字段 | 说明 |
| --- | --- |
| `CallID` | 对应请求 ID。 |
| `ToolName` | 工具名称。 |
| `Status` | `success`、`failed`、`timeout`、`cancelled`。 |
| `Content` | 结构化工具结果，建议 JSON。 |
| `Summary` | 给模型和结果记录使用的摘要。 |
| `ErrorMessage` | 失败原因。 |
| `StartedAt` / `EndedAt` | 开始和结束时间。 |
| `Metadata` | 调用方透传元数据。 |

工具调用约束：

- SDK 不拼接 shell 命令，不解释工具参数的业务含义。
- SDK 必须在调用前确认工具已注册。
- SDK 必须在调用前执行参数 schema 校验。
- 高风险工具是否允许自动调用由 `ToolPolicy` 决定。
- 工具超时后记录状态为 `timeout`，并触发错误策略。

### 7.3 ExperienceProvider

```go
type ExperienceProvider interface {
    Fetch(ctx context.Context, req ExperienceRequest) (ExperienceResponse, error)
}
```

使用场景：

- 任务开始前，根据用户目标加载初始经验摘要。
- 执行中，根据审计、反思或 ReAct 请求补充经验。
- 任务结束后，调用方可以从 `TaskResult` 中提取可复用经验自行保存。

MVP 建议只实现任务开始前经验摘要，执行中请求预留接口。

### 7.4 HookSink

```go
type HookSink interface {
    Handle(ctx context.Context, event HookEvent) error
}
```

Hook 设计原则：

- 默认异步执行，不阻塞 Runtime 主流程。
- 可配置关键事件同步执行，例如安全审批或强制审计。
- Hook 失败默认只记录，不导致任务失败。
- 如果配置 `FailOnHookError`，同步 Hook 失败可使任务失败。
- Hook 接收的是快照，不允许直接修改 `TaskContext`。

### 7.5 PromptProvider

```go
type PromptProvider interface {
    Build(ctx context.Context, req PromptRequest) (PromptBundle, error)
}
```

用途：

- 允许调用方替换计划、执行、审计、反思、纠偏、总结 Prompt。
- 保持 SDK 默认 Prompt 可用。
- PromptProvider 只负责构造消息，不负责调用模型。

## 8. 配置设计

### 8.1 RuntimeConfig

```go
type RuntimeConfig struct {
    MaxTotalTurns          int
    MaxPlanSteps           int
    MaxStepReactTurns      int
    MaxToolCalls           int
    MaxToolCallsPerStep    int
    MaxToolFailures        int
    MaxModelFailures       int
    MaxParseFailures       int
    MaxNoProgressTurns     int

    TaskTimeout            time.Duration
    ModelTimeout           time.Duration
    ToolTimeout            time.Duration
    HookTimeout            time.Duration

    EnableReflection       bool
    EnableAudit            bool
    EnableCorrection       bool
    EnableExperience       bool

    AuditEveryNSteps       int
    AuditEveryNTurns       int
    MaxAudits              int
    MaxCorrections         int
    MaxReflections         int

    AllowDynamicNewSteps   bool
    AllowSkipFailedStep    bool
    AllowBestEffortAnswer  bool
    AllowHighRiskTools     bool
    AllowDangerousTools    bool

    DisabledTools          []string
    FailOnHookError        bool
}
```

### 8.2 默认配置建议

| 配置项 | 默认值 | 说明 |
| --- | --- | --- |
| `MaxTotalTurns` | 40 | 任务总 ReAct 轮数上限。 |
| `MaxPlanSteps` | 8 | 初始计划最多步骤数。 |
| `MaxStepReactTurns` | 6 | 单步骤内部最大 ReAct 轮数。 |
| `MaxToolCalls` | 20 | 单任务最大工具调用次数。 |
| `MaxToolCallsPerStep` | 4 | 单步骤最大工具调用次数。 |
| `MaxToolFailures` | 5 | 单任务最大工具失败次数。 |
| `MaxParseFailures` | 3 | 连续模型解析失败上限。 |
| `MaxNoProgressTurns` | 3 | 连续无进展上限。 |
| `TaskTimeout` | 10 分钟 | 单任务最大运行时间。 |
| `ModelTimeout` | 60 秒 | 单次模型调用超时。 |
| `ToolTimeout` | 60 秒 | 单次工具调用默认超时。 |
| `EnableReflection` | true | MVP 可开启基础反思。 |
| `EnableAudit` | true | MVP 按步骤数量触发。 |
| `EnableCorrection` | true | 只纠正未执行步骤。 |
| `AuditEveryNSteps` | 3 | 每完成 3 个步骤审计一次。 |
| `MaxAudits` | 5 | 防止审计循环。 |
| `MaxCorrections` | 3 | 防止纠偏循环。 |
| `AllowBestEffortAnswer` | true | 超限或部分失败时允许总结已有结果。 |

### 8.3 配置校验规则

- 所有最大次数必须大于 0，除非该能力被显式禁用。
- `ToolTimeout`、`ModelTimeout`、`TaskTimeout` 必须大于 0。
- `AuditEveryNSteps` 或 `AuditEveryNTurns` 至少一个有效，否则审计只在最终总结前触发。
- 禁用 `EnableCorrection` 时，审计和反思只能建议继续、跳过、总结或失败退出。
- 允许危险工具前必须显式设置 `AllowDangerousTools`。
- `MaxPlanSteps` 不能小于 1。

## 9. 核心数据模型

### 9.1 TaskInput

```go
type TaskInput struct {
    TaskID              string
    UserInput           string
    UserContext         map[string]any
    InitialExperience   []ExperienceItem
    Metadata            map[string]string
    ConfigPatch         *ConfigPatch
}
```

规则：

- `TaskID` 为空时由 SDK 生成。
- `UserInput` 不能为空。
- `InitialExperience` 由调用方预先提供，SDK 不校验其业务真实性。
- `ConfigPatch` 只影响当前任务。

### 9.2 TaskContext

`TaskContext` 是单任务内存状态中心，不负责持久化。

主要字段：

```go
type TaskContext struct {
    TaskID          string
    Input           TaskInput
    Config          RuntimeConfig
    Status          TaskStatus
    ExitReason      ExitReason

    ToolSnapshot    []ToolDescriptor
    InitialPlan     *Plan
    CurrentPlan     *Plan
    CurrentStepID   string

    Counters        Counters
    Timeline        []TimelineEvent

    Steps           []StepExecution
    ToolCalls       []ToolCallRecord
    ModelCalls      []ModelCallRecord
    Errors          []RuntimeError
    Reflections     []ReflectionResult
    Audits          []AuditResult
    Corrections     []CorrectionResult
    ExperienceUsage []ExperienceUsage
    ConfigChanges   []ConfigChange

    Interrupted     bool
    InterruptReason string
    StartedAt       time.Time
    EndedAt         time.Time
}
```

并发规则：

- Runtime 主执行循环独占写入 `TaskContext`。
- Hook、Adapter、审计、总结只接收快照。
- `Interrupt` 和 `UpdateConfig` 通过受控方法修改中断标记和配置补丁。
- 不向外暴露可变 `TaskContext` 指针。

### 9.3 Plan

```go
type Plan struct {
    PlanID      string
    Version     int
    Goal        string
    Assumptions []string
    Steps       []PlanStep
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type PlanStep struct {
    StepID          string
    Title           string
    Objective       string
    ExpectedOutput  string
    SuggestedTools  []string
    Dependencies    []string
    Status          StepStatus
    RetryCount      int
    CreatedBy       string // planner, correction
    ChangeReason    string
    RiskLevel       RiskLevel
}
```

计划约束：

- `StepID` 在一个任务内唯一。
- `Dependencies` 只能引用同计划内步骤。
- 已执行步骤不能删除。
- 已完成、失败、跳过、被替换步骤保留在最终计划中。
- 纠偏产生新版本计划，必须记录差异。

### 9.4 StepExecution

```go
type StepExecution struct {
    StepID        string
    Attempt       int
    Status        StepStatus
    StartedAt     time.Time
    EndedAt       time.Time
    ReactTurns    []ReactTurn
    Result        string
    Evidence      []Evidence
    Error         *RuntimeError
    NoProgressHit bool
}
```

### 9.5 ReactTurn

```go
type ReactTurn struct {
    TurnIndex       int
    ModelCallID     string
    Action          ReactAction
    ToolCallID      string
    Observation     *Observation
    ParseError      *RuntimeError
    ProgressSummary string
    StartedAt       time.Time
    EndedAt         time.Time
}
```

`ProgressSummary` 保存简短进展摘要，不保存模型隐藏推理。

### 9.6 ReactAction

```go
type ReactAction struct {
    Type              ReactActionType
    Summary           string
    ToolName          string
    ToolArgs          map[string]any
    StepResult        string
    NeedsExperience   bool
    NeedsUserInput    bool
    UserInputQuestion string
}
```

动作类型：

| 类型 | 含义 |
| --- | --- |
| `tool_call` | 请求调用工具。 |
| `step_result` | 当前步骤完成。 |
| `request_experience` | 请求更多历史经验。 |
| `need_user_input` | 当前信息不足，需要调用方补充。MVP 可转为提前总结或失败。 |
| `fail_step` | 模型判断步骤无法继续。 |

### 9.7 TaskResult

```go
type TaskResult struct {
    TaskID          string
    Status          TaskStatus
    ExitReason      ExitReason
    FinalAnswer     string
    Completion      CompletionSummary

    InitialPlan     *Plan
    FinalPlan       *Plan
    StepExecutions  []StepExecution
    ToolCalls       []ToolCallRecord
    ModelCalls      []ModelCallRecord
    Errors          []RuntimeError
    Reflections     []ReflectionResult
    Audits          []AuditResult
    Corrections     []CorrectionResult
    ExperienceUsage []ExperienceUsage
    ConfigChanges   []ConfigChange
    Metrics         RuntimeMetrics

    StartedAt       time.Time
    EndedAt         time.Time
    Metadata        map[string]string
}
```

结果要求：

- 任务成功与否都返回结构化结果。
- `FinalAnswer` 只能基于已执行步骤、工具结果、经验摘要和明确的不确定性。
- 未执行内容不能描述为已完成。
- 工具失败、超时、中断、超限必须进入 `Errors` 和 `Completion`。

## 10. 状态机设计

### 10.1 TaskStatus

| 状态 | 说明 |
| --- | --- |
| `initializing` | 初始化任务上下文。 |
| `planning` | 生成初始计划。 |
| `plan_failed` | 初始计划生成或校验失败。 |
| `running` | 正在执行计划步骤。 |
| `waiting_tool` | 等待工具结果。 |
| `auditing` | 阶段性审计。 |
| `reflecting` | 错误反思。 |
| `correcting` | 计划纠偏。 |
| `summarizing` | 生成最终总结。 |
| `completed` | 正常完成。 |
| `failed` | 失败结束。 |
| `interrupted` | 用户或调用方中断。 |
| `limited` | 命中限制后结束。 |

### 10.2 TaskStatus 流转

正常路径：

```text
initializing -> planning -> running -> summarizing -> completed
```

审计路径：

```text
running -> auditing -> running
running -> auditing -> correcting -> running
running -> auditing -> summarizing -> completed|failed
```

错误恢复路径：

```text
running -> reflecting -> running
running -> reflecting -> correcting -> running
running -> reflecting -> summarizing -> failed|limited
```

中断路径：

```text
running|waiting_tool|auditing|reflecting|correcting -> summarizing -> interrupted
```

### 10.3 StepStatus

| 状态 | 说明 |
| --- | --- |
| `pending` | 待执行。 |
| `running` | 正在执行。 |
| `waiting_tool` | 当前步骤等待工具结果。 |
| `completed` | 步骤成功完成。 |
| `failed` | 步骤失败。 |
| `skipped` | 被策略跳过。 |
| `retrying` | 准备重试。 |
| `replaced` | 被纠偏替换，但历史保留。 |
| `invalidated` | 审计判定对目标无效。 |

### 10.4 ExitReason

| 退出原因 | 说明 |
| --- | --- |
| `normal_completed` | 所有必要步骤完成。 |
| `user_interrupted` | 调用方主动中断。 |
| `task_timeout` | 任务超时。 |
| `max_total_turns` | 总轮数超限。 |
| `max_tool_calls` | 工具调用次数超限。 |
| `max_tool_failures` | 工具失败次数超限。 |
| `max_model_failures` | 模型失败次数超限。 |
| `max_parse_failures` | 模型输出解析失败超限。 |
| `no_progress` | 连续无进展。 |
| `plan_generation_failed` | 计划生成失败。 |
| `plan_validation_failed` | 计划校验失败。 |
| `audit_unrecoverable` | 审计认为无法继续。 |
| `reflection_unrecoverable` | 反思认为无法恢复。 |
| `tool_unavailable` | 工具网关不可用或关键工具不可用。 |
| `model_unavailable` | 模型服务不可用。 |
| `system_error` | SDK 内部异常。 |

## 11. Runtime 主流程

### 11.1 总体流程

```text
Run(ctx, input)
  1. 校验 Runtime 依赖
  2. 构造 TaskContext
  3. 应用默认配置和任务级 ConfigPatch
  4. 复制 ToolRegistry 快照
  5. 触发 TaskStarted Hook
  6. 加载初始 Experience
  7. 调用 Planner 生成初始计划
  8. 校验计划并保存
  9. 触发 PlanCreated Hook
 10. 进入执行循环
 11. 每个关键点调用 ExitController
 12. 必要时审计、反思、纠偏
 13. 进入总结阶段
 14. 构造 TaskResult
 15. 触发 TaskFinished Hook
 16. 返回 TaskResult
```

### 11.2 执行循环伪代码

```go
for {
    decision := exitController.Check(taskCtx)
    if decision.ShouldExit {
        break
    }

    step := planManager.NextExecutableStep(taskCtx.CurrentPlan)
    if step == nil {
        taskCtx.ExitReason = exit.NormalCompleted
        break
    }

    stepResult := stepRunner.Run(ctx, taskCtx.Snapshot(), step)
    planManager.ApplyStepResult(taskCtx, stepResult)

    if reflectionPolicy.ShouldReflect(taskCtx, stepResult) {
        reflection := reflector.Reflect(ctx, taskCtx.Snapshot(), stepResult)
        taskCtx.Reflections = append(taskCtx.Reflections, reflection)
        applyReflectionDecision(taskCtx, reflection)
    }

    if auditPolicy.ShouldAudit(taskCtx) {
        auditResult := auditor.Audit(ctx, taskCtx.Snapshot())
        taskCtx.Audits = append(taskCtx.Audits, auditResult)
        applyAuditDecision(taskCtx, auditResult)
    }

    if correctionPolicy.ShouldCorrect(taskCtx) {
        correction := corrector.Correct(ctx, taskCtx.Snapshot())
        planManager.ApplyCorrection(taskCtx, correction)
    }
}
```

### 11.3 关键检查点

以下位置必须调用 `ExitController.Check`：

- Planner 调用前和调用后。
- 每次模型调用前。
- 每次工具调用前。
- 每个 ReAct turn 结束后。
- 每个 Step 结束后。
- 审计、反思、纠偏前后。
- 最终总结前。

## 12. Planner 设计

### 12.1 Planner 输入

Planner 输入包括：

- 用户原始任务。
- 用户上下文。
- 工具描述快照。
- 初始历史经验摘要。
- 运行限制，例如最大步骤数、禁用工具、高风险工具策略。
- 可选业务元数据。

### 12.2 Planner 输出 Schema

模型应输出结构化 JSON：

```json
{
  "goal": "string",
  "assumptions": ["string"],
  "steps": [
    {
      "title": "string",
      "objective": "string",
      "expected_output": "string",
      "suggested_tools": ["string"],
      "dependencies": ["step_ref"],
      "risk_level": "read_only|low|high|dangerous"
    }
  ]
}
```

解析规则：

- 不接受空计划。
- 步骤数量不能超过 `MaxPlanSteps`。
- `suggested_tools` 必须存在于 ToolRegistry。
- `risk_level` 不能高于步骤中建议工具的最高风险等级。
- `dependencies` 必须能解析为已有步骤。
- `objective` 必须可判断完成，不能只有“继续分析”这类模糊描述。

### 12.3 计划校验失败处理

处理顺序：

1. 记录 `plan_validation_error`。
2. 若未超过模型解析或计划生成重试上限，请求模型按错误信息重新生成。
3. 多次失败后设置 `ExitReason=plan_validation_failed`。
4. 进入总结阶段，返回失败结果。

### 12.4 初始计划示例

用户任务：“检查当前主机安全风险”。

```json
{
  "goal": "识别当前主机的主要安全风险并给出整改建议",
  "assumptions": ["调用方提供的工具只能读取主机信息，不直接修改系统"],
  "steps": [
    {
      "title": "收集主机基础信息",
      "objective": "获取操作系统、内核、用户、进程和基础配置摘要",
      "expected_output": "主机基础信息摘要",
      "suggested_tools": ["host_info"],
      "dependencies": [],
      "risk_level": "read_only"
    },
    {
      "title": "检查暴露服务",
      "objective": "识别监听端口和对应进程",
      "expected_output": "开放端口和服务列表",
      "suggested_tools": ["port_scan"],
      "dependencies": ["收集主机基础信息"],
      "risk_level": "read_only"
    }
  ]
}
```

## 13. ReAct Executor 设计

### 13.1 单步骤执行流程

```text
StepRunner.Run(step)
  -> 标记 step running
  -> 初始化 StepExecution
  -> for turn in MaxStepReactTurns:
       检查步骤级退出条件
       构造 ReAct Prompt
       调用 LLMClient
       解析 ReactAction
       如果 action=tool_call:
           校验工具存在、参数、风险和次数限制
           调用 ToolGateway
           写入 Observation
           继续下一轮
       如果 action=step_result:
           校验结果是否满足 ExpectedOutput
           标记 completed
           返回
       如果 action=request_experience:
           调用 ExperienceProvider 或记录不可用
           继续下一轮
       如果 action=fail_step:
           标记 failed
           返回
       如果解析失败:
           记录 parse_error
           判断是否重试
  -> 超过轮数后标记 failed 或 no_progress
```

### 13.2 ReAct 输出 Schema

```json
{
  "action": "tool_call|step_result|request_experience|need_user_input|fail_step",
  "summary": "string",
  "tool_call": {
    "tool_name": "string",
    "reason": "string",
    "args": {}
  },
  "step_result": {
    "result": "string",
    "evidence": ["string"],
    "confidence": "low|medium|high"
  },
  "experience_request": {
    "query": "string",
    "reason": "string"
  },
  "failure": {
    "reason": "string",
    "recoverable": true
  }
}
```

约束：

- `action=tool_call` 时必须提供 `tool_name`、`reason`、`args`。
- `action=step_result` 时必须提供 `result`，并引用已有工具结果或上下文依据。
- `summary` 只能是面向记录的简短说明，不要求保存模型隐藏推理。
- 模型不能声称工具已执行，除非当前上下文已有对应 `Observation`。

### 13.3 无进展判断

以下情况视为可能无进展：

- 连续调用同一工具且参数相同，结果无新增信息。
- 连续输出同类解析错误。
- 多轮只重复已有结论，没有新增 Observation 或 StepResult。
- 反复请求不存在的工具。
- 反复请求历史经验但没有改变后续动作。

无进展达到阈值后：

- 当前步骤进入失败或触发反思。
- 若反思认为可恢复，可以纠偏后继续。
- 若不可恢复，进入总结或失败退出。

## 14. Tool Registry 和 Tool Policy

### 14.1 ToolDescriptor

```go
type ToolDescriptor struct {
    Name              string
    Description       string
    ArgsSchema        map[string]any
    ResultSchema      map[string]any
    RiskLevel         RiskLevel
    AutoCallable      bool
    RequiresApproval  bool
    DefaultTimeout    time.Duration
    Idempotent        bool
    TypicalFailures   []string
    Tags              []string
}
```

### 14.2 RiskLevel

| 等级 | 说明 | 默认自动调用 |
| --- | --- | --- |
| `read_only` | 只读查询，不改变外部状态。 | 允许 |
| `low` | 低风险操作，影响范围有限。 | 允许 |
| `high` | 可能改变系统状态、产生费用或影响服务。 | 默认不允许 |
| `dangerous` | 删除、阻断、提权、持久化修改等危险操作。 | 禁止，除非显式开启 |

### 14.3 ToolPolicy

```go
type ToolPolicy interface {
    Evaluate(ctx context.Context, req ToolPolicyRequest) (ToolPolicyDecision, error)
}
```

决策结果：

- `allow`：允许调用。
- `deny`：拒绝调用，步骤失败或请求纠偏。
- `require_approval`：需要调用方审批。MVP 可返回不支持审批并进入总结。
- `replace_tool`：建议替代工具。

MVP 可提供默认策略：

- 允许 `read_only` 和 `low`。
- 禁止 `high` 和 `dangerous`，除非配置显式允许。
- 禁止 `DisabledTools` 中的工具。

## 15. 审计设计

### 15.1 审计触发条件

支持以下触发：

- 每完成 `AuditEveryNSteps` 个步骤。
- 总 ReAct 轮数达到 `AuditEveryNTurns`。
- 连续工具失败。
- 连续无进展。
- 计划纠偏后需要复核。
- 最终总结前。
- 调用方通过 Hook 或外部控制主动请求审计。

### 15.2 AuditContext

审计输入使用快照：

- 用户原始目标。
- 初始计划和当前计划。
- 已完成、失败、跳过步骤。
- 最近 N 次工具调用摘要。
- 最近 N 次错误摘要。
- 反思和纠偏记录。
- 历史经验使用情况。
- 当前限制计数器。

### 15.3 AuditResult

```go
type AuditResult struct {
    AuditID            string
    Trigger            string
    Drifted            bool
    RiskLevel          RiskLevel
    Findings           []string
    Decision           AuditDecision
    CorrectionHint     string
    NeedExperience     bool
    NeedUserInput      bool
    ShouldExit         bool
    ExitReason         ExitReason
    CreatedAt          time.Time
}
```

`AuditDecision`：

- `continue`：继续执行。
- `minor_adjustment`：轻微调整，不重写计划。
- `correct_plan`：进入计划纠偏。
- `request_experience`：请求历史经验。
- `summarize_now`：提前总结。
- `fail`：失败退出。

### 15.4 审计保护

- 审计次数不能超过 `MaxAudits`。
- 审计失败不应默认导致任务失败，除非配置要求。
- 审计不能直接修改计划，只能给出决策和建议。
- 审计结果必须进入最终结果。

## 16. 反思设计

### 16.1 反思触发条件

触发条件：

- 工具连续失败。
- 工具参数连续非法。
- 模型输出连续不可解析。
- 步骤达到最大 ReAct 轮数。
- 步骤连续无进展。
- 审计发现偏离目标。
- 纠偏失败。
- 即将因超限退出但允许尽力恢复。

### 16.2 ReflectionResult

```go
type ReflectionResult struct {
    ReflectionID      string
    Trigger           string
    RootCause         string
    Impact            string
    Recoverable       bool
    Recommendation    ReflectionDecision
    DisableTools      []string
    CorrectionHint    string
    ExperienceQuery   string
    ReusableLesson    string
    CreatedAt         time.Time
}
```

`ReflectionDecision`：

- `retry_step`：重试当前步骤。
- `skip_step`：跳过当前步骤。
- `correct_plan`：进入计划纠偏。
- `request_experience`：请求历史经验。
- `summarize_now`：基于已有内容总结。
- `fail`：失败退出。

### 16.3 反思保护

- 反思次数不能超过 `MaxReflections`。
- 同一错误类型连续反思不得无限重试。
- 反思不能覆盖原始错误，只能追加分析和建议。
- 反思建议禁用工具时，必须记录原因和影响范围。

## 17. 计划纠偏设计

### 17.1 纠偏原则

- 只修改未执行步骤。
- 不删除已完成、失败、跳过、被替换步骤。
- 不改变用户原始目标。
- 不伪造已完成结果。
- 新增步骤必须说明原因。
- 跳过步骤必须说明原因。
- 替换工具路径必须说明新工具为何更合适。
- 每次纠偏生成新的 `Plan.Version`。

### 17.2 CorrectionResult

```go
type CorrectionResult struct {
    CorrectionID     string
    Trigger          string
    FromPlanVersion  int
    ToPlanVersion    int
    Actions          []CorrectionAction
    Reason           string
    Valid            bool
    ValidationErrors []string
    CreatedAt        time.Time
}
```

纠偏动作：

- `add_step`
- `skip_step`
- `replace_step`
- `split_step`
- `merge_steps`
- `reorder_steps`
- `replace_tool`
- `reduce_scope`
- `summarize_now`

### 17.3 纠偏校验

纠偏结果应用前必须校验：

- 未修改历史步骤状态和结果。
- 新步骤数量没有超过计划上限，除非配置允许动态新增。
- 新步骤引用的工具均存在且未禁用。
- 依赖关系无环。
- 纠偏不会导致所有剩余步骤都不可执行，除非决策为提前总结。

## 18. 错误处理设计

### 18.1 RuntimeError

```go
type RuntimeError struct {
    ErrorID        string
    Kind           ErrorKind
    Stage          string
    TaskID         string
    StepID         string
    ToolCallID     string
    ModelCallID    string
    Message        string
    Recoverable    bool
    Cause          string
    ActionTaken    string
    OccurredAt     time.Time
}
```

### 18.2 ErrorKind

| 类型 | 说明 | 默认策略 |
| --- | --- | --- |
| `config_error` | 配置非法。 | 启动前失败。 |
| `plan_generation_error` | 计划生成失败。 | 重试后失败。 |
| `plan_validation_error` | 计划校验失败。 | 要求重生成。 |
| `model_call_error` | 模型调用失败。 | 重试、反思或退出。 |
| `model_parse_error` | 模型输出不可解析。 | 要求重输，超限退出。 |
| `tool_not_found` | 工具不存在。 | 步骤失败、反思或纠偏。 |
| `tool_policy_denied` | 工具策略拒绝。 | 纠偏或总结。 |
| `tool_call_error` | 工具调用失败。 | 记录、反思、重试或退出。 |
| `tool_timeout` | 工具超时。 | 取消、反思或退出。 |
| `experience_error` | 历史经验获取失败。 | 记录后继续，除非强依赖。 |
| `audit_error` | 审计失败。 | 记录后继续或退出。 |
| `correction_error` | 纠偏失败。 | 记录后继续、总结或退出。 |
| `interrupt` | 调用方中断。 | 总结后返回。 |
| `system_error` | SDK 内部异常。 | 失败退出。 |

### 18.3 错误返回原则

- 不只返回 `error`，任务进入执行后必须尽量返回 `TaskResult`。
- 错误需要包含阶段、类型、影响范围和处理动作。
- 可恢复错误和不可恢复错误要明确区分。
- 触发退出的错误必须对应 `ExitReason`。

## 19. Hook 事件设计

### 19.1 HookEvent

```go
type HookEvent struct {
    EventID     string
    TaskID      string
    StepID      string
    Type        HookEventType
    Payload     any
    Snapshot    TaskSnapshot
    CreatedAt   time.Time
}
```

### 19.2 事件类型

| 事件 | 触发时机 |
| --- | --- |
| `task_started` | TaskContext 创建完成。 |
| `experience_loaded` | 初始历史经验加载完成。 |
| `plan_created` | 初始计划生成并校验完成。 |
| `step_started` | 步骤开始执行。 |
| `model_call_started` | 模型调用前。 |
| `model_call_finished` | 模型调用后。 |
| `tool_call_started` | 工具调用前。 |
| `tool_call_finished` | 工具调用后。 |
| `step_completed` | 步骤完成。 |
| `step_failed` | 步骤失败。 |
| `audit_started` | 审计开始。 |
| `audit_finished` | 审计完成。 |
| `reflection_started` | 反思开始。 |
| `reflection_finished` | 反思完成。 |
| `correction_applied` | 纠偏应用完成。 |
| `config_changed` | 动态配置变更。 |
| `task_interrupted` | 任务收到中断。 |
| `task_finished` | 任务结果构造完成。 |

### 19.3 Hook 执行模型

- Hook Manager 内部使用有界队列。
- 异步 Hook 队列满时，默认丢弃低优先级事件并记录错误。
- 同步 Hook 使用 `HookTimeout`。
- `task_finished` 事件建议同步触发，便于调用方持久化最终结果。

## 20. 中断、取消和超时

### 20.1 中断来源

- 调用方传入的 `context.Context` 被取消。
- 调用方调用 `Runtime.Interrupt`。
- 任务级 `TaskTimeout` 到期。
- 动态配置将 `AllowContinue` 设置为 false。
- 系统关闭时调用方取消父 context。

### 20.2 中断处理流程

```text
收到中断
  -> 设置 TaskContext.Interrupted
  -> 停止新的模型调用
  -> 停止新的工具调用
  -> 如果当前等待工具，调用 ToolGateway.Cancel
  -> 等待当前操作返回或超时
  -> 设置 ExitReason=user_interrupted 或 task_timeout
  -> 进入总结阶段
  -> 返回 TaskResult
```

### 20.3 工具取消语义

- `Cancel` 是取消意图，不保证外部工具真实停止。
- 如果工具不支持取消，SDK 等待工具超时或返回。
- 中断后返回的工具结果可以记录，但不再驱动后续步骤。

## 21. 并发设计

### 21.1 MVP 并发策略

- 单个任务内部计划步骤默认串行。
- 单个步骤内部工具调用默认串行。
- 多个任务可以并发调用同一个 Runtime。
- Adapter 是否线程安全由调用方保证；SDK 可以在文档中声明要求。

### 21.2 TaskContext 写入规则

- 主执行循环是唯一写入者。
- 中断和配置更新通过受控方法写入。
- Hook 和 Adapter 不允许持有可变引用。
- 如需跨 goroutine 写入，必须经 Runtime 内部事件队列合并。

### 21.3 后续并发扩展

后续支持并行步骤或并行工具前，需要补充：

- 步骤依赖 DAG。
- 工具结果合并策略。
- 并发失败传播策略。
- 审计快照一致性。
- 计划纠偏与并发执行的冲突处理。

## 22. Prompt 和结构化输出设计

### 22.1 Prompt 类型

SDK 默认提供以下 Prompt：

- `plan_generation`
- `react_step`
- `experience_request`
- `reflection`
- `audit`
- `plan_correction`
- `final_summary`

### 22.2 统一约束

所有 Prompt 都必须强调：

- 只能基于给定上下文和工具结果回答。
- 不能伪造工具结果。
- 不能把未执行步骤描述为已完成。
- 不能删除或改写历史错误。
- 不允许调用未注册工具。
- 不允许绕过工具风险策略。
- 输出必须符合指定 JSON schema。
- 不确定时必须标注不确定性。

### 22.3 Parser 设计

每类模型输出应有独立 parser：

- `planner.Parser`
- `executor.ActionParser`
- `audit.ResultParser`
- `reflection.ResultParser`
- `correction.ResultParser`
- `result.SummaryParser`

Parser 职责：

- JSON 解析。
- 必填字段校验。
- 枚举值校验。
- 工具名和步骤 ID 引用校验。
- 返回结构化错误，便于模型重试或退出。

## 23. 最终总结设计

### 23.1 FinalAnswer 要求

最终回答必须包含：

- 任务是否完成。
- 已完成的关键步骤。
- 关键发现和依据。
- 未完成内容和原因。
- 失败、超时、超限或中断信息。
- 风险和不确定性。
- 后续建议。

### 23.2 CompletionSummary

```go
type CompletionSummary struct {
    CompletedSteps      int
    FailedSteps         int
    SkippedSteps        int
    ToolCalls           int
    ModelCalls          int
    KeyFindings         []string
    UnfinishedItems     []string
    Risks               []string
    Recommendations     []string
}
```

### 23.3 可持久化要求

`TaskResult` 必须可 JSON 序列化：

- 所有时间使用 RFC3339 或 Unix 毫秒。
- `map[string]any` 中不保存不可序列化对象。
- 大字段应保存摘要，原始内容由调用方决定是否另存。
- 敏感内容是否脱敏由调用方 Adapter 或 Hook 负责，SDK 可提供可选脱敏 Hook。

## 24. 安全设计

### 24.1 工具安全

必须支持：

- 工具注册白名单。
- 参数 schema 校验。
- 风险等级。
- 自动调用开关。
- 高风险和危险工具显式配置。
- 禁用工具列表。
- 工具调用次数限制。
- 工具超时。

### 24.2 模型安全

需要防止：

- 调用不存在工具。
- 伪造 Observation。
- 忽略工具错误。
- 绕过计划。
- 绕过审计和退出限制。
- 无限请求经验。
- 输出不可解析内容。

### 24.3 执行安全

需要防止：

- 无限循环。
- 无限审计。
- 无限反思。
- 无限纠偏。
- 计划无限增长。
- 步骤重复执行。
- 工具重复调用无进展。

## 25. 测试策略

### 25.1 单元测试

必须覆盖：

- Config 默认值和校验。
- PlanValidator。
- ToolRegistry 和 ToolPolicy。
- ReAct ActionParser。
- ExitController。
- AuditPolicy。
- CorrectionValidator。
- ErrorManager。
- ResultBuilder。

### 25.2 使用 Fake Adapter 的集成测试

构造以下 Fake：

- `FakeLLMClient`：按请求目的返回预设 JSON。
- `FakeToolGateway`：按工具名和参数返回成功、失败、超时。
- `FakeExperienceProvider`：返回固定经验摘要。
- `RecordingHookSink`：记录事件顺序。

集成测试场景：

- 正常完成任务。
- 工具失败后反思并纠偏。
- 模型输出解析失败后重试。
- 审计发现偏离后纠偏。
- 用户中断。
- 工具超时。
- 总轮数超限后尽力总结。
- Hook 失败不影响主流程。

### 25.3 并发和竞态测试

- 多个任务并发 Run。
- Run 过程中调用 Interrupt。
- Run 过程中调用 UpdateConfig。
- 异步 Hook 队列满。
- 使用 `go test -race` 检查竞态。

### 25.4 回归测试样例

建议建立 `testdata/scenarios/`：

```text
testdata/scenarios/
├── normal_host_audit.json
├── tool_timeout_reflect.json
├── parse_error_retry.json
├── audit_correction.json
├── interrupt_during_tool.json
└── no_progress_exit.json
```

每个场景定义：

- 输入任务。
- 可用工具。
- Fake LLM 响应序列。
- Fake Tool 响应序列。
- 期望最终状态、退出原因、步骤状态和 Hook 顺序。

## 26. MVP 开发范围

### 26.1 MVP 必须实现

- `Runtime.New` 和 `Runtime.Run`。
- `RuntimeConfig`、默认值、校验。
- `TaskContext` 和只读快照。
- `LLMClient` 接口。
- `ToolGateway` 接口。
- `ToolRegistry` 和 `ToolDescriptor`。
- Planner 结构化计划生成和校验。
- PlanManager。
- ReAct 单步骤串行执行。
- 工具调用和 Observation 记录。
- ExitController。
- 基础错误模型。
- 基础 Reflector。
- 按步骤数量触发的 Auditor。
- 只修改未执行步骤的 PlanCorrector。
- Hook Manager。
- ResultBuilder。
- Fake Adapter 测试套件。

### 26.2 MVP 可以简化

- 经验只支持任务开始前传入，执行中经验请求先记录为未支持。
- 审计只支持每 N 个步骤和最终总结前触发。
- 反思只处理工具失败、解析失败、无进展。
- 工具调用不并发。
- 步骤不并发。
- 不实现人工审批，遇到需要审批的工具按策略拒绝或提前总结。
- 不实现任务暂停和恢复。

### 26.3 MVP 不包含

- HTTP、gRPC、CLI、Web UI。
- 数据库持久化。
- 分布式调度。
- 多 Agent 协作。
- 并行工具调用。
- 向量记忆。
- 复杂权限系统。
- 沙箱执行器。

## 27. 推荐实现顺序

### 阶段 1：基础骨架

目标：能创建 Runtime，校验配置，返回最小失败或空计划结果。

任务：

- 建立包结构。
- 实现 `RuntimeConfig`。
- 实现 `TaskInput`、`TaskContext`、状态枚举。
- 实现 `TaskResult` 基础结构。
- 实现 `LLMClient`、`ToolGateway` 接口。
- 实现 `HookSink` 基础事件。

验收：

- 配置校验测试通过。
- `Run` 在缺少依赖时返回明确错误。

### 阶段 2：计划生成

目标：能调用 Fake LLM 生成计划并校验。

任务：

- 实现 Planner。
- 实现计划 JSON parser。
- 实现 PlanValidator。
- 实现 PlanManager 初始保存。
- 增加 `plan_created` Hook。

验收：

- 合法计划通过。
- 空计划、未知工具、依赖错误、步骤超限都能失败并返回结构化错误。

### 阶段 3：ReAct 和工具调用

目标：能按计划执行步骤，调用 Fake Tool 并完成结果。

任务：

- 实现 ReActExecutor。
- 实现 ActionParser。
- 实现 ToolRegistry 参数校验。
- 实现 ToolGateway 调用包装。
- 实现 StepExecution 和 ToolCallRecord。

验收：

- 正常工具调用后步骤完成。
- 工具失败、超时、未知工具进入错误记录。
- 单步骤轮数和工具次数限制生效。

### 阶段 4：退出、错误、总结

目标：所有失败路径都能返回可持久化结果。

任务：

- 实现 ExitController。
- 实现 ErrorManager。
- 实现 ResultBuilder。
- 实现中断处理。
- 实现基础最终总结 Prompt。

验收：

- 用户中断、总轮数超限、工具失败超限、解析失败超限都有正确 `ExitReason`。
- `TaskResult` 可 JSON 序列化。

### 阶段 5：审计、反思、纠偏

目标：具备基本自恢复能力。

任务：

- 实现 Reflector。
- 实现 Auditor。
- 实现 PlanCorrector。
- 实现纠偏校验和 PlanDiff。
- 增加对应 Hook。

验收：

- 工具连续失败触发反思。
- 审计要求纠偏时只修改未执行步骤。
- 纠偏次数限制生效。

### 阶段 6：测试和文档收口

目标：形成可发布 MVP。

任务：

- 补齐 Fake Adapter。
- 补齐场景测试。
- 跑 `go test ./...` 和 `go test -race ./...`。
- 完善 README 使用示例。
- 固定公共 API。

验收：

- 核心路径测试通过。
- race 测试无数据竞争。
- 示例代码可运行。

## 28. 使用示例

```go
rt, err := agentruntime.New(
    agentruntime.WithLLMClient(myLLM),
    agentruntime.WithToolGateway(myTools),
    agentruntime.WithTools([]agentruntime.ToolDescriptor{
        {
            Name:         "host_info",
            Description:  "读取主机基础信息",
            RiskLevel:    agentruntime.RiskReadOnly,
            AutoCallable: true,
        },
        {
            Name:         "port_scan",
            Description:  "读取本机监听端口",
            RiskLevel:    agentruntime.RiskReadOnly,
            AutoCallable: true,
        },
    }),
)
if err != nil {
    return err
}

result, err := rt.Run(ctx, agentruntime.TaskInput{
    UserInput: "检查当前主机安全风险，并给出整改建议",
})
if err != nil {
    return err
}

saveResult(result)
```

## 29. 示例运行流程

以“检查当前主机安全风险”为例：

1. 调用方创建 Runtime，注入 LLM、ToolGateway 和工具描述。
2. 调用方提交 `TaskInput`。
3. Runtime 创建 `TaskContext`。
4. Runtime 校验配置并加载工具快照。
5. Runtime 触发 `task_started` Hook。
6. Planner 生成初始计划。
7. PlanValidator 校验步骤、工具和依赖。
8. Runtime 触发 `plan_created` Hook。
9. StepRunner 执行第一个步骤“收集主机基础信息”。
10. ReAct 输出 `tool_call=host_info`。
11. ToolGateway 调用外部工具进程。
12. 工具返回主机信息 Observation。
13. ReAct 输出步骤结果。
14. PlanManager 标记步骤完成。
15. StepRunner 执行第二个步骤“检查暴露服务”。
16. ReAct 调用 `port_scan`。
17. 工具返回开放端口列表。
18. 每完成 3 个步骤后触发 Auditor。
19. Auditor 判断没有偏离目标，继续执行。
20. 某工具连续失败时触发 Reflector。
21. Reflector 判断参数缺失或工具不可用。
22. PlanCorrector 调整未执行步骤或替换工具路径。
23. 所有可执行步骤完成。
24. Runtime 最终审计。
25. ResultBuilder 生成最终答案和结构化结果。
26. Runtime 触发 `task_finished` Hook。
27. SDK 返回 `TaskResult`。

## 30. 开发验收标准

MVP 交付时应满足：

- 公共 API 清晰，调用方无需了解内部包即可运行任务。
- 任务成功、失败、中断、超限均能返回结构化 `TaskResult`。
- 所有工具调用都有请求、响应、耗时、状态和错误记录。
- 模型输出均经过 parser 和 validator。
- 计划纠偏不会篡改已执行历史。
- Hook 事件顺序可预测。
- 核心限制器能防止无限循环。
- Fake Adapter 场景测试覆盖主要路径。
- 文档中的使用示例与实际 API 保持一致。

## 31. 后续扩展方向

可在 MVP 稳定后扩展：

- 执行中历史经验请求。
- 人工审批和高风险工具审批。
- 任务暂停和恢复。
- 并行步骤执行。
- 并行工具调用。
- 多 Agent 协作。
- 多模型路由。
- 外部工作流系统集成。
- 任务事件流适配器。
- 向量记忆适配器。
- 安全场景专用工具协议。
- 结果报告模板。

## 32. 最终结论

`AgentRuntime SDK` 是一个 Go 实现的轻量级 Agent 运行时内核。它负责计划、ReAct 执行、工具调用编排、错误反思、阶段性审计、计划纠偏、退出控制和结果生成。

它不绑定具体模型、工具、数据库、服务端或前端。调用方通过 Adapter 和 Hook 提供外部能力，并自行决定如何持久化、展示和审批。

后续开发应优先保证三件事：

- 执行可控：所有循环、工具调用、纠偏和审计都有上限。
- 状态可信：历史不可篡改，最终结果可追溯。
- 接口稳定：调用方只依赖清晰的 Runtime、Adapter、Hook 和 Result 契约。
