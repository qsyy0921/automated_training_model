# Agent Runtime MVP SDD

版本：v0.1  
日期：2026-06-03

## 1. 目标

本 SDD 定义 `automated_training_model` 当前阶段的 Agent Runtime MVP。目标不是把视频审核页继续做大，而是把 Web、CLI、桌面端、QQ/NapCat 四类入口统一接入同一个 Agent Runtime，并让 Agent 能围绕“小模型从数据到部署”的流程完成可审计的规划、工具调用和结果追踪。

## 2. 范围

MVP 必须覆盖：

- 四入口统一：Web、CLI、桌面端、QQ/NapCat 都进入同一个 runtime；QQ 支持 test-message、HTTP webhook/outbound 和可选 OneBot WebSocket reader。
- Mimo 模型路由：文本规划走 `mimo-v2.5-pro`，视觉理解走 `mimo-v2.5`。
- 离线规则命令：`/bot-ping`、`/bot-me`、`/bot-status`、`/bot-runs`、`/bot-run dry`、`/bot-verify-hf-job`、`/bot-train-dry` 不依赖模型即可测试；即使启用 Mimo，也会走 Go 本地 fast-path。
- 本地语义 fast-path：自我介绍/能力说明、已知 `LocateAnything-3B` 下载、`LocateAnything-3B + ShanghaiTech` smoke 这类高置信度固定意图由 Go 直接生成受控计划，避免为了意图识别单独等待模型。
- Sub-agent 决策：普通文本、视觉附件、数据附件分别进入不同 agent 角色。
- ToolExecutor：所有副作用都通过工具出口，不能让 channel 或 UI 直接写数据湖、下载模型或提交训练。
- Runtime trace：每次会话、意图、工具调用、错误、metadata 都可通过 API/CLI/Web 观察。
- Gateway remote guard：本机 loopback 默认可开发调试，非 loopback `/api/` 访问必须配置并携带 Gateway token，allowed origins 由环境变量控制。
- HuggingFace 下载 skill：支持 dry-run、远端清单、断点续传、verify-only 和 Git 排除边界。
- ShanghaiTech original 数据 smoke：能识别数据源、生成 data intake workflow/plan trace，并明确真实推理阻塞点。

## 3. 非目标

当前 MVP 不承诺：

- 完整训练、评估、压缩、发布和线上监控闭环已经真实运行。
- QQ 真实账号群聊 @Bot 尚未完成端到端实测；当前仓库测试先覆盖 webhook/test-message/fake WebSocket，OneBot WebSocket reader 已有组件测试。
- `ModelJobStore` 已具备 JSON MVP 持久化、阶段进度、生命周期日志、日志查询/最小 NDJSON stream、取消请求和手动 resume；尚未具备逐文件字节级进度、真实 worker stdout/stderr 日志流和自动后台恢复。
- LocateAnything-3B 已完成真实 ShanghaiTech 推理；当前只完成下载、verify-only 和模型加载 smoke。
- skill 自进化默认关闭；当前支持手动生成草稿、列出草稿、写入 approve/reject 人工审批记录，但仍不会自动启用。

## 4. 分层设计

```text
Entry Points
  Web / CLI / Desktop / QQ-NapCat
    |
Gateway / Channel Adapter
  normalize message / account / peer / attachment
    |
Agent Runtime
  session / intent / sub-agent decision / planner / trace
    |
Tool Executor
  runtime tools / workflow tools / intake tools / model jobs
    |
Workers and Providers
  Python Mimo planner / VLM / HF downloader / future model workers
```

## 5. 运行时模块边界

