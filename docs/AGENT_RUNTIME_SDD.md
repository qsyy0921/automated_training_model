# Agent Runtime SDD

版本：v0.1  
日期：2026-06-02  
范围：设计本机 Agent Runtime，承接 Web、CLI、桌面端、QQ/NapCat 以及后续 Channel 的远程消息。

## 1. 目标

Agent Runtime 是所有入口共享的运行时核心。它负责把远程入口来的消息路由到本机合适的 Agent session，并把执行结果回传给原入口。

```text
Web / CLI / Desktop / QQ / Telegram / Feishu
  -> Gateway
  -> Channel Adapter
  -> Channel Router
  -> Intent Router
  -> Agent Session
  -> Tool / Workflow / Data Intake
  -> Reply Router
```

当前 MVP 先支持 QQ/NapCat 的文本通信验证。长期方案采用 **Go Gateway + Python Agent Runtime**：Go 负责远程连接、鉴权、治理和审计；Python 负责 LLM planner、多模态理解、skill 选择和 tool-call plan。

## 2. 为什么不能让 QQ Adapter 直接调业务

QQ Adapter 只知道 QQ/OneBot 协议，不应该知道数据集、训练、评估、部署细节。否则后续接 Telegram、飞书、桌面端时，每个入口都会复制一套业务逻辑，最后形成耦合。

严格边界：

- QQ Adapter：OneBot event -> `channel.InboundMessage`，`channel.OutboundMessage` -> OneBot send payload。
- Channel Router：身份、账号、peer、群策略、session key。
- Intent Router：命令、自然语言、附件上传、审批意图。
- Agent Runtime：创建/恢复 session，调用 planner，输出 workflow 或 data intake 的结构化计划。
- Tool / Workflow：真正执行任务。

## 2.1 语言边界

| 模块 | 推荐语言 | 原因 |
| --- | --- | --- |
| Gateway / Channel Router / Audit | Go | 长连接、HTTP API、并发、部署和控制面稳定。 |
| Agent Loop / Planner / Skill Resolver | Python | LLM、VLM、训练、评估和数据科学生态更完整。 |
| Tool Executor control plane | Go + Python | Go 做权限和状态，Python 做模型/数据执行。 |
| 高性能热点 | Rust / WASM | IoU、mask/RLE、轨迹几何、大文件解析。 |
| Web / 桌面端 UI | TypeScript / React | 复用现有 Web 工程和组件。 |

Go 中的 `internal/app/agentruntime` 是最小控制面 shim，用来验证 Channel 通信和规则命令。真正的 LLM-heavy runtime 会放在 `workers/python/agent_runtime`，通过 JSON envelope 被 Go 调用。

## 3. QQ 消息如何转发给本机 Agent

以本机挂 QQ + NapCat 为例：

```text
手机/远程 QQ 发送消息
  -> 腾讯 QQ 网络
  -> 本机登录的 QQ 号
  -> NapCat 捕获消息并转成 OneBot 事件
  -> POST http://127.0.0.1:7870/api/channels/qq/onebot
  -> qqbot.NormalizeEvent(...)
  -> AgentRuntime.HandleChannelMessage(...)
  -> 返回 OneBot send_msg payload
  -> 如果启用 QQ_ONEBOT_OUTBOUND_ENABLED，再主动 POST 到 NapCat /send_msg
```

如果 NapCat 使用反向 WebSocket，后续可以改成：

```text
NapCat WebSocket
  -> qqbot runtime reader
  -> AgentRuntime
  -> qqbot outbound sender
```

核心数据结构不变。

## 4. 意图识别策略

意图识别分两级，不一上来就全交给 LLM：

### 4.1 规则层

低风险、明确命令用规则识别：

| 输入 | 意图 | 动作 |
| --- | --- | --- |
| `/bot-ping` | health check | 直接回 pong |
| `/bot-me` | identity check | 返回 channel/account/peer/sender |
| `/bot-status` | runtime status | 返回 Gateway/Runtime 状态 |
| `/bot-runs` | list runs | 查询最近 Agent runs |
| `/bot-run dry <dataset>` | dry-run workflow | 提交 `data-to-deployment-lifecycle` dry-run |

