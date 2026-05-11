# AgentRuntime SDK 开发提示词

这份提示词用于后续让 Codex、Claude、Cursor、ChatGPT 或其他 AI 编程助手继续开发 `/code/agent-runtime` 项目。使用时可以整段复制，也可以按阶段复制对应章节。

## 1. 主提示词

```text
你是一个资深 Go SDK 工程师，现在要在 /code/agent-runtime 仓库中实现 AgentRuntime SDK。

请先阅读并遵守以下设计文档：

- /code/agent-runtime/agent-runtime.md
- /code/agent-runtime/agent-runtime.optimized.md

项目目标：

实现一个基于 Go 的 AgentRuntime SDK。它是依赖包，不是独立服务。SDK 负责 Agent 任务的计划、ReAct 执行、外部工具调用编排、错误反思、阶段性审计、计划纠偏、退出控制和最终结果生成。

边界要求：

- 不实现 HTTP Server。
- 不实现 Web UI。
- 不内置数据库。
- 不内置真实工具执行环境。
- 不绑定具体模型厂商。
- 不实现多租户、认证、分布式调度。
- 所有外部能力必须通过接口注入。

核心开发原则：

- 先实现稳定、可测试、可复用的 SDK 内核。
- 优先保证任务执行可控，所有循环、工具调用、审计、反思、纠偏都必须有上限。
- 计划纠偏只能修改未执行步骤，不能篡改已完成、失败、跳过或已记录的历史。
- 模型输出必须经过结构化 parser 和 validator，不能直接相信自然语言。
- 工具调用必须经过 ToolRegistry、参数校验、风险策略、次数限制和超时控制。
- 任务开始执行后，即使失败、中断或超限，也应尽量返回结构化 TaskResult。
- Hook 和 Adapter 不应直接持有可变 TaskContext，只能接收只读快照。
- 默认步骤串行执行，默认工具串行调用。

建议实现顺序：

1. 建立 Go module 和基础包结构。
2. 实现 RuntimeConfig、默认值、配置校验。
3. 实现 TaskInput、TaskContext、TaskSnapshot、TaskResult、状态枚举。
4. 定义 LLMClient、ToolGateway、ExperienceProvider、HookSink、PromptProvider、ToolPolicy 等外部接口。
5. 实现 ToolDescriptor、ToolRegistry、风险等级和默认 ToolPolicy。
6. 实现 Planner、Plan、PlanStep、PlanValidator、PlanManager。
7. 实现 ReActExecutor、ActionParser、StepRunner、Observation、StepExecution。
8. 实现 ToolGateway 调用包装、ToolCallRecord、工具失败和超时处理。
9. 实现 ExitController、RuntimeError、ErrorManager、ResultBuilder。
10. 实现 Interrupt 和 UpdateConfig。
11. 实现基础 Reflector、Auditor、PlanCorrector。
12. 实现 HookManager。
13. 编写 FakeLLMClient、FakeToolGateway、RecordingHookSink 和场景测试。
14. 最后补 README 示例和公共 API 使用说明。

每次开发前请先：

- 查看 git 状态，避免覆盖已有用户修改。
- 阅读当前文件结构和最近变更。
- 对照设计文档确认本次改动属于哪个阶段。
- 选择最小可验证的开发范围。

代码风格要求：

- 使用 Go 标准库优先。
- 包名简短清晰，不使用 context、errors 等容易与标准库混淆的包名。
- 公共类型和公共函数必须有清晰注释。
- 不要过早引入复杂抽象。
- 不要引入网络依赖或重量级第三方库，除非明确必要。
- 对外 API 保持稳定，内部实现可以逐步演进。
- 错误需要包含阶段、类型、任务 ID、步骤 ID、处理动作等上下文。
- 测试中使用 fake adapter，不依赖真实模型和真实工具。

测试要求：

- 每完成一个阶段，运行最窄范围的 go test。
- 核心包需要单元测试。
- Runtime 主路径需要 fake adapter 集成测试。
- 中断、超时、工具失败、模型解析失败、审计纠偏都需要测试。
- 涉及并发、Hook、Interrupt、UpdateConfig 时，应运行 go test -race。

完成一次开发任务后，请输出：

- 修改了哪些文件。
- 实现了设计文档中的哪些能力。
- 运行了哪些测试。
- 还有哪些能力未完成。
- 如有未运行测试，说明原因。

不要做的事情：

- 不要把 SDK 改成服务端项目。
- 不要加入数据库、HTTP 路由、前端页面。
- 不要真实执行 shell 命令作为工具实现。
- 不要在测试里调用真实模型。
- 不要绕过 parser 和 validator。
- 不要让纠偏覆盖历史执行记录。
- 不要为追求完整性一次性写过大的代码改动。
```

## 2. 阶段 1 提示词：基础骨架