| 模块 | 当前实现 | 责任 | 不能做 |
| --- | --- | --- | --- |
| Channel Adapter | `internal/api/httpapi/channel_handlers.go`、`internal/infrastructure/qqbot` | OneBot/test-message 归一化、outbound envelope | 不能写 Data Lake、不能调模型、不能绕过 runtime |
| Runtime Service | `internal/app/agentruntime/service.go` | 入口门面 | 不堆业务分支 |
| Session Runner | `session.go` | session key、router 选择、planner 调用、tool 调用、trace 写入 | 不直接下载模型或写数据 |
| Runtime Router | `router.go` | 参考 CCB/Hermes 的混合路由方式，先判定 local control、local semantic、external planner 三类路径 | 不执行工具，不保存状态 |
| Error Envelope | `errors.go`、`ports.go`、`internal/api/httpapi/server.go` | runtime stream 和 Gateway JSON error 的结构化错误契约：`code`、`message`、`source`、`retryable` | 保留旧 `error` 字符串兼容；不暴露 token、prompt 或密钥 |
| PlannerPort | `planner.go`、`python_planner.go` | 规则计划和 Python/Mimo 计划 | 不执行副作用 |
| Sub-agent Router | `subagent.go` | 决定是否委托 planner/vision/data-intake/training/skill-miner | 不绕过 approval |
| Tool Schema / Preflight | `internal/app/toolapp/schema.go` | tool registry、参数 schema、risk、approval/preflight | 不执行真实副作用 |
| Tool Runner | `internal/app/toolapp/runner.go` | preflight、handler dispatch、结果合并、未注册 handler 拦截、输出最小工具进度事件 | 不绑定 channel/session/runtime store |
| ToolExecutor | `tools.go` | 注册 MVP 工具 handler；`intake.plan` / `vlm.inspect` 只调用 `intakeapp`，`workflow.list_runs` / `workflow.submit_run` 只调用 `runtimeworkflow`，`model.*` 只调用 `modelruntime` 并管理异步 `ModelJob` 生命周期；worker-backed job 完成后会尝试归档 `artifact manifest` | 后续把 model job 生命周期迁移到 task/model worker，把 `runtimeworkflow` 接到正式 workflow/task repository |
| Model Runtime | `internal/app/modelruntime` | `model.download_hf` / `model.verify_hf` / `model.smoke_locateanything` 参数规范化、路径白名单、脚本调用、超时和 smoke JSON 解析 | 不持有 channel/session/trace；后续接统一模型任务 worker |
| Runtime Store | `store.go`、`model_jobs.go`、`internal/infrastructure/runtimerepo`、`internal/infrastructure/intakerepo` | sessions、traces、model jobs、dry-run intake plans/workflows | session/trace、model jobs、intake plans 和 intake workflows 默认 JSON 持久化；后续迁移到 task repository / intake repository |
| Gateway Middleware | `internal/infrastructure/middleware` | request id、CORS、recover、Gateway token auth、non-loopback access guard | 不读取模型密钥；不把 token 写入 status 或日志 |
| CLI Agent Shell | `internal/cli/labelctl/runtime_chat.go` | 参考 `ccb` / Claude Code / Hermes 的结构化 REPL：运行态面板、session、runtime snapshot、trace tree、doctor、raw JSON escape hatch、状态芯片和消息面板 | 不直接执行业务副作用；自然语言和 `/ping` 都进入同一个 Gateway runtime path |
| Web Agent Overview | `web/src/pages/agent-overview/AgentOverviewPage.tsx` | 展示 runtime status、sessions、traces、model jobs、model job logs、intake workflows 和 QQ test-message 入口 | 不直接写 Data Lake，不绕过 Gateway API |
| Python Runtime | `workers/python/agent_runtime` | Mimo fast chat、Mimo planner、guard plan、VLM 路由 | 不保存密钥到仓库 |
| Python Model Worker | `workers/python/agent_worker`、`internal/app/modelruntime/worker_runner.go` | 模型/数据任务的 worker envelope、`--health`、heartbeat、logs、artifact 引用、attempt/max_attempts/retryable 契约；当前 `model.download_hf` 默认经由 Go `ModelJob` 启动 `python -m agent_worker.main`，`dry_run=true` 与真实下载共用同一条 worker 路径；`model.verify_hf`、`model.smoke_locateanything` 和 `training.run(dry_run)` 也支持 worker job 模式，并把 worker 结果写回同一份 job store；timeout / decode failure 也会回写 stdout、stderr 和错误类型 metadata | 不拥有 Go task lifecycle；不直接写 runtime session/trace；真实训练/评估任务后续仍需统一迁移到 task runner |
| Skills | `skills/*` | 可复用操作说明和脚本 | 不提交权重或 token |

