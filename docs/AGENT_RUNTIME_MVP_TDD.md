# Agent Runtime MVP TDD

版本：v0.1  
日期：2026-06-03

## 1. TDD 目标

本 TDD 定义 Agent Runtime MVP 的测试分层、测试文件、测试命令和下一步补测计划。目标是让 runtime、channel、tool、skill、worker、UI 的边界通过测试固定下来，避免后期耦合成一个大 service。

## 2. 测试金字塔

```text
Unit Tests
  intent / sub-agent / tool preflight / path safety / trace metadata

Component Tests
  SessionRunner + RulePlanner + GoToolExecutor
  Model runtime app service
  Python runtime contract + Mimo guard plan
  HuggingFace downloader dry-run / verify-only

Integration / Smoke Tests
  Web + CLI + Desktop + QQ test-message
  Runtime model job endpoint
  ShanghaiTech data intake plan
```

## 3. Go 单元测试

| 模块 | 文件 | 当前覆盖 |
| --- | --- | --- |
| Intent | `internal/app/agentruntime/intent_test.go` | `/bot-*`、附件识别、普通文本、runtime self-description、LocateAnything 安装/测试高置信度意图 |
| Runtime Router | `internal/app/agentruntime/router_test.go` | CCB/Hermes 式混合路由：local control、local semantic、external planner；可用 env 关闭本地语义 fast-path |
| Sub-agent | `internal/app/agentruntime/subagent_test.go` | 确定性命令不委托、文本/视觉/数据附件委托、模型固定流程委托 `model-agent` |
| Planner selection | `internal/app/agentruntime/python_planner_test.go` | `AGENT_RUNTIME_USE_MIMO=true` 自动选择 PythonPlanner，`AGENT_RUNTIME_PLANNER=rule` 显式覆盖 |
| Python worker transport | `internal/app/agentruntime/python_planner_test.go` | 默认启用常驻 Python worker，`AGENT_RUNTIME_PYTHON_WORKER=false` 回退 spawn，并在 runtime status 暴露 transport |
| Local control fast-path | `internal/app/agentruntime/session_test.go` | `/bot-ping`、runtime self-description 等控制/身份问题即使配置外部 planner，也由 Go 本地规则规划，不调用 Python/Mimo |
| Local semantic fast-path | `internal/app/agentruntime/session_test.go` | 已知 LocateAnything 安装/ShanghaiTech 测试固定流程由 Go 直接生成受控工具链，可用 `AGENT_RUNTIME_LOCAL_SEMANTIC_FASTPATH=false` 关闭 |
| Data intake fast-path | `internal/app/agentruntime/service_test.go` | `规划 ShanghaiTech 数据接入` 直接进入 `data-intake-agent` + `intake.plan`，trace 记录 dataset/workflow metadata |
| Mandatory tool guard | `internal/app/agentruntime/session_test.go` | data-intake / vision 附件场景下，外部 Mimo planner 缺少必需 tool-call 或扩展额外工具链时回退本地计划并保留 sub-agent delegation |
| Session Runner | `internal/app/agentruntime/service_test.go` | workflow dry-run、附件 data intake trace、vision trace、model download policy |
| Model Jobs | `internal/app/agentruntime/service_test.go`、`internal/infrastructure/runtimerepo/json_model_jobs_test.go` | 异步下载排队、默认 Python worker 路径、`model.verify_hf job=true` / `model.smoke_locateanything job=true` / `training.run(dry_run)` worker 路径、service fallback、取消请求、`canceled/resumable` 状态、手动 resume child job、生命周期日志裁剪和终态判断，以及 worker timeout / decode failure 的 stdout/stderr + retryable/error-kind 落库；运行中 heartbeat / `stdout>` / `stderr>` 事件写回 store；artifact manifest 路径回填；`artifact-manifest/v1` 中的 `artifact_summary`（artifact_count / role_counts / kind_counts / execution_mode_counts / primary_artifact） |
| Tool schema/preflight | `internal/app/toolapp/schema_test.go` | 注册工具、参数白名单、高风险审批、未注册工具拦截 |
| Tool runner | `internal/app/toolapp/runner_test.go` | preflight 先于 handler、handler dispatch、结果合并、缺失 handler 拦截、handler error、ExecuteStream 输出 preflight/tool progress 事件 |
| Runtime stream | `internal/app/agentruntime/session_test.go`、`internal/app/agentruntime/errors_test.go`、`internal/cli/labelctl/runtime_chat_test.go` | `RunStream` 能把工具进度事件带上 session 输出到 NDJSON；planner/tool 失败输出 `error_envelope`；CLI 能解析 runtime stream event 和结构化错误消息 |
| Runtime workflow app | `internal/app/runtimeworkflow/service_test.go` | `workflow.submit_run` dry-run guard、RunRequest 构造、`workflow.list_runs` 回复格式 |
| Lifecycle task queue | `internal/infrastructure/queue/json_test.go`、`internal/app/lifecycleapp/service_test.go`、`internal/infrastructure/modelgateway/worker_test.go`、`internal/api/httpapi/lifecycle_handlers_test.go`、`npm run build --prefix web` | `tasks.json` 持久化恢复、重启后 `running/pending -> interrupted + resumable`、task id 连续性、training/evaluation/deployment 提交透传 gateway、worker-backed dry-run 与 `dry_run=false` repo-owned recipe / command execution 状态流转、heartbeat/logs/stdout/artifact 回写、artifact manifest 归档、task logs / NDJSON stream、`GET /api/tasks/{id}/manifest`、`POST /api/tasks/{id}/resume`、取消路径；artifact manifest 现为 `artifact-manifest/v1` 并带 `artifact_summary`；Web Agent Overview 构建通过并消费 `/api/tasks`、`/api/tasks/{id}/logs` 与 `/api/tasks/{id}/manifest` |
| Model runtime app | `internal/app/modelruntime/service_test.go` | HuggingFace 默认参数、目录逃逸拦截、LocateAnything smoke 默认路径、下载审批开关、smoke JSON 解析 |
| Runtime Store | `internal/infrastructure/runtimerepo/json_store_test.go`、`json_model_jobs_test.go`、`internal/infrastructure/intakerepo/json_repository_test.go` | session/trace JSON 持久化、model job 恢复和 interrupted/resumable 标记、intake plan/workflow JSON 恢复 |
| Intake workflow | `internal/app/intakeapp/workflow_test.go` | quarantine、静态 scan、pending approval、reject unsafe metadata、approve 后 register |
| Text-only intake | `internal/app/intakeapp/workflow_test.go` | 纯文本远程数据接入指令生成 synthetic text source attachment，仍走 scan 和 pending approval |
| Gateway middleware | `internal/infrastructure/middleware/middleware_test.go` | loopback 默认放行、非 loopback 无 token 拒绝、Bearer token 放行、强制 loopback token、health public |
| Runtime HTTP API | `internal/api/httpapi/runtime_handlers_test.go` | model job logs JSON、`/manifest` 和 NDJSON stream 入口；Gateway JSON error 保留 `error` 并返回 `error_envelope` |
| Channel domain | `internal/domain/channel/*_test.go` | approval policy |
| QQ adapter | `internal/infrastructure/qqbot/*_test.go` | OneBot normalize/outbound envelope；fake OneBot WebSocket reader 读取 message event 并回写 `send_msg` |
| CLI agent | `internal/cli/labelctl/runtime_chat_test.go`、`internal/cli/labelctl/domain_commands_test.go`、`labelctl agent` smoke | PowerShell BOM 输入归一化、trace metadata 渲染、交互式 `/status`、`/doctor`、`/ping`、`/job`、`/job-logs`、`/job-manifest`、`/follow-job`、`/tasks`、`/task`、`/task-logs`、`/task-manifest`、`/resume-task`、`/follow-task` 和自然语言消息进入同一 Runtime；`/follow-job` 与 `/follow-task` 终态事件都会显示 retry、heartbeat、artifact、manifest、stdout/stderr 摘要，运行中则分别复用对应 log stream 输出 worker heartbeat 与 `stdout>` / `stderr>` 行；dataset/models/autolabel/training/evaluation/deploy/logs/doctor 领域命令组路由到正确 Gateway API 并携带 token；`runtime/models/logs job-logs|job-manifest` 与 `runtime/logs follow-task|task-manifest` 路由到正确的 API，`runtime/training/evaluation/deploy/autolabel resume-task` 会命中 `POST /api/tasks/{id}/resume` |
| Skill drafts | `internal/app/skillapp/service_test.go`、`internal/cli/labelctl/skill_commands_test.go` | 草稿创建、列表、approve/reject 人工审批记录、secret-like 内容拦截；审批不自动启用 skill |