```text
请在 /code/agent-runtime 中实现 AgentRuntime SDK 的阶段 1：基础骨架。

请先阅读：

- agent-runtime.md
- agent-runtime.optimized.md

本阶段只实现 SDK 骨架，不实现真实执行逻辑。

需要完成：

- 初始化 Go module。
- 建立推荐包结构。
- 实现 RuntimeConfig、默认值和 Validate。
- 实现 TaskInput、TaskContext、TaskSnapshot、Counters。
- 实现 TaskStatus、StepStatus、ExitReason、RiskLevel 等枚举。
- 实现 TaskResult 的基础结构。
- 定义 LLMClient、ToolGateway、ExperienceProvider、HookSink、PromptProvider、ToolPolicy 接口。
- 实现 Runtime.New 和基础 Option。
- 实现 Runtime.Run 的启动级校验：缺少必需依赖时返回明确错误。

不需要完成：

- Planner 调用。
- ReAct 执行。
- 工具调用。
- 审计、反思、纠偏。
- Hook 异步队列。

验收标准：

- go test ./... 通过。
- 配置默认值和配置校验有单元测试。
- Runtime.New 对缺少 LLMClient、ToolGateway 等情况有测试。
- 当前代码可以作为后续阶段继续开发的稳定基础。
```

## 3. 阶段 2 提示词：计划生成

```text
请实现 AgentRuntime SDK 的阶段 2：计划生成。

开发依据：

- /code/agent-runtime/agent-runtime.md 中 Planner、Plan Manager、PlanValidator 相关章节。

需要完成：

- 实现 Plan、PlanStep、PlanDependency、PlanVersion。
- 实现 Planner，调用 LLMClient 生成结构化计划。
- 实现 Planner 输出 JSON parser。
- 实现 PlanValidator。
- 实现 PlanManager 初始计划保存、状态查询和 NextExecutableStep 的基础能力。
- Runtime.Run 能完成：初始化 -> 计划生成 -> 计划校验 -> 返回包含计划的 TaskResult。
- 增加 plan_created Hook 事件，可以先同步触发或用简化 HookManager。

计划校验必须覆盖：

- 空计划。
- 步骤数量超限。
- 步骤 ID 或标题重复。
- 引用不存在的工具。
- 依赖不存在。
- 依赖成环。
- 步骤目标为空或过于模糊。

测试要求：

- 使用 FakeLLMClient 返回合法计划。
- 使用 FakeLLMClient 返回非法计划。
- 验证 TaskResult 中包含 InitialPlan 和 FinalPlan。
- 验证非法计划返回 plan_validation_failed 或结构化错误。
```

## 4. 阶段 3 提示词：ReAct 和工具调用

```text
请实现 AgentRuntime SDK 的阶段 3：ReAct 执行和工具调用。

开发依据：

- agent-runtime.md 中 ReAct Executor、Tool Gateway、Tool Registry、Tool Policy 相关章节。

需要完成：

- 实现 ReActExecutor 和 StepRunner。
- 实现 ReactAction、ReactTurn、Observation、StepExecution。
- 实现 ActionParser，解析模型输出的结构化 JSON。
- 实现 ToolDescriptor、ToolRegistry、ToolRequest、ToolResponse、ToolCallRecord。
- 实现默认 ToolPolicy。
- Runtime.Run 能按计划串行执行步骤。
- 工具调用前校验工具存在、参数结构、风险等级、禁用工具和次数限制。
- 工具结果写入 Observation，并进入下一轮 ReAct。
- 步骤完成时更新 PlanStep 状态和 StepExecution。

限制要求：

- 单步骤最大 ReAct 轮数必须生效。
- 单步骤最大工具调用次数必须生效。
- 单任务最大工具调用次数必须生效。
- 模型解析失败次数必须生效。
- 工具失败和超时必须进入 RuntimeError。

测试要求：

- FakeLLMClient 先返回 tool_call，再返回 step_result。
- FakeToolGateway 返回成功结果。
- 测试未知工具、工具失败、工具超时、参数非法。
- 测试步骤完成后 PlanManager 能选择下一个步骤。
```

## 5. 阶段 4 提示词：退出、错误和结果

```text
请实现 AgentRuntime SDK 的阶段 4：退出控制、错误管理和最终结果。

开发依据：

- agent-runtime.md 中 Exit Controller、错误处理、最终结果、中断与取消相关章节。

需要完成：

- 实现 ExitController。
- 实现 RuntimeError、ErrorKind、ErrorManager。
- 实现 ResultBuilder。
- 实现 Runtime.Interrupt。
- 实现 Runtime.UpdateConfig 的安全子集。
- 实现任务超时、总轮数超限、工具调用超限、模型解析失败超限、无进展退出。
- 最终 TaskResult 必须包含状态、退出原因、最终答案、计划、步骤记录、工具调用记录、模型调用摘要、错误记录和指标。

中断要求：

- context 取消时停止新的模型调用和工具调用。
- Interrupt 设置任务中断标记。
- 当前工具调用如果存在，调用 ToolGateway.Cancel。
- 中断后进入总结阶段，返回 interrupted 状态。

测试要求：

- 用户中断。
- context 超时。
- 工具调用次数超限。
- 模型解析失败超限。
- ResultBuilder 产物可 JSON 序列化。
```

## 6. 阶段 5 提示词：审计、反思和纠偏