## 6. Sub-agent 使用规则

| 输入 | 是否使用 sub-agent | Agent | 原因 |
| --- | --- | --- | --- |
| `/bot-ping`、`/bot-status` 等确定性命令 | 否 | Go control plane | 低风险、离线可测 |
| 自我介绍/能力说明 | 否 | Go control plane | 本地确定性回答，避免把项目身份问题交给模型自由发挥 |
| 普通自然语言 | 是 | `planner-agent` | 需要意图细化和 tool-call plan |
| 高置信度数据接入规划，例如 `规划 ShanghaiTech 数据接入` | 是 | `data-intake-agent` | Go 先生成 `intake.plan`；未知参数再交给 Mimo planner |
| 已知模型下载/测试固定流程 | 是 | `model-agent` | Go 先生成固定工具链；未知模型仍交给 Mimo planner |
| 图片、截图、异常帧 | 是 | `vision-agent` | 需要 `mimo-v2.5` 视觉路由 |
| zip、manifest、目录索引、数据附件 | 是 | `data-intake-agent` | 需要 quarantine、scan、dry-run intake plan、pending approval workflow 和审批 |
| 训练、评估、部署长流程 | 是 | `training-agent` / future release agent | 需要任务生命周期、日志、artifact |
| 成功 trace 总结 skill | 是但默认关闭 | `skill-miner-agent` | 只能生成草稿，人工审批后启用 |

## 7. Mimo 和密钥边界

- Mimo 配置从 `C:\Users\10495\Desktop\mimo.txt` 读取，或整理为本机环境变量。
- 文本规划默认：`mimo-v2.5-pro`。
- 视觉理解默认：`mimo-v2.5`。
- `ops/scripts/load-mimo-env.ps1` 会自动设置 `AGENT_RUNTIME_PLANNER=python`、`AGENT_RUNTIME_USE_MIMO=true`、`AGENT_RUNTIME_PYTHONPATH=workers/python`，确保 `labelserver` 启动后 `labelctl agent` 通过 Python/Mimo planner 工作。
- Runtime status 暴露实际 planner 状态：`planner.mode`、`planner.effective_mode`、`planner.mimo_enabled`、`planner.token_present`。CLI `/status` 和 `/doctor` 必须显示这些字段，避免只看静态 provider route 误判为已接入。
- API Key 只能放在服务端环境变量或本机 secret 文件中，不能进入 Git、前端 bundle、runtime trace 或 channel payload。
- 测试脚本只能输出模型名、HTTP 状态和摘要，不能打印 token。

## 7.1 CLI 延迟策略

`labelctl agent` 的普通聊天和工具规划必须分离：

- 自我介绍、能力说明等项目身份问题走 Go 本地 fast-path，直接返回 runtime 身份，避免模型回答成“我是 Mimo”。
- 数据接入/入湖/manifest/本地文件夹/ShanghaiTech 这类高置信度工程意图走 Go 本地语义 fast-path，直接生成 `intake.plan` 并进入 ToolExecutor，保留审批、trace 和 workflow metadata。
- 普通聊天、概念解释走 `Mimo fast chat`，直接请求自然语言回复，不要求模型输出 JSON。
- 下载模型、安装依赖、数据接入、测试、训练、评估、部署、HuggingFace、ShanghaiTech、tool/skill/MCP 等复杂请求走 `Mimo planner`，输出受控 tool-call JSON，再由 Go ToolExecutor 执行。
- 已知 `LocateAnything-3B` 下载和 `LocateAnything-3B + ShanghaiTech` smoke 属于高置信度固定流程，默认 `AGENT_RUNTIME_LOCAL_SEMANTIC_FASTPATH=true`，由 Go 直接生成 `model.download_hf` 或 `model.verify_hf -> model.smoke_locateanything -> workflow.submit_run(dry_run=true)`；设置 `AGENT_RUNTIME_LOCAL_SEMANTIC_FASTPATH=false` 可强制回到 Mimo planner。
- `AGENT_RUNTIME_FAST_CHAT=false` 可关闭 fast chat，强制普通聊天也走 planner。
- `AGENT_RUNTIME_MIMO_CHAT_MAX_TOKENS` 控制普通聊天输出上限，默认 180；`AGENT_RUNTIME_MIMO_PLAN_MAX_TOKENS` 控制 planner 输出上限，默认 800。