命令：

```powershell
. .\ops\scripts\resolve-go.ps1
$go = Resolve-Go
& $go test ./...
```

## 4. Python Runtime / Worker 测试

| 模块 | 当前测试 |
| --- | --- |
| Python 语法和 import | `python -m compileall workers\python` |
| Fast chat / Go intent metadata 分流 | ``$env:PYTHONPATH=(Resolve-Path .\workers\python).Path; python -m unittest discover -s workers\python\agent_runtime\tests`` |
| Model worker envelope / heartbeat / logs / artifacts / retry metadata | ``$env:PYTHONPATH=(Resolve-Path .\workers\python).Path; python -m unittest discover -s workers\python\agent_worker\tests`` |
| Go Python worker runner | `go test ./internal/app/modelruntime -run TestPythonModelWorkerRunner` |
| Mimo API | `ops/scripts/smoke-mimo-api.ps1` |
| Mimo planner / guard plan | `ops/scripts/smoke-mimo-planner.ps1` |

约束：

- 测试可以读取 `C:\Users\10495\Desktop\mimo.txt`。
- 测试不能打印 API Key。
- Mimo 不稳定时允许 guard plan，但必须输出受控 tool-call JSON。

## 5. HuggingFace Downloader 测试

当前脚本：

```text
skills/huggingface-model-downloader/scripts/download_hf_snapshot.py
```

