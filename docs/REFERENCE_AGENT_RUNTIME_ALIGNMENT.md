# 参考项目对齐：Agent Runtime

版本：v0.1
日期：2026-06-02
参考源码：

- `E:\agent\openclaw`
- `E:\agent\cc`
- `E:\agent\Hermes`

本文只提炼可落到本项目的架构原则，不照搬代码。

## 1. 三个项目各自值得借鉴的点

| 项目 | 可借鉴点 | 对本项目的落点 |
| --- | --- | --- |
| OpenClaw | Gateway 把多端、多 channel、模型、插件和记忆连接起来；channel/plugin 有独立 runtime surface。 | `Go Gateway + Channel Runtime Surface`，QQ/Telegram/飞书只能通过 Channel Adapter 进入。 |
| cc | CLI-first；QueryEngine/Agentic Loop/Tool/Permission/MCP 分层清晰；工具调用前做权限检查；上下文压缩是 runtime 能力。 | CLI 作为主入口；Agent Loop 必须有 permission preflight、context budget、tool result envelope。 |
| Hermes | Python 生态承载 LLM loop、tools、skills、MCP、memory、gateway platforms；gateway session 带来源上下文。 | Python Agent Runtime 负责 planner/tool-call/skills；Go 只保留控制面、审计和生命周期编排。 |

## 2. 本项目采用的长期结构

```text
Entry Points
  CLI / Web / Desktop / QQ(NapCat) / future Telegram / Feishu
    |
Go Gateway
  auth / channel policy / session key / audit / approval / workflow registry
    |
Session Runner
  restore session / build runtime context / budget / route to agent
    |
Python Agent Runtime
  intent refine / planner / skill resolver / tool-call plan / memory
    |
Tool Executor
  Go tools / Python workers / MCP servers / model providers / data workflows
    |
Reply Router
  channel-specific formatting / outbound delivery / trace
```

## 3. 必须保留的边界

### 3.1 Channel Adapter 不碰业务

QQ/NapCat adapter 只做：

- OneBot event -> `channel.InboundMessage`
- `channel.OutboundMessage` -> OneBot `send_msg`
- 账号、peer、sender、附件 metadata 归一化

它不能直接：

- 写 Data Lake
- 提交训练任务
- 调模型
- 创建 skill
- 操作部署

这些必须走 Agent Runtime、Tool Registry 和治理 preflight。

### 3.2 Go 不承载 LLM-heavy loop

Go 适合：

- Gateway / HTTP / WebSocket
- channel 连接生命周期
- session key 和审计
- tool/workflow registry
- approval 和 policy enforcement
- task 状态和队列

Python 适合：

- LLM planner
- VLM/多模态理解
- skills 检索和执行计划
- MCP client/tool schema 适配
- 数据、训练、评估 worker

### 3.3 Tool 是唯一执行出口

Agent 不直接调用业务 service。Agent 只能输出结构化 plan：

```json
{
  "intent": "submit_workflow_dry_run",
  "tool_calls": [
    {
      "tool_id": "workflow.submit_run",
      "params": {
        "workflow_id": "data-to-deployment-lifecycle",
        "dataset_id": "shanghaitech-original",
        "dry_run": true
      }
    }
  ]
}
```

Tool Executor 在执行前必须检查：

- input schema
- channel/source 权限
- approval gate
- data scope
- secret scope
- budget
- sandbox/runtime

## 4. 我们和三个项目的取舍

### 不做 OpenClaw 式大插件系统的第一版

先实现稳定内部 registry：

- tool registry
- skill registry
- channel registry
- provider registry
- MCP registry

等边界稳定后，再允许插件扩展。

### 不做 cc 式全 TypeScript runtime

本项目要处理训练、视觉、数据湖和模型生命周期，Python 生态收益更大。TypeScript 只负责 Web/桌面 UI 和轻量客户端，不放核心 agent loop。

### 不做 Hermes 式全 Python Gateway

Go 控制面更适合长期承载本项目的 API、任务状态、审计、权限和部署生命周期。Python 作为 worker/runtime，被 Go 调度。

## 5. 下一阶段代码拆分顺序

1. `[done]` `internal/app/agentruntime` 已增加 `SessionRunner`、`PlannerPort`、`ToolExecutorPort`，命令处理已经从 `service.go` 拆到 `planner.go` 与 `tools.go`。
2. `workers/python/agent_runtime` 增加 Mimo planner，普通文本输出结构化 intent/plan。
3. 新增 `internal/app/toolapp`，统一 tool schema、permission、approval 和 execution envelope。
4. 新增 `internal/app/sessionapp`，保存 channel session、runtime trace、context summary。
5. QQ inbound 从 HTTP debug handler 升级为长期运行的 OneBot WS reader，并接 outbound delivery。
6. 把 `workflow.submit_run` 从 runtime 命令变成 Tool Executor 调用，不再在 runtime service 里直接调 agent app。

## 6. 当前 MVP 和目标差距

| 能力 | 当前状态 | 目标 |
| --- | --- | --- |
| QQ/NapCat 连接 | Linux Docker NapCat 已验证在线 | 长驻 reader + outbound 自动回复 |
| Intent | 规则命令 + 附件优先 | 规则层 + Mimo planner JSON intent |
| Session | 只有 channel peer 信息 | 持久 session、context summary、runtime trace |
| Tool | 直接调用 Go app service | Tool registry + preflight + executor |
| Skill | draft-only | skill registry、审批、版本、回滚 |
| MCP | 文档和设计 | MCP registry、tool discovery、permission wrapper |
| Sub-agent | 决策契约 | Python runtime 可调度、限深、限并发、可追踪 |