规则层的好处是可测、可审计、不会因为模型幻觉触发危险动作。

### 4.2 LLM 层

自然语言和数据上传进入 LLM planner：

```text
用户消息 + 附件 metadata + session context
  -> LLM / VLM
  -> Structured Plan
  -> Governance
  -> Execute or Approval
```

LLM 只能输出结构化计划，不能直接执行命令。

计划类型：

- `chat`
- `submit_workflow_dry_run`
- `create_data_intake_plan`
- `ask_followup`
- `request_approval`

Intent 到 Tool / Skill / MCP 的完整映射规则见 [INTENT_TOOL_SKILL_MCP_SDD.md](INTENT_TOOL_SKILL_MCP_SDD.md)。

## 5. Session Key

同一个远程对话必须落到稳定 session：

```text
agent:<agentId>:qq:direct:<user_id>
agent:<agentId>:qq:group:<group_id>
```

后续 Telegram / 飞书只换 channel 名：

```text
agent:<agentId>:telegram:group:<chat_id>
agent:<agentId>:feishu:channel:<open_chat_id>
```

## 6. 当前代码落点

```text
internal/app/agentruntime/
  service.go       Agent Runtime MVP，处理 Channel message 和 /bot-* 命令

workers/python/agent_runtime/
  main.py          Python Agent Runtime prototype
  intent.py        Python intent classifier
  contracts.py     Runtime request/result JSON contract

internal/infrastructure/qqbot/
  onebot.go        NapCat / OneBot 事件归一化和 send_msg payload 构造

internal/api/httpapi/
  channel_handlers.go
    GET  /api/channels
    GET  /api/channels/qq/status
    POST /api/channels/qq/test-message
    POST /api/channels/qq/onebot
```

这只是最小可测通信链路。下一步再加入持久 session、LLM planner、approval queue、NapCat outbound sender。

## 7. 本机 QQ + NapCat 验证方案

推荐使用测试 QQ 号：

1. 在本机登录测试 QQ。
2. 启动 NapCat，并启用 OneBot HTTP 事件上报。
3. 将上报地址配置为：

```text
http://127.0.0.1:7870/api/channels/qq/onebot
```

4. 启动 Go Gateway：

```powershell
go run .\cmd\labelserver -addr 127.0.0.1:7870 ...
```

5. 用另一个 QQ 号给本机测试 QQ 发消息：

```text
/bot-ping
/bot-me
/bot-status
/bot-runs
/bot-run dry shanghaitech-original
```

当前 HTTP handler 会返回 `onebot_reply`，用于验证 Agent Runtime 产出的回复结构。启用以下环境变量后，会主动调用 NapCat `send_msg`：

```powershell
$env:QQ_ONEBOT_OUTBOUND_ENABLED="true"
$env:QQ_ONEBOT_HTTP_URL="http://127.0.0.1:3000"
$env:QQ_ONEBOT_ACCESS_TOKEN="replace_me_if_napcat_requires_token"
```

如果没有开启 outbound，接口只返回调试 JSON，不主动发回 QQ。

## 8. SDD 测试

| ID | 场景 | 验收标准 |
| --- | --- | --- |
| ART-001 | OneBot 私聊 `/bot-ping` | 返回 `onebot_reply.action=send_msg`，message 为 `pong`。 |
| ART-002 | OneBot 群聊图片消息 | 归一化为 group peer，附件进入 `attachments`。 |
| ART-003 | `/bot-run dry shanghaitech-original` | 创建 dry-run Agent run，params 包含 `source=qq`。 |
| ART-004 | 普通文本 | 进入 Agent Runtime，返回当前 runtime 能力说明。 |
| ART-005 | 附件消息 | 返回 Data Intake Plan/quarantine 提示，不直接写 Data Lake。 |
| ART-006 | 后续 Telegram/飞书 | 只能新增 adapter，不能修改 Agent Runtime 核心行为。 |