必须覆盖：

- `--dry-run`：读取远端文件清单，不下载权重。
- 默认下载：写入 data_lake ignored 目录。
- `--verify-only`：对比远端文件清单和本地文件大小。
- 缺依赖时输出安装指令。
- token 只从 `HF_TOKEN` / `HUGGINGFACE_HUB_TOKEN` 读取。

建议后续补充 Python 单测：

```text
skills/huggingface-model-downloader/tests/test_download_hf_snapshot.py
```

待测点：

- `compare_remote_files` 能识别 missing。
- `compare_remote_files` 能识别 size_mismatch。
- dry-run manifest 包含 `remote_file_count` 和 `remote_total_bytes`。

## 6. Web 测试

当前最低验收：

```powershell
cd F:\automated_training_model\web
npm run build
```

当前 Web 已覆盖：

- Agent Overview 调用 runtime status。
- sessions/traces/model-jobs 查询。
- model job logs 查询和展示。
- trace metadata 摘要显示 `plan_id`、`dataset_name`、`source_uri`。

后续应补：

- Playwright 打开 `/`，断言 Agent Overview 首屏存在。
- 点击 QQ test-message，断言 trace 刷新。
- Runtime Traces 面板展示 plan metadata。

## 7. Smoke 测试

| 脚本 | 目的 |
| --- | --- |
| `smoke-agent-entrypoints.ps1` | 原有四入口、OneBot envelope、desktop、skill draft |
| `smoke-runtime-mvp.ps1` | Runtime MVP：sub-agent、model-jobs、ShanghaiTech data intake trace、intake plan/workflow JSON 写入、session/trace/intake workflow 重启恢复 |
| `smoke-mimo-api.ps1` | Mimo API 可用性 |
| `smoke-mimo-planner.ps1` | Mimo planner 输出受控 tool-call |
| `runtime-hf-install.ps1` | Runtime + Mimo 触发 HF 安装预检；显式 `-StartDownload -WaitForCompletion` 才真实下载并等待 job 完成 |
| `smoke-hf-verify-worker.ps1` | Runtime 发送 `/bot-verify-hf-job`，验证 Go fast-path 命令可直接排队 `model.verify_hf job=true` 的 Python worker job，并落回 trace / job logs / heartbeat / artifacts |
| `smoke-training-dry-worker.ps1` | Runtime 发送 `/bot-train-dry`，验证 `training.run(dry_run)` 已进入 Python worker `ModelJob`，并落回 trace / job logs / heartbeat / artifacts |
| `smoke-evaluation-dry-worker.ps1` | Runtime 发送 `/bot-eval-dry`，验证 `evaluation.run(dry_run)` 已进入 Python worker `ModelJob`，并落回 trace / job logs / heartbeat / artifacts |
| `smoke-deployment-dry-worker.ps1` | Runtime 发送 `/bot-deploy-dry`，验证 `deployment.run(dry_run)` 已进入 Python worker `ModelJob`，并落回 trace / job logs / heartbeat / artifacts |
| `smoke-runtime-execution-worker.ps1` | Runtime 发送 `/bot-train-run`、`/bot-eval-run`、`/bot-deploy-run`，验证 `dry_run=false` 的 `training.run` / `evaluation.run` / `deployment.run` 已进入 Python worker `ModelJob`，默认执行 `execution_recipe=default` 并落回 trace / job logs / heartbeat / `request/plan/result/recipe_spec/recipe_report` artifacts |
| `smoke-lifecycle-execution-worker.ps1` | 直接调用 `/api/training/runs`、`/api/evaluation/runs`、`/api/deployments` 提交带 `execution_recipe=default` 的 `dry_run=false` 请求，验证 lifecycle task 会真实执行 repo-owned recipe runner 并落地 `request/plan/result/recipe_spec/recipe_report`、heartbeat、task logs 和 artifact manifest |
| `smoke-lifecycle-cli-execution-worker.ps1` | 通过 `labelctl training/evaluation/deploy submit -exec-recipe default` 提交任务，验证 CLI 可把 repo-owned recipe 透传给 Python worker |
| `smoke-locateanything-model.ps1` | Runtime 触发 `model.verify_hf`、`model.smoke_locateanything`、`workflow.submit_run`，验证模型可加载但真实推理仍未完成 |

