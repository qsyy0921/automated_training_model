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

参考项目取舍见 [REFERENCE_AGENT_RUNTIME_ALIGNMENT.md](REFERENCE_AGENT_RUNTIME_ALIGNMENT.md)。本项目吸收 OpenClaw 的 Gateway/channel/plugin 边界、cc 的 CLI-first agent loop/permission/MCP 分层、Hermes 的 Python tools/skills/runtime 生态，但不照搬任何一个项目的整体技术栈。

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
  service.go       Agent Runtime 门面，暴露 HandleChannelMessage 和运行态快照
  session.go       SessionRunner，负责 session key、规划调用、工具执行和 trace 记录
  planner.go       默认规则 Planner，可离线运行并输出 ToolCall 计划
  python_planner.go 可选 Python Planner 适配器，通过 AGENT_RUNTIME_PLANNER=python 启用
  tools.go         Go ToolExecutor，执行 runtime、workflow、intake、vision、llm.plan 最小工具
  store.go         内存 session/trace store，支撑 /api/runtime/sessions 和 /api/runtime/traces
  intent.go        规则意图识别
  subagent.go      sub-agent 路由决策

workers/python/agent_runtime/
  main.py          Python Agent Runtime prototype，可输出 JSON plan/tool_calls
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
    GET  /api/runtime/status
    GET  /api/runtime/sessions
    GET  /api/runtime/traces