Go `PythonPlanner` 默认使用常驻 `python -m agent_runtime.worker`，通过 stdin/stdout JSONL 发送请求，避免每轮 `exec python -m agent_runtime.main` 的冷启动。设置 `AGENT_RUNTIME_PYTHON_WORKER=false` 可回退到旧的单次 spawn 模式。

`labelctl agent` 在等待 runtime 返回时必须即时显示 `planner-agent working...` 和耗时，避免用户误判为 PowerShell 卡死。当前优化已减少 planner prompt / JSON repair / validation 和 Python 子进程冷启动成本。

普通 fast chat 已通过 `/api/runtime/stream-message` 接入 NDJSON 事件流：Go `SessionRunner.RunStream` -> `PythonPlanner.PlanStream` -> 常驻 `python -m agent_runtime.worker` -> Mimo Anthropic-compatible SSE。CLI 收到 `delta` 事件后立即写到终端；如果反向代理不支持 SSE，Python runtime 会退回一次性 Mimo 回复并以单个 `delta` 发出，CLI 仍不需要走第二套 UI。

Go `RuntimeRouter` 会在进入 PlannerPort 前选择 `local_control`、`local_semantic` 或 `external_planner`。Go 计算出的 `go_intent` 会随 metadata 传给 Python worker，Python 不再盲目重算入口意图，只在需要 Mimo 二级规划时继续细化参数。复杂任务仍走受控 planner/tool-call JSON。当前 stream 已覆盖 `status`、`tool_start`、`tool_progress` 和 `final`；`tool_progress` 来自 `internal/app/toolapp.Runner` 的 preflight、handler start/done、blocked/error 事件，再由 `GoToolExecutor` 映射为 runtime NDJSON。下一步需要把审批确认、model job 日志和会话恢复继续事件化，才能完全接近 `ccb` / Claude Code 的体感速度。

交互式 `labelctl agent` 也必须能在同一 shell 内观测长任务，避免用户离开 Agent CLI 再开第二套命令。`/job <id>`、`/job-logs <id>` 和 `/follow-job <id>` 只访问 Gateway 的 `/api/runtime/model-jobs/*` 与 `/logs/stream`，不直接读取 `data_lake`，不绕过 runtime store 和权限中间件。`/follow-job` 的终态事件现在会直接携带 retry、heartbeat、artifact、manifest 路径和 stdout/stderr 摘要，便于在单次跟随里完成排障。

错误契约采用兼容扩展：HTTP JSON 错误保留 `error` 字符串并新增 `error_envelope`；runtime NDJSON `error` 事件同样携带 `error_envelope`。CLI 优先显示 envelope 中的 `message`，而自动化测试可以使用 `code`、`source` 和 `retryable` 做稳定断言。

## 8. HuggingFace 模型下载边界

目标模型：`nvidia/LocateAnything-3B`。

模型下载只能写入：

```text
data_lake/models/artifacts/huggingface/nvidia/LocateAnything-3B
```

manifest 默认写入：

```text
data_lake/catalog/models/nvidia_LocateAnything-3B.download.json
```

`data_lake/` 当前被 Git ignore。模型权重、checkpoint、HF cache、真实 API Key 不得提交。

当前 downloader 能力：

- `--dry-run`：读取远端清单，记录 `remote_file_count` 和 `remote_total_bytes`，不下载权重。
- 默认下载：调用 `huggingface_hub.snapshot_download`，支持 resume。
- `--verify-only`：对比远端文件清单和本地文件大小，缺失或大小不一致时失败。
- `GET /api/runtime/model-jobs/{job_id}/logs`：读取已持久化的模型任务生命周期日志，并返回 `metadata.artifact_manifest` 以定位归档后的 artifact manifest。
- `GET /api/runtime/model-jobs/{job_id}/logs/stream`：以 NDJSON 输出已有日志和终态事件，为后续真实 worker 日志流保留兼容入口。
- Web Agent Overview 可点击 model job 并查询 `/logs`，与 CLI/API 共用同一 Gateway 边界。

