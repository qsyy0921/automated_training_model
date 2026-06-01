# Sub-Agent Strategy SDD

版本：v0.1  
日期：2026-06-02

## 目标

本项目的核心是 Agent Runtime，不是单一页面工具。Sub-agent 的作用是把复杂任务拆成可审计、可限流、可替换的小执行单元，避免主 Agent loop 后期变成一个巨大分支树。

## 参考项目吸收点

- Claude Code 风格：CLI 是主入口，工具调用、权限判断、MCP 和会话恢复都在 runtime 前后形成明确边界。
- Hermes 风格：Python runtime 更适合承载 LLM loop、tool calling、skills、subagents 和模型生态；TypeScript/Go 客户端只做入口与展示。
- OpenClaw 风格：Gateway 把平台、客户端、channel、模型、工具和记忆连接起来；channel/provider/plugin 不能绕过 SDK/manifest/runtime 边界。

## 什么时候使用 sub-agent

当前判断顺序：

1. 先判断是不是确定性低风险命令。`/bot-ping`、`/bot-me`、`/bot-status`、`/bot-runs`、`/bot-run dry` 由 Go control plane 直接处理，不进入 sub-agent。
2. 再判断是否有附件。图片、截图、异常帧等视觉附件交给 `vision-agent`；zip、manifest、目录索引等非视觉数据交给 `data-intake-agent`。
3. 再判断是否是自由文本。自然语言请求先交给 `planner-agent` 做意图细化、tool-call plan 和 preflight。
4. 长流程任务只规划，不直接执行。训练、评估、部署、模型下载等有副作用任务必须经过 tool executor 的权限和审批边界。
5. skill 自进化默认关闭。只有成功 trace 可以进入 `skill-miner-agent` 生成草稿，草稿必须人工审批后才启用。

使用 sub-agent：

| 场景 | 原因 | 默认 sub-agent |
| --- | --- | --- |
| 自由文本需要规划工具链 | 需要 LLM 推理、意图细化、工具预检 | `planner-agent` |
| QQ/远程 channel 上传图片 | 需要视觉模型检查、隔离、风险判断 | `vision-agent` |
| QQ/远程 channel 上传 zip、manifest、文件 | 必须先 quarantine、scan、生成 Data Intake Plan | `data-intake-agent` |
| 训练、评估、部署计划跨多个任务 | 需要长流程、重试、artifact 记录 | `training-agent` / `release-agent` |
| 并行研究或大范围代码/数据检查 | 可独立收集证据，降低主会话上下文压力 | 专用 reviewer / analyst sub-agent |
| 总结可复用 skill | 必须从成功 trace 里提取草稿并等待人工批准 | `skill-miner-agent` |

不使用 sub-agent：

| 场景 | 原因 |
| --- | --- |
| `/bot-ping`、`/bot-me`、`/bot-status` | 低风险确定性命令，Go 控制面直接返回。 |
| `/bot-runs`、`/bot-run dry` | 已有明确 workflow API，不需要再让 LLM 决策。 |
| 单次只读 API 查询 | 直接由 CLI/Web 调 Gateway。 |
| 高风险写操作未获得审批 | 不能通过 sub-agent 绕过审批。 |
| 需要同一个可变状态的强一致事务 | 暂时由 Go 应用服务串行处理，避免并发写坏状态。 |

## 调度规则

```text
EntryPoint(CLI/Web/Desktop/QQ)
  -> Gateway
  -> Normalize Request
  -> Intent Router
  -> Delegation Decision
  -> Main Agent or Sub-Agent
  -> Tool/MCP/Workflow
  -> Audit + Result Egress
```

当前 Go 最小实现：

- `internal/app/agentruntime/intent.go`：规则意图识别。
- `internal/app/agentruntime/subagent.go`：sub-agent 决策。
- `internal/app/agentruntime/status.go`：runtime 状态、模型路由和入口状态。
- `/api/runtime/status`：暴露运行时契约。

后续 Python runtime 会接管 LLM-heavy planning，但 Go 仍保留入口、权限、审计和 workflow 控制面。

## 模型路由

| Route | 用途 | Provider / Model |
| --- | --- | --- |
| `text-planning` | 意图细化、计划、JSON workflow、策略判断 | Mimo `mimo-v2.5-pro` |
| `vision` | 图片、截图、异常帧、视觉数据理解 | Mimo `mimo-v2.5` |
| `image-generation` | 架构图、产品图、视觉资产生成 | ChatGPT 5.5 image reverse proxy via MCP |

所有 key 只允许在服务端环境变量或 secret store 中引用，不进入 Git、浏览器端或 channel 消息。

## 测试

| ID | 输入 | 预期 |
| --- | --- | --- |
| SA-001 | `/bot-status` | `use_sub_agent=false`，Go 控制面直接处理。 |
| SA-002 | QQ 文本 `帮我做训练 dry-run` | `planner-agent`。 |
| SA-003 | QQ 图片附件 | `vision-agent`，route=`vision`。 |
| SA-004 | QQ zip 附件 | `data-intake-agent`，route=`text-planning`。 |
| SA-005 | skill 自进化关闭 | 不生成启用态 skill，只允许草稿和审计。 |