```text
请实现 AgentRuntime SDK 的阶段 5：审计、反思和计划纠偏。

开发依据：

- agent-runtime.md 中 Auditor、Reflector、Plan Corrector、审计策略、纠偏策略相关章节。

需要完成：

- 实现基础 Reflector。
- 实现基础 Auditor。
- 实现 PlanCorrector。
- 实现 CorrectionValidator 和 PlanDiff。
- 实现审计触发策略：每 N 个步骤、最终总结前、连续失败。
- 实现反思触发策略：工具连续失败、模型解析失败、无进展、步骤失败。
- 纠偏只能修改未执行步骤。
- 每次纠偏生成新的 Plan.Version。
- 审计、反思、纠偏结果进入 TaskResult。

保护要求：

- MaxAudits 生效。
- MaxReflections 生效。
- MaxCorrections 生效。
- 审计失败不能默认导致任务失败，除非配置要求。
- 纠偏失败不能覆盖原始计划和历史执行记录。

测试要求：

- 工具连续失败触发反思。
- 审计判断偏离目标后触发纠偏。
- 纠偏新增步骤、跳过步骤、替换步骤都有测试。
- 尝试修改已完成步骤时，CorrectionValidator 必须拒绝。
```

## 7. 阶段 6 提示词：测试、文档和发布前收口

```text
请对 /code/agent-runtime 做 MVP 收口。

需要完成：

- 补齐 FakeLLMClient、FakeToolGateway、FakeExperienceProvider、RecordingHookSink。
- 在 testdata/scenarios 下增加场景测试数据。
- 覆盖正常完成、工具失败、工具超时、解析失败、审计纠偏、中断、无进展、超限。
- 补充 README 使用示例。
- 检查公共 API 命名是否稳定。
- 检查 TaskResult 是否可 JSON 序列化。
- 检查所有 public 类型注释。
- 运行 go test ./...。
- 涉及并发后运行 go test -race ./...。

输出最终报告：

- 已完成能力。
- 测试结果。
- MVP 尚未包含的能力。
- 后续建议。
```

## 8. 单次任务模板

```text
请在 /code/agent-runtime 中完成下面这个开发任务：

任务：
【在这里写具体任务】

约束：

- 先阅读 agent-runtime.md 和 agent-runtime.optimized.md 中相关章节。
- 不要修改与本任务无关的文件。
- 不要覆盖用户已有修改。
- 保持 SDK 边界，不加入服务端、数据库、前端或真实工具执行。
- 为新增行为补测试。
- 完成后运行最窄范围的 go test。

完成后请说明：

- 改了哪些文件。
- 实现了什么。
- 运行了什么测试。
- 有没有未完成或需要后续处理的事项。
```

## 9. Code Review 提示词

```text
请对 /code/agent-runtime 当前改动做代码审查。

审查重点：

- 是否符合 agent-runtime.md 的 SDK 边界。
- 是否引入了不该有的服务端、数据库、前端或真实工具执行逻辑。
- Runtime、TaskContext、Plan、ReAct、Tool、Exit、Hook 的职责是否清晰。
- 模型输出是否经过 parser 和 validator。
- 工具调用是否经过注册、参数校验、风险策略、次数限制和超时。
- 纠偏是否只修改未执行步骤。
- 失败、中断、超限是否仍能返回结构化 TaskResult。
- 是否存在数据竞争、状态篡改、无限循环风险。
- 测试是否覆盖主要成功和失败路径。

请按严重程度列出问题，给出文件和行号，并说明建议修改方式。
```

## 10. Bug 修复提示词

```text
请修复 /code/agent-runtime 中的以下问题：

问题描述：
【在这里写 bug 现象、报错或失败测试】

要求：

- 先定位根因，再做最小修改。
- 不要顺手重构无关代码。
- 不要改变公共 API，除非 bug 必须如此修复。
- 修复后补充或更新回归测试。
- 运行能覆盖该 bug 的最窄范围测试。

完成后请说明：

- 根因是什么。
- 如何修复。
- 新增或更新了哪些测试。
- 测试结果如何。
```

## 11. 验收清单

后续每次阶段性交付前，至少确认：

- [ ] `Runtime.New` 能完成依赖校验和配置初始化。
- [ ] `Runtime.Run` 在成功、失败、中断、超限时都有明确结果。
- [ ] `TaskResult` 可 JSON 序列化。
- [ ] `ToolGateway` 不真实执行工具，只负责接口抽象。
- [ ] `LLMClient` 不绑定具体模型厂商。
- [ ] Planner 输出经过 parser 和 validator。
- [ ] ReAct 动作经过 parser 和 validator。
- [ ] 工具调用经过 ToolRegistry 和 ToolPolicy。
- [ ] ExitController 在关键点生效。
- [ ] 审计、反思、纠偏都有最大次数限制。
- [ ] 纠偏不能修改已执行历史。
- [ ] Hook 不直接修改 TaskContext。
- [ ] Fake Adapter 测试覆盖核心路径。
- [ ] 没有引入服务端、数据库、前端、真实工具执行等非目标能力。