## 9. 当前可验收证据

| 证据 | 命令 |
| --- | --- |
| Go 单元/集成测试 | `go test ./...` |
| Python runtime/worker 编译 | `python -m compileall workers\python` |
| Python model worker 契约 | `python -m unittest discover -s workers\python\agent_worker\tests` |
| Web 构建 | `npm run build` |
| 四入口 smoke | `powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\smoke-agent-entrypoints.ps1` |
| Runtime MVP smoke | `powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\smoke-runtime-mvp.ps1` |
| Mimo API smoke | `powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\smoke-mimo-api.ps1` |
| Mimo planner smoke | `powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\smoke-mimo-planner.ps1` |
| HF dry-run | `python skills\huggingface-model-downloader\scripts\download_hf_snapshot.py --repo-id nvidia/LocateAnything-3B --local-dir data_lake\models\artifacts\huggingface\nvidia\LocateAnything-3B --manifest data_lake\catalog\models\nvidia_LocateAnything-3B.download.json --dry-run` |
| Python worker ModelJob | `go test ./internal/app/agentruntime` |
| HF real download | `powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\runtime-hf-install.ps1 -StartDownload -WaitForCompletion` |
| HF verify-only | `python skills\huggingface-model-downloader\scripts\download_hf_snapshot.py --repo-id nvidia/LocateAnything-3B --local-dir data_lake\models\artifacts\huggingface\nvidia\LocateAnything-3B --manifest data_lake\catalog\models\nvidia_LocateAnything-3B.download.json --verify-only` |
| HF verify worker smoke | `powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\smoke-hf-verify-worker.ps1` |
| Training dry-run worker smoke | `powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\smoke-training-dry-worker.ps1` |
| LocateAnything load smoke | `powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\smoke-locateanything-model.ps1` |
| Gateway auth 单元测试 | `go test ./internal/infrastructure/middleware` |

## 10. 未完成项

- ShanghaiTech original 真实推理。
- model job 逐文件字节级进度、实时 worker stdout/stderr NDJSON 流和自动 resume；当前 `model.download_hf` 已默认通过 Go `ModelJob` 启动 Python worker，`model.verify_hf` 与 `model.smoke_locateanything` 也支持显式 worker job，并把 heartbeat/log/artifact/retry/stdout/stderr 摘要写入 store，同时把 artifact 摘要归档到 `runtime-root/artifacts/*.artifact_manifest.json`；timeout / decode failure 也会保留部分 stdout/stderr 和错误类型，但仍未提供逐文件流式输出和自动恢复执行。
- Tool runner 分发已迁移到 `internal/app/toolapp`；`intake.plan` / `vlm.inspect` 的 quarantine/scan/plan/workflow 构造已迁移到 `internal/app/intakeapp`，并通过 `internal/infrastructure/intakerepo.JSONRepository` 写入 `runtime-root/intake/intake_plans.json` 和 `intake_workflows.json`；`workflow.list_runs` / `workflow.submit_run` 的 dry-run 规则和 RunRequest 构造已迁移到 `internal/app/runtimeworkflow`；`model.download_hf` / `model.verify_hf` / `model.smoke_locateanything` 的参数规范化、路径安全、脚本执行和 smoke 解析已迁移到 `internal/app/modelruntime`。后续仍需把 model job 生命周期和 workflow run 接入正式 task repository。
- QQ 真实账号群聊 @Bot 实测。
- CLI / Gateway 的复杂 planner 分步流式、审批确认、model job 日志流和会话恢复；最小 tool progress streaming 已完成。
- Python worker 已有 heartbeat、logs、retries、artifacts 的最小 health/job 契约，且 `model.download_hf` 默认、`model.verify_hf` 显式 `job=true`、`model.smoke_locateanything` 显式 `job=true`、`training.run(dry_run)` 已接入 Go `ModelJob`；后续仍需把真实训练/评估统一接到 task repository、artifact manifest 和失败重试调度。
