# Intent / Tool / Skill / MCP SDD

版本：v0.1  
日期：2026-06-02  
范围：设计 QQ、CLI、Web、桌面端进入 Agent Runtime 后，识别出的意图如何落到 Tool、Skill、MCP 和 workflow。

## 1. 参考结论

参考 OpenClaw、Hermes、Claude Code 三类设计后，采用下面的分工：

| 概念 | 职责 | 对应参考 |
| --- | --- | --- |
| Intent | 用户想做什么，只描述意图，不执行。 | OpenClaw channel routing、Claude Code QueryEngine 前置解析。 |
| Skill | 一段可复用流程知识，告诉 Agent 如何完成一类任务。 | OpenClaw skills、Hermes skills。 |
| Tool | 最小可执行能力，有 schema、权限、风险和审计。 | Claude Code tools、Hermes tools。 |
| MCP | 外部工具/服务的协议边界，用于接数据库、文件、浏览器、模型服务等。 | Hermes MCP、OpenClaw plugin tools。 |
| Workflow | 多 Agent / 多 tool 的长期任务编排。 | 当前 `data-to-deployment-lifecycle`。 |

结论：**Intent 不直接执行；Intent 先变 Plan；Plan 再选择 Skill；Skill 决定要调用哪些 Tool/MCP；Tool 调用前必须过 Governance。**

语言选择上，Intent/Planner/Skill Resolver 更适合 Python，因为后续要接 LLM/VLM、多模态数据和训练生态。Go 保留规则意图、权限、状态、审计和 tool preflight。

## 2. 执行链路

```text
InboundMessage
  -> Intent Router
  -> Structured Intent
  -> Planner
  -> Action Plan
  -> Skill Resolver
  -> Tool / MCP Resolver
  -> Governance Preflight
  -> Executor
  -> Audit / Reply
```

## 3. Intent

Intent 是最薄的一层，只回答“用户要干什么”：

```json
{
  "kind": "submit_dry_run",
  "raw_text": "/bot-run dry shanghaitech-original",
  "skill_id": "data-to-deployment-lifecycle",
  "tool_id": "workflow.submit_run",
  "dataset_id": "shanghaitech-original",
  "confidence": 1.0
}
```

当前代码落点：

```text
internal/app/agentruntime/intent.go
```

规则层识别：

- `/bot-ping` -> `health_check`
- `/bot-me` -> `identify_actor`
- `/bot-status` -> `runtime_status`
- `/bot-runs` -> `list_runs`
- `/bot-run dry <dataset>` -> `submit_dry_run`
- 附件消息 -> `data_intake`
- 普通文本 -> `chat`

后续自然语言由 LLM planner 识别，但输出仍然必须符合 Intent schema。

## 4. Skill

Skill 是流程知识，不是执行器。它可以告诉 Agent：

- 需要哪些字段。
- 如何判断风险。
- 什么时候需要审批。
- 用哪些 tool。
- 失败后如何恢复。

建议 Skill 目录：

```text
skills/
  channel-data-intake/
    SKILL.md
  qq-remote-operator/
    SKILL.md
  data-to-deployment-lifecycle/
    SKILL.md
```

示例：

| Skill | 场景 | 典型 Tool |
| --- | --- | --- |
| `channel-data-intake` | QQ 上传图片/zip/manifest | `intake.quarantine`、`intake.scan`、`vlm.inspect` |
| `data-to-deployment-lifecycle` | 从数据到部署的 dry-run / 正式 run | `workflow.submit_run`、`dataset.register`、`model.evaluate` |
| `qq-remote-operator` | QQ 远程查询、运行、审批 | `runtime.status`、`workflow.list_runs`、`approval.decide` |

## 5. Tool

Tool 是最小能力单元，必须带 contract：

```json
{
  "id": "workflow.submit_run",
  "input_schema": "schema://workflow.submit_run.input.v1",
  "risk_level": "medium",
  "permission_scopes": ["workflow:run"],
  "approval_required": false,
  "sandbox_policy": "control-plane-only"
}
```

当前建议工具分组：

| Tool | 作用 | 风险 |
| --- | --- | --- |
| `runtime.health` | 查询 runtime 健康 | low |
| `runtime.identify_actor` | 返回 channel sender 身份 | low |
| `workflow.list_runs` | 查询最近 run | low |
| `workflow.submit_run` | 提交 dry-run 或 workflow | medium |
| `intake.quarantine` | 附件进入隔离区 | medium |
| `intake.scan` | 扫描 zip/manifest/图片 | medium |
| `dataset.register` | 注册数据集 | high |
| `model.promote` | 发布模型 | high |
| `deployment.rollback` | 回滚部署 | high |

高风险 tool 必须 Approval。

## 6. MCP

MCP 用来接外部工具/服务，不应该替代业务边界。

适合 MCP 的能力：

- 文件系统只读扫描。
- 数据库查询。
- 对象存储。
- 浏览器自动化。
- 外部模型服务。
- 远程训练集群。

不适合 MCP 的能力：

- 核心权限判断。
- 业务状态机。
- 数据集版本注册的 source of truth。
- 审批和审计。

这些必须留在 Go control plane。

## 7. QQ 示例

用户在 QQ 发送：

```text
@Agent 这是今天采集的数据，帮我检查并注册
```

并上传 `dataset.zip`。

链路：

```text
QQ -> NapCat -> OneBot event
  -> qqbot.NormalizeEvent
  -> IntentRouter: data_intake
  -> SkillResolver: channel-data-intake
  -> Plan:
       intake.quarantine
       intake.scan
       llm/vlm.inspect
       dataset.register dry-run
       approval if accepted
  -> ReplyRouter: 回 QQ
```

如果用户发送：

```text
/bot-run dry shanghaitech-original
```

链路：

```text
Intent: submit_dry_run
Skill: data-to-deployment-lifecycle
Tool: workflow.submit_run
Governance: restricted tool allowed
Executor: agentapp.SubmitWorkflowRun
Reply: run id + task id
```

## 8. 防止代码变成耦合堆的规则

- Channel adapter 不允许直接 import `datasetapp`、`workflowapp`、`modelrepo`。
- Intent router 不允许执行工具。
- Skill resolver 不允许直接写数据库。
- Tool executor 必须经过 Governance preflight。
- MCP server 返回结果必须经过 schema validation。
- 所有 channel-origin action 必须写 audit。

## 9. 当前实现状态

已实现：

- `internal/app/agentruntime/intent.go`：规则意图识别。
- `internal/app/agentruntime/service.go`：最小 Agent Runtime。
- `workers/python/agent_runtime`：Python Agent Runtime prototype。
- `internal/infrastructure/qqbot/onebot.go`：OneBot/NapCat 事件归一化。
- `POST /api/channels/qq/onebot`：QQ/NapCat 入站验证接口。

未实现：

- LLM planner。
- Skill resolver。
- Tool registry executor。
- MCP executor。
- Approval queue。
- NapCat outbound sender 自动回发。

## 10. SDD 测试

| ID | 场景 | 验收标准 |
| --- | --- | --- |
| ITM-001 | `/bot-run dry xxx` | 识别为 `submit_dry_run`，映射 `workflow.submit_run`。 |
| ITM-002 | QQ 上传图片 | 识别为 `data_intake`，映射 `channel-data-intake`。 |
| ITM-003 | 自然语言训练请求 | LLM planner 输出结构化 Intent，不直接执行。 |
| ITM-004 | 高风险发布请求 | 计划进入 Approval，不调用 deployment tool。 |
| ITM-005 | MCP 工具返回非 schema 数据 | Executor 拒绝并记录 audit。 |