```

当前实现已经是可运行的最小完整 runtime：通道消息进入后会归一化为 `channel.InboundMessage`，进入 `SessionRunner`，生成 session key，调用 Planner 输出直接回复或 ToolCall，再由 ToolExecutor 执行，并把 session 与 trace 写入内存 store。下一步再加入持久 session、真实 LLM planner、approval queue 和长期运行的 OneBot WebSocket reader。

默认模式不依赖外部模型：

```powershell
$env:AGENT_RUNTIME_PLANNER="rule"
```

如需切换到 Python runtime prototype：

```powershell
$env:AGENT_RUNTIME_PLANNER="python"
$env:AGENT_RUNTIME_PYTHON="python"
$env:AGENT_RUNTIME_PYTHONPATH="F:\automated_training_model\workers\python"
```

Mimo / VLM provider key 只允许放在服务端环境变量或 SecretRef 中，不能写入仓库、前端代码或 channel payload。
HuggingFace 模型安装不由 Codex 直接执行；Codex 只维护 prompt、skill、tool contract 和测试入口。实际安装流程必须由 Agent Runtime 调用 Mimo 输出 `model.download_hf` / `model.verify_hf` tool-call plan，再由 Go ToolExecutor 受控执行。Mimo 安装提示词见 [AGENT_RUNTIME_MIMO_INSTALL_PROMPT.md](AGENT_RUNTIME_MIMO_INSTALL_PROMPT.md)。

当前本机开发策略是 Agent Runtime 默认拥有执行权限：`model.download_hf` 可以直接执行真实下载，但仍被限制在 `data_lake/models/artifacts/huggingface` 目录内，不能写入 Git 路径或任意路径。需要收紧权限时，服务端设置 `AGENT_RUNTIME_REQUIRE_MODEL_DOWNLOAD_APPROVAL=true`，此时真实下载必须由 tool-call params 显式包含 `approved=true`。

Mimo 路由规则：
- `mimo-v2.5-pro`：文本意图识别、任务规划、tool-call JSON、workflow reasoning。
- `mimo-v2.5`：视觉理解，处理 QQ 图片、截图、异常帧、样例图和其他需要 VLM 的附件。
- Python planner 会根据 `delegation.model_route=vision` 或图片附件自动选择 `MIMO_VISION_MODEL` / `VLM_MODEL`，默认值为 `mimo-v2.5`。

### 6.1 当前 Runtime 能力闭环

| 能力 | 当前实现 | 后续替换点 |
| --- | --- | --- |
| Session | `DefaultSessionKey(agentId, channel, peer)` + `InMemoryRuntimeStore` | 持久化到 SQLite/Postgres，并增加 context summary |
| Intent | Go `ClassifyIntent` 规则层 | Python/Mimo planner 做二级语义识别 |
| Planner | 默认 `RulePlanner`，可选 `PythonPlanner` | 接入 Mimo 2.5 Pro 输出结构化 JSON plan |
| Tool Executor | `GoToolExecutor` 支持 runtime、workflow、intake、vision、llm.plan 最小工具 | 拆到 `internal/app/toolapp`，增加 schema、permission、approval |
| Model Install | `model.download_hf` / `model.verify_hf` 只能由 Mimo plan 触发并限制在 data_lake；本机开发默认全权限，可用 `AGENT_RUNTIME_REQUIRE_MODEL_DOWNLOAD_APPROVAL=true` 收紧 | 后续接入模型注册、下载任务状态和断点续传 UI |
| Trace | 每条消息写入 `TraceEvent` | 持久化、检索、成本统计、skill mining 输入 |
| Observability | `/api/runtime/status`、`/api/runtime/sessions`、`/api/runtime/traces` | Web/CLI/桌面端统一展示 |
| Channel | QQ/NapCat webhook/test-message | OneBot WebSocket reader、Telegram、飞书 |

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

## 9. 当前验证记录

日期：2026-06-02

- Mimo API smoke 已通过：`mimo-v2.5-pro` 和 `mimo-v2.5` 都能通过 `C:\Users\10495\Desktop\mimo.txt` 加载到服务端环境变量后访问；测试脚本只输出模型名、HTTP 状态和响应摘要，不打印密钥。
- Mimo 路由已验证：文本规划默认走 `mimo-v2.5-pro`，视觉附件或 `delegation.model_route=vision` 走 `mimo-v2.5`。
- Mimo planner 安装请求已验证：用户请求安装 `nvidia/LocateAnything-3B` 时，planner 输出 `model.download_hf` tool-call plan，而不是直接输出 shell 命令。
- Go ToolExecutor 权限边界已验证：本机开发默认允许 Agent Runtime 执行 `model.download_hf`；设置 `AGENT_RUNTIME_REQUIRE_MODEL_DOWNLOAD_APPROVAL=true` 后才会返回 `approval_required`，并要求 `approved=true`。
- ShanghaiTech original 数据目录已验证存在：`F:\automated_training_model\data_lake\raw\datasets\shanghaitech\original`，顶层包含 `training`、`testing`、`testframemask`。
- ShanghaiTech 测试计划已验证：当用户要求用 ShanghaiTech original 测试 LocateAnything-3B 时，runtime 会规划 `model.verify_hf` + `workflow.submit_run(dry_run=true)`，并生成 trace；真实推理仍依赖模型权重下载、依赖安装和显存条件。
- HuggingFace downloader skill dry-run 已通过：`nvidia/LocateAnything-3B` 的下载路径限制在 `data_lake/models/artifacts/huggingface/nvidia/LocateAnything-3B`，manifest 路径为 `data_lake/catalog/models/nvidia_LocateAnything-3B.download.json`；dry-run 不创建权重目录。
- 新增 `ops/scripts/smoke-mimo-planner.ps1`：用于验证 Mimo planner 对 LocateAnything 安装请求和 ShanghaiTech dry-run 请求输出受控 tool-call plan。
- 中断残留的 `LocateAnything-3B` 不完整下载目录已删除；当前仓库不应提交模型权重、checkpoint、HF cache、API Key 或 provider token。

当前仍未完成的验收项：

- 尚未由 Agent Runtime 正式执行真实 `model.download_hf` 下载完整 `nvidia/LocateAnything-3B`。
- 尚未完成 LocateAnything-3B 的加载 smoke 或真实 ShanghaiTech 推理。
- QQ/NapCat 真实账号回发需要本机 NapCat 登录态和 outbound 环境变量，本仓库 smoke 先覆盖 HTTP test-message / OneBot webhook 闭环。
