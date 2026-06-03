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

Gateway 访问保护位于 Channel Adapter 和 Agent Runtime 之前。当前 `internal/infrastructure/middleware.GatewayAuth` 保护 `/api/`：loopback 默认可本机开发，非 loopback 请求必须配置并携带 `ATM_GATEWAY_TOKEN` / `GATEWAY_AUTH_TOKEN`；`ATM_ALLOWED_ORIGINS` / `GATEWAY_ALLOWED_ORIGINS` 控制跨域来源。该 token 只用于 Gateway 连接，不等同于 Mimo/HF/API Key，也不会在 runtime status 中明文返回。

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

如果 NapCat 使用 OneBot 正向 WebSocket，当前 Gateway 可选启用长连接：

```text
NapCat WebSocket
  -> qqbot WebSocket reader (QQ_ONEBOT_WS_ENABLED=true)
  -> AgentRuntime
  -> 同一 WebSocket 写回 send_msg action
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
| `你好 / 你是谁 / 你能做什么` | runtime_about | Go 本地返回 Agent Runtime 身份和能力 |
| `下载 LocateAnything-3B` | model_install | Go 本地生成 `model.download_hf` 受控计划 |
| `LocateAnything-3B 测试 ShanghaiTech` | model_test | Go 本地生成 verify / smoke / dry-run 固定工具链 |

规则层的好处是可测、可审计、不会因为模型幻觉触发危险动作。

本地语义 fast-path 只覆盖高置信度固定流程。默认开启 `AGENT_RUNTIME_LOCAL_SEMANTIC_FASTPATH=true`；如果需要强制让 Mimo 生成模型下载/测试 tool-call plan，可设置为 `false`。

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
  tools.go         Go ToolExecutor，注册 runtime、workflow app、intake app、vision、llm.plan、modelruntime handler adapter
  store.go         RuntimeStore 端口和内存开发实现，支撑 /api/runtime/sessions 和 /api/runtime/traces
  intent.go        规则意图识别
  subagent.go      sub-agent 路由决策

internal/app/toolapp/
  schema.go        Tool schema、risk level、allowed params、approval/preflight gate
  runner.go        Tool runner，负责 preflight、handler dispatch、结果合并和未注册 handler 拦截

internal/app/runtimeworkflow/
  service.go       workflow.list_runs / workflow.submit_run dry-run app service

internal/app/modelruntime/
  service.go       model.download_hf / model.verify_hf / model.smoke_locateanything 的参数规范化、路径安全、脚本执行和 smoke 解析

internal/infrastructure/runtimerepo/
  json_store.go    JSON RuntimeStore 适配器，默认持久化 session/trace 到 data_lake/runtime

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

当前实现已经是可运行的最小完整 runtime：通道消息进入后会归一化为 `channel.InboundMessage`，进入 `SessionRunner`，生成 session key，`RuntimeRouter` 先选择 `local_control`、`local_semantic` 或 `external_planner`。低风险控制意图先走 Go 本地 fast-path，高置信度固定流程走 Go 本地语义 fast-path，其他消息再调用 Planner 输出直接回复或 ToolCall，并由 `toolapp.Runner` 执行。Runner 先调用 `toolapp.Preflight`，检查工具是否注册、参数是否在 schema 内、高风险工具是否需要审批，然后分发到 `GoToolExecutor` 注册的 MVP handler。Go 计算出的 `go_intent` 会通过 Python request metadata 传给 `workers/python/agent_runtime`，避免 Python worker 重复粗分类；Python/Mimo 只做二级语义规划和参数细化。Data Intake / Vision 这类请求有 mandatory tool guard：Mimo/Python planner 必须输出精确单工具计划 `intake.plan` 或 `vlm.inspect`，否则 Go `SessionRunner` 会保留 Go 侧 sub-agent delegation 并回退本地 `RulePlanner`，避免模型临时扩展工具链或破坏必要的数据治理路径。Data Intake 的 quarantine、静态 scan、dry-run plan、pending approval workflow 已经外迁到 `internal/app/intakeapp`；纯文本远程指令会被记录为 synthetic text source attachment，仍然经过 scan 和 pending approval，不直接写入 Data Lake。runtime 只调用 intake app 并把 `workflow_id`、`plan_id`、scan 结果摘要和审批边界写入 trace metadata；HuggingFace 下载、校验和 LocateAnything smoke 的参数规范化、目录边界、脚本调用和结果解析已经外迁到 `internal/app/modelruntime`，runtime 只保留审批开关、异步 `ModelJob` 生命周期、cancel/resume 和 trace 适配。当前 `model.download_hf` 默认由 Go `ModelJob` 启动 `python -m agent_worker.main` 执行，worker 返回的 heartbeat、logs、artifacts、attempt/max_attempts、retryable 和 stdout/stderr 摘要都会写回同一份 `ModelJobStore`；worker 执行期间还会通过 `stderr` 输出结构化事件，Go runner 会实时把 heartbeat 与 `stdout/stderr` 行写回同一份 store。worker timeout、坏 JSON 和显式 failed 结果现在也会保留部分 stdout/stderr，并附带 `worker_error_kind` / `worker_error_retryable` metadata，避免 Web/CLI 只看到泛化的 execution failed。`model.verify_hf` 也支持显式 `job=true` 的 worker job 模式，且 `/bot-verify-hf-job [repo_id]` 已由 Go fast-path 直接排队后台 verify worker；`model.smoke_locateanything` 也支持显式 `job=true` 的 worker smoke 模式，但默认普通 `model.verify_hf` / `model.smoke_locateanything` 仍保持同步执行，避免打断现有 LocateAnything smoke 链。只有显式设置 `AGENT_RUNTIME_HF_DOWNLOAD_RUNNER=service` 或 `AGENT_RUNTIME_HF_DOWNLOAD_SYNC=true` 时才回退旧下载路径。服务启动时通过 `internal/infrastructure/intakerepo.JSONRepository` 把计划和 workflow 持久化到 `data_lake/runtime/intake`。session、trace、model job 写入 `RuntimeStore` / `ModelJobStore`，服务启动时默认持久化到 `data_lake/runtime`；测试脚本会使用 `tmp/runtime-smoke-*` 做重启恢复验证。QQ/NapCat 已支持 HTTP webhook/test-message/outbound 和可选 OneBot WebSocket reader。下一步再加入 approval queue、正式 workflow/task repository 和真实账号群聊 @Bot 实测。

低延迟策略参考 ccb / Hermes / OpenClaw 的工程做法：明确命令不等待模型，项目身份不让模型自由发挥，高置信度工程意图先路由到受控 tool，长任务不阻塞入口，普通 chat 用流式首包改善体感，复杂任务才进入结构化 planner。当前 fast-path 覆盖 `/bot-ping`、`/bot-me`、`/bot-status`、`/bot-runs`、`/bot-run dry`、`/bot-verify-hf-job`、`/bot-train-dry`、`/bot-help`、runtime self-description、`规划 ShanghaiTech 数据接入`、已知 LocateAnything 下载和 ShanghaiTech smoke 固定工具链；这些请求即使启用了 `AGENT_RUNTIME_USE_MIMO=true` 也不会等待 Python/Mimo。

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
HuggingFace 模型安装不由 Codex 直接执行；Codex 只维护 prompt、skill、tool contract 和测试入口。实际安装流程必须由 Agent Runtime 生成 `model.download_hf` / `model.verify_hf` tool-call plan，再由 Go ToolExecutor 受控执行。未知模型或不完整参数交给 Mimo planner；已知 `nvidia/LocateAnything-3B` 固定流程可由 Go 本地语义 fast-path 直接生成受控计划。Mimo 安装提示词见 [AGENT_RUNTIME_MIMO_INSTALL_PROMPT.md](AGENT_RUNTIME_MIMO_INSTALL_PROMPT.md)。

当前本机开发策略是 Agent Runtime 默认拥有执行权限：`model.download_hf` 可以创建真实下载任务，但必须以异步 `ModelJob` 形式运行，写入仍被限制在 `data_lake/models/artifacts/huggingface` 目录内，不能写入 Git 路径或任意路径。需要收紧权限时，服务端设置 `AGENT_RUNTIME_REQUIRE_MODEL_DOWNLOAD_APPROVAL=true`，此时真实下载必须由 tool-call params 显式包含 `approved=true`。

工具 preflight 有两层：

- `internal/app/toolapp`：默认检查工具注册、参数 schema、risk level 和通用高风险审批开关 `AGENT_RUNTIME_REQUIRE_HIGH_RISK_TOOL_APPROVAL=true`。
- `agentruntime.GoToolExecutor`：保留 runtime 侧审批、异步 job 生命周期和 handler 注册；`workflow.submit_run` 的 dry-run 规则由 `runtimeworkflow` 负责，模型目录边界由 `modelruntime` 负责。

Mimo 路由规则：
- `mimo-v2.5-pro`：文本意图识别、任务规划、tool-call JSON、workflow reasoning。
- `mimo-v2.5`：视觉理解，处理 QQ 图片、截图、异常帧、样例图和其他需要 VLM 的附件。
- Python planner 会根据 `delegation.model_route=vision` 或图片附件自动选择 `MIMO_VISION_MODEL` / `VLM_MODEL`，默认值为 `mimo-v2.5`。

### 6.1 当前 Runtime 能力闭环

| 能力 | 当前实现 | 后续替换点 |
| --- | --- | --- |
| Session | `DefaultSessionKey(agentId, channel, peer)` + `RuntimeStore`，默认 JSON 持久化到 `data_lake/runtime` | 迁移到 SQLite/Postgres，并增加 context summary |
| Intent / Router | Go `ClassifyIntent` + `RuntimeRouter` 规则层；控制命令和高置信度固定语义走本地 fast-path；`go_intent` 传入 Python metadata | Python/Mimo planner 做二级语义识别和参数细化 |
| Planner | 默认 `RulePlanner`，可选 `PythonPlanner`；控制命令启用本地 `RulePlanner` 快速计划 | 接入 Mimo 2.5 Pro 输出结构化 JSON plan |
| Tool Schema / Preflight | `internal/app/toolapp` 支持 tool registry、allowed params、risk、approval gate | 接入持久 tool registry 和人工审批队列 |
| Tool Runner | `internal/app/toolapp.Runner` 负责 preflight、handler dispatch、结果合并和未注册 handler 拦截 | 增加 handler registry 持久化和审批队列联动 |
| Tool Executor | `GoToolExecutor` 当前只注册 runtime、workflow app 调用、intake app 调用、vision 调用、llm.plan、modelruntime adapter | 将 model job 生命周期迁移到 task/model worker，将 workflow app 接到 workflow/task repository |
| Runtime Workflow | `internal/app/runtimeworkflow` 管理 `workflow.list_runs` / `workflow.submit_run` 的 dry-run 规则、RunRequest 构造和回复格式 | 后续替换 agent app 内存队列，接入持久 task repository、日志和 artifacts |
| Model Runtime | `internal/app/modelruntime` 管理 `model.download_hf` / `model.verify_hf` / `model.smoke_locateanything` 的参数默认值、路径白名单、Python 脚本调用、超时和 smoke JSON 解析；已知 LocateAnything 固定流程可由 Go 本地语义 fast-path 生成计划；`model.download_hf` 默认进入异步 `ModelJob` 并启动 Python worker，`model.verify_hf` / `model.smoke_locateanything` 可显式 `job=true` 进入 worker job，可用 `AGENT_RUNTIME_REQUIRE_MODEL_DOWNLOAD_APPROVAL=true` 收紧，`AGENT_RUNTIME_HF_DOWNLOAD_RUNNER=service` 回退旧下载路径；worker timeout / decode failure 会把 stdout、stderr 和错误类型带回 Go store，worker 运行中的结构化 heartbeat / `stdout/stderr` 行也会持续写回，worker artifacts 会额外归档到 runtime artifact manifest | 后续接入模型注册、统一 task repository、逐文件进度、原始字节流直通和断点续传 UI |
| Training Runtime | 当前先通过 `training.run(dry_run)` 进入 Python worker `ModelJob`，由 `/bot-train-dry <dataset_id> [target_task] [model_family]` 触发，返回 dry-run artifact 和 heartbeat | 后续接入真实训练 recipe、GPU 调度、checkpoint artifact 和统一 task repository |
| Data Intake Workflow | `internal/app/intakeapp` 生成 quarantine、静态 scan、`intake.plan` / `vlm.inspect` dry-run 计划和 pending approval workflow；`internal/infrastructure/intakerepo.JSONRepository` 默认持久化到 `data_lake/runtime/intake`；ShanghaiTech 数据附件会在 trace metadata 中记录 `workflow_id`、`plan_id`、`dataset_name`、`source_uri`、`dry_run` 和审批边界 | 将 JSON MVP 推进到真实文件隔离区、深度 scan、审批队列和正式 dataset registry 写入 |
| Trace | 每条消息写入 `TraceEvent`，JSON MVP 可跨重启恢复 | 检索、成本统计、skill mining 输入 |
| Observability | `/api/runtime/status`、`/api/runtime/sessions`、`/api/runtime/traces` | Web/CLI/桌面端统一展示 |
| Channel | QQ/NapCat webhook/test-message/outbound；可选 OneBot WebSocket reader | 真实账号群聊 @Bot 实测、Telegram、飞书 |

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
| ART-005 | 附件消息 | 通过 ToolExecutor 生成 Data Intake Workflow 或视觉检查 Workflow，trace 包含 `workflow_id`、`intake.plan` / `vlm.inspect`，不直接写正式 Data Lake。 |
| ART-006 | 后续 Telegram/飞书 | 只能新增 adapter，不能修改 Agent Runtime 核心行为。 |
| ART-006b | 启用 Mimo 后发送 `/bot-ping` | 仍由 Go 本地 fast-path 返回 `pong`，不调用 Python/Mimo planner。 |

## 9. 当前验证记录

日期：2026-06-02

- Mimo API smoke 已通过：`mimo-v2.5-pro` 和 `mimo-v2.5` 都能通过 `C:\Users\10495\Desktop\mimo.txt` 加载到服务端环境变量后访问；测试脚本只输出模型名、HTTP 状态和响应摘要，不打印密钥。
- Mimo 路由已验证：文本规划默认走 `mimo-v2.5-pro`，视觉附件或 `delegation.model_route=vision` 走 `mimo-v2.5`。
- Mimo planner 安装请求已验证：用户请求安装 `nvidia/LocateAnything-3B` 时，planner 输出 `model.download_hf` tool-call plan，而不是直接输出 shell 命令。
- Go ToolExecutor 权限边界已验证：本机开发默认允许 Agent Runtime 执行 `model.download_hf`；设置 `AGENT_RUNTIME_REQUIRE_MODEL_DOWNLOAD_APPROVAL=true` 后才会返回 `approval_required`，并要求 `approved=true`。
- ShanghaiTech original 数据目录已验证存在：`F:\automated_training_model\data_lake\raw\datasets\shanghaitech\original`，顶层包含 `training`、`testing`、`testframemask`。
- ShanghaiTech 测试计划已验证：当用户要求用 ShanghaiTech original 测试 LocateAnything-3B 时，runtime 会规划 `model.verify_hf` + `model.smoke_locateanything` + `workflow.submit_run(dry_run=true)`，并生成 trace；真实推理仍依赖 GPU/runtime 条件。
- ShanghaiTech Channel 数据附件 smoke 已验证：`smoke-runtime-mvp.ps1` 会把 `F:\automated_training_model\data_lake\raw\datasets\shanghaitech\original` 作为附件源发送到 QQ test-message，runtime 生成 `intake.plan` trace 和 intake workflow，并在 metadata 记录 `workflow_id`、`dataset_name=shanghaitech-original` 与 `source_uri`。
- HuggingFace downloader skill dry-run 已通过：`nvidia/LocateAnything-3B` 的下载路径限制在 `data_lake/models/artifacts/huggingface/nvidia/LocateAnything-3B`，manifest 路径为 `data_lake/catalog/models/nvidia_LocateAnything-3B.download.json`；dry-run 不创建权重目录。
- Agent Runtime 真实下载已通过：`runtime-hf-install.ps1 -StartDownload -WaitForCompletion` 通过 Mimo planner 触发 `model.download_hf`，job `model-job-1780371206860804000` 最终 `succeeded`。
- HuggingFace verify-only 已通过：`download_hf_snapshot.py --verify-only` 显示 `complete=true`、`missing_files=[]`、远端文件数 38、远端总字节 7,795,875,224；本地模型目录仍位于 ignored 的 `data_lake/` 下。
- Runtime verify worker job smoke 已通过：`smoke-hf-verify-worker.ps1` 通过 `/bot-verify-hf-job nvidia/LocateAnything-3B` 直接触发 `model.verify_hf job=true`，trace、job heartbeat 和 artifacts 都能从同一份 runtime store 读回。
- Runtime training dry-run worker smoke 已通过：`smoke-training-dry-worker.ps1` 通过 `/bot-train-dry shanghaitech-original detection yolo11n` 直接触发 `training.run` 的 Python worker dry-run，trace、job heartbeat 和 dry-run artifact 都能从同一份 runtime store 读回。
- LocateAnything-3B 模型加载 smoke 已通过：`smoke-locateanything-model.ps1` 经 Runtime 触发 `model.verify_hf`、`model.smoke_locateanything`、`workflow.submit_run(dry_run=true)`；报告显示 `AutoConfig`、`AutoProcessor`、safetensors shard 和 `AutoModel.from_pretrained` 通过，参数量 3,517,975,280。
- 新增 `ops/scripts/smoke-mimo-planner.ps1`：用于验证 Mimo planner 对 LocateAnything 安装请求和 ShanghaiTech dry-run 请求输出受控 tool-call plan。
- Mimo runtime smoke 已通过：`smoke-runtime-mvp.ps1 -UseMimoPlanner` 覆盖 Web/CLI/Desktop/QQ、planner-agent、vision-agent、data-intake-agent 和 ShanghaiTech source trace。

当前仍未完成的验收项：

- 尚未完成 LocateAnything-3B 的真实 ShanghaiTech 推理；当前 PyTorch 为 CPU-only，`real_inference=false`。
- QQ/NapCat 真实账号回发需要本机 NapCat 登录态和 outbound 环境变量，本仓库 smoke 先覆盖 HTTP test-message / OneBot webhook 闭环。

## 10. 2026-06-02 Runtime 长任务问题分析与修正

### 10.1 问题结论

这次真实触发 `nvidia/LocateAnything-3B` 下载时，下载体量约 7.26 GiB。原实现把 `model.download_hf` 当作同步 Tool 执行，导致 `labelctl runtime send ...`、QQ、Web 或桌面端入口都必须等待下载完成。

这暴露出的根因不是单纯 HTTPS、代理或 HuggingFace 问题，而是 Agent Runtime 的执行模型问题：长任务不能阻塞 Channel request。

同步执行的具体风险：

- 入口请求会长期占用，无法快速回包。
- 用户中断终端后，Go server、labelctl 或 Python downloader 可能继续留在后台。
- Runtime trace 只能在工具完成或失败后落地，无法在下载进行中展示状态。
- QQ/Web/CLI/桌面端无法统一观察同一个长任务。

### 10.2 参考项目对齐

- OpenClaw 的 Gateway 思路：入口只做接入、校验、路由和结果回传，长任务应进入任务系统。
- Claude Code / cc 的 QueryEngine + ToolExecutor 思路：Agent loop 产生 tool-call，ToolExecutor 负责执行 envelope、权限、状态和结果。
- Hermes 的 Gateway + Python runtime 思路：会话上下文、工具执行和外部平台连接分离，长耗时能力不应绑死入口 adapter。

本项目采用 `Go Gateway + Python Agent Runtime + Go ToolExecutor + Runtime Job Store` 的最小落地方式。

### 10.3 新执行契约

`model.download_hf` 默认仍拥有本机开发全权限，但不再同步下载：

1. Mimo planner 输出 `model.download_hf` tool-call。
2. Go ToolExecutor 校验 `repo_id`、`local_dir`、`manifest` 和 `data_lake` 写入边界。
3. ToolExecutor 创建 `ModelJob`，立即返回 `queued` 和 `job_id`。
4. Go worker runner 生成 job envelope，启动 `python -m agent_worker.main`，由 Python worker 调用 `skills/huggingface-model-downloader/scripts/download_hf_snapshot.py`。
5. `JSONModelJobStore` 将任务状态、进度、生命周期日志、heartbeat、artifacts、stdout/stderr 摘要、取消标记和 parent/resume 关系写入 `data_lake/runtime/model_jobs.json`，并把 worker-backed artifacts 归档到 `data_lake/runtime/artifacts/<job_id>.artifact_manifest.json`。
6. Web、CLI、桌面端和 QQ 后续都通过 runtime job 状态查询进度；CLI/Gateway 可请求取消或基于 HuggingFace cache 新建恢复任务；如需回退旧 service runner，可显式设置 `AGENT_RUNTIME_HF_DOWNLOAD_RUNNER=service`。

如果服务重启时存在 `queued` 或 `running` 的模型任务，JSON store 会把它们恢复为 `interrupted` 并标记 `resumable=true`，避免 UI 误判后台 worker 仍在执行。HuggingFace snapshot cache 本身支持断点续传；用户通过 `resume-job` 新建恢复任务后可继续利用本地 cache。

新增可观测入口：

```text
GET /api/runtime/model-jobs
GET /api/runtime/model-jobs/{job_id}
POST /api/runtime/model-jobs/{job_id}/cancel
POST /api/runtime/model-jobs/{job_id}/resume
labelctl runtime model-jobs
labelctl runtime job <job_id>
labelctl runtime cancel-job <job_id>
labelctl runtime resume-job <job_id>
```

如确实需要调试同步模式，可显式设置：

```powershell
$env:AGENT_RUNTIME_HF_DOWNLOAD_SYNC="true"
```

生产或安全模式仍可收紧下载权限：

```powershell
$env:AGENT_RUNTIME_REQUIRE_MODEL_DOWNLOAD_APPROVAL="true"
```

### 10.4 SDD 测试补充

| ID | 场景 | 验收标准 |
| --- | --- | --- |
| ART-007 | 通过 runtime 请求下载 `nvidia/LocateAnything-3B` | 入口立即返回 `status=queued` 和 `job_id`，不等待 7GB 下载完成。 |
| ART-008 | 查询 `/api/runtime/model-jobs` | 返回模型下载任务，状态至少包括 `queued`、`running`、`succeeded`、`failed`、`interrupted`。 |
| ART-009 | CLI 查询 `labelctl runtime model-jobs` | 能看到与 Web/API 一致的任务列表。 |
| ART-010 | 用户中断 CLI/Web 请求 | 已经排队的后台任务由 Runtime job store 管理，入口中断不再等同于 runtime 会话失败。 |
| ART-011 | 设置 `AGENT_RUNTIME_REQUIRE_MODEL_DOWNLOAD_APPROVAL=true` | `model.download_hf` 返回 `approval_required`，不会创建后台下载任务。 |
| ART-012 | 服务重启后查询 `/api/runtime/model-jobs` | 已完成任务仍可恢复；重启前未完成任务标记为 `interrupted`。 |
| ART-013 | POST `/api/runtime/model-jobs/{job_id}/cancel` | running/queued 任务写入 `cancel_requested=true`，后台进程收到取消信号后进入 `canceled`，并保留 `resumable=true`。 |
| ART-014 | POST `/api/runtime/model-jobs/{job_id}/resume` | 对 `failed`、`interrupted`、`canceled` 的 `model.download_hf` 任务新建 child job，`parent_id` 指向原任务。 |

### 10.5 当前边界

- 当前 `ModelJobStore` 已有 JSON MVP 持久化，服务重启后不会丢失 job 记录；未完成任务会标记为 `interrupted/resumable`，但不会自动重新拉起 worker 进程，需要显式 `resume-job`。
- 下载脚本本身使用 HuggingFace snapshot cache 支持恢复；当前进度仍是 runtime 阶段进度，不是逐文件字节级进度。worker 运行中的 heartbeat 与结构化 `stdout/stderr` 行已经可以进入 job 日志流，但原始字节级直通和逐文件进度仍未完成。
- 后续应把 `model.download_hf`、默认同步的 `model.verify_hf`、训练、评估、部署统一迁移到 `internal/app/taskapp` 或持久化 workflow queue。