## 8. Red / Green / Refactor 规则

1. 先写失败测试或 smoke 断言。
2. 只改使测试通过所需的最小模块。
3. 通过后再整理边界，避免把逻辑塞回 `Service`。
4. 每次改动更新 SDD / ATDD / TDD 或 TODO / DONE。
5. 提交前运行安全检查。

## 9. 提交前测试清单

```powershell
. .\ops\scripts\resolve-go.ps1
$go = Resolve-Go
& $go test ./...
python -m compileall workers\python
$env:PYTHONPATH=(Resolve-Path .\workers\python).Path
$env:PYTHONPATH=(Resolve-Path .\workers\python).Path; python -m unittest discover -s workers\python\agent_worker\tests
$env:PYTHONPATH=(Resolve-Path .\workers\python).Path; python -m unittest discover -s workers\python\agent_runtime\tests
& $go test ./internal/app/modelruntime -run TestPythonModelWorkerRunner
& $go test ./internal/app/agentruntime -run TestModelDownloadDryRunQueuesPythonWorkerJob
cd F:\automated_training_model\web
npm run build
cd F:\automated_training_model
powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\smoke-agent-entrypoints.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\smoke-runtime-mvp.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\smoke-runtime-mvp.ps1 -UseMimoPlanner
powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\smoke-hf-verify-worker.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\smoke-training-dry-worker.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\smoke-evaluation-dry-worker.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\smoke-deployment-dry-worker.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\smoke-runtime-execution-worker.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\smoke-locateanything-model.ps1
rg -n "tp-[A-Za-z0-9]{20,}|sk-[A-Za-z0-9_-]{20,}|tp-c3" README.md docs internal workers web ops skills -S
git status --short --ignored data_lake\models data_lake\catalog tmp
```

## 10. 当前测试缺口

- ModelJob 逐文件字节级进度、原始 stdout/stderr 字节流直通和自动 resume 测试；当前已覆盖 `model.download_hf` 的默认 Python worker 调度、`model.verify_hf job=true`、`model.smoke_locateanything job=true` 与 `training.run(dry_run)` 的 worker 调度、worker stdout/stderr 摘要入库、运行中 heartbeat / `stdout>` / `stderr>` 写回、timeout / decode failure 的错误落库和生命周期日志查询，但不覆盖逐文件流式输出。
- `modelruntime` 接入统一 task/model worker 和 workflow repository 后的集成测试；当前 artifact manifest 归档只覆盖 JSON runtime store。
- QQ OneBot WebSocket reader 长连接测试。
- Mimo 启用后的 fast-path smoke：`/bot-ping`、`/bot-status`、`你好你是谁`、`规划 ShanghaiTech 数据接入`、已知 LocateAnything 安装请求应保持 Go 本地即时返回或排队，不等待 Python/Mimo planner。
- Gateway auth 集成 smoke：非 loopback 模拟、CLI `-token`、桌面端 `-token` 和前端 token profile。
- ShanghaiTech original 真实推理 smoke。
- Python worker 到统一 Go task repository 的真实调度集成测试；当前已覆盖 worker 自身 envelope、health、heartbeat、logs、artifact、retry metadata，以及 `model.download_hf` 的 Go `ModelJob` 调度链、`model.verify_hf job=true`、`model.smoke_locateanything job=true`、`training.run(dry_run)` 的 worker 调度和 service fallback。
- lifecycle HTTP task queue 的自动恢复和真实 GPU recipe 调度测试；当前已覆盖 `tasks.json` 持久化、worker-backed dry-run 状态流转、`dry_run=false` repo-owned recipe runner 与命令执行、heartbeat/logs/stdout/artifact 回写、artifact manifest、task logs / NDJSON stream、重启后 interrupted/resume 和取消，但不覆盖真实训练/评估/部署 side effect。
