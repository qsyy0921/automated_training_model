# Agent Runtime MVP ATDD

版本：v0.1  
日期：2026-06-03

## 1. ATDD 目标

本 ATDD 用业务可理解的场景定义 Agent Runtime MVP 是否达标。每个场景都必须有可执行证据，不能只靠代码阅读或口头说明。

## 2. 验收前置条件

- 仓库路径：`F:\automated_training_model`。
- PowerShell 会话启用 UTF-8：`. .\ops\scripts\utf8.ps1`。
- Go 可用：默认使用完整 MSI 安装版 `C:\Users\10495\AppData\Local\Programs\Go\bin\go.exe`；也可设置 `ATM_GO` 指向其他 `go.exe`。
- Web 依赖已安装。
- Mimo 配置存在：`C:\Users\10495\Desktop\mimo.txt`。
- 代理优先使用 `http://127.0.0.1:7890`。
- ShanghaiTech original 数据目录存在：`F:\automated_training_model\data_lake\raw\datasets\shanghaitech\original`。
- 真实 API Key 不出现在 Git、前端 bundle、trace、日志或文档中。

## 3. 验收场景

### ATDD-001 四入口进入同一个 Runtime

Given labelserver 已启动  
When Web、CLI、桌面端、QQ test-message 分别查询或发送消息  
Then 都能看到同一个 `automated-training-agent-runtime`，并在 `/api/runtime/sessions` 与 `/api/runtime/traces` 中留下记录。

证据：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\smoke-runtime-mvp.ps1
```

### ATDD-002 规则命令离线可测

Given 不启用 Mimo planner  
When 发送 `/bot-ping`、`/bot-status`、`/bot-runs`  
Then runtime 返回确定性结果，不调用模型。

证据：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\smoke-agent-entrypoints.ps1
```

### ATDD-003 普通文本进入 planner-agent

Given QQ test-message 发送普通自然语言请求  
When runtime 识别为 `chat`  
Then trace 的 `agent_id` 包含 `planner-agent`，状态为 planned 或 tool planned。

证据：`smoke-runtime-mvp.ps1` 中普通文本断言。

### ATDD-004 图片附件进入 vision-agent

Given QQ test-message 带 `image/png` 附件  
When runtime 识别为 `data_intake` 且附件是视觉类型  
Then sub-agent 为 `vision-agent`，tool trace 包含 `vlm.inspect`，metadata 包含 `model=mimo-v2.5`。

证据：`smoke-runtime-mvp.ps1` 中图片附件断言。

### ATDD-005 ShanghaiTech 数据附件生成 dry-run Data Intake Plan

Given QQ test-message 带 ShanghaiTech original 数据源附件  
When runtime 识别为数据接入请求  
Then sub-agent 为 `data-intake-agent`，tool trace 包含 `intake.plan`，metadata 包含：

- `dataset_name=shanghaitech-original`
- `source_uri=F:\automated_training_model\data_lake\raw\datasets\shanghaitech\original`
- `dry_run=true`
- `approval=human_review_before_data_lake_write`

And `runtime-root\intake\intake_plans.json` 包含 `dataset_name=shanghaitech-original` 的 dry-run plan。

证据：`smoke-runtime-mvp.ps1` 中数据附件断言和 intake repository 文件断言。

### ATDD-006 Mimo 文本规划可用

Given `C:\Users\10495\Desktop\mimo.txt` 可加载  
When 执行 Mimo planner smoke  
Then LocateAnything 安装请求输出 `model.download_hf`，ShanghaiTech 测试请求输出 `model.verify_hf` + `model.smoke_locateanything` + `workflow.submit_run(dry_run=true)`。

证据：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\smoke-mimo-planner.ps1
```

### ATDD-007 HuggingFace 下载 dry-run 不下载权重

Given 代理 7890 可用  
When 执行 HF downloader dry-run  
Then manifest 中记录远端文件数量和总字节，不创建或提交模型权重。

证据：

```powershell
$env:HTTP_PROXY="http://127.0.0.1:7890"
$env:HTTPS_PROXY="http://127.0.0.1:7890"
python skills\huggingface-model-downloader\scripts\download_hf_snapshot.py `
  --repo-id nvidia/LocateAnything-3B `
  --local-dir data_lake\models\artifacts\huggingface\nvidia\LocateAnything-3B `
  --manifest data_lake\catalog\models\nvidia_LocateAnything-3B.download.json `
  --dry-run
```

当前已知远端清单：

- `remote_file_count=38`
- `remote_total_bytes=7795875224`
- 最大文件包括两个 safetensors：约 4.96GB 和 2.70GB。

### ATDD-008 Runtime + Mimo 下载预检

Given Mimo planner 可用  
When 通过 runtime 发送“安装 nvidia/LocateAnything-3B”  
Then runtime 产生 `model.download_hf` trace。默认预检模式设置 `AGENT_RUNTIME_REQUIRE_MODEL_DOWNLOAD_APPROVAL=true`，因此状态应为 `approval_required`，不会下载权重。

证据：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\runtime-hf-install.ps1
```

### ATDD-009 真实下载和 verify-only

Given 磁盘空间、代理和时间充足  
When 显式执行真实下载脚本  
Then runtime 创建 `ModelJob`，默认 `execution_path=python-worker`，下载完成后 `verify-only` manifest 显示 `complete=true`；如需回退旧路径，可设置 `AGENT_RUNTIME_HF_DOWNLOAD_RUNNER=service`。

证据命令：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\runtime-hf-install.ps1 -StartDownload -WaitForCompletion
python skills\huggingface-model-downloader\scripts\download_hf_snapshot.py `
  --repo-id nvidia/LocateAnything-3B `
  --local-dir data_lake\models\artifacts\huggingface\nvidia\LocateAnything-3B `
  --manifest data_lake\catalog\models\nvidia_LocateAnything-3B.download.json `
  --verify-only
```

当前状态：已完成。通过 Agent Runtime + Mimo 触发 `model.download_hf`，job `model-job-1780371206860804000` 最终 `succeeded`；随后 `verify-only` 显示 `complete=true`、`missing_files=[]`、远端文件数 38、远端总字节 7,795,875,224。

### ATDD-010 LocateAnything 模型加载 smoke

Given `nvidia/LocateAnything-3B` 已下载并 verify-only 完成
When 通过 runtime 发送 “用 ShanghaiTech original 测试 LocateAnything-3B dry-run”
Then runtime trace 包含 `model.verify_hf`、`model.smoke_locateanything`、`workflow.submit_run`；smoke report 显示 `model_load=true`、`real_inference=false`，并记录 ShanghaiTech `training/testing/testframemask` split 存在。

证据：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\smoke-locateanything-model.ps1
```

当前状态：已完成模型加载 smoke。当前 Python 环境为 `torch 2.11.0+cpu`、无 CUDA；真实 ShanghaiTech 推理未完成。

### ATDD-011 Runtime session/trace 重启恢复

Given `smoke-runtime-mvp.ps1` 使用独立 `tmp/runtime-smoke-*` runtime store
When Web/CLI/桌面端/QQ test-message 都写入 session/trace 后重启 labelserver
Then `/api/runtime/sessions` 仍至少包含四类入口产生的 session，`/api/runtime/traces` 仍能看到 `intake.plan` trace。

证据：`smoke-runtime-mvp.ps1` 中重启恢复断言。

### ATDD-012 ModelJob 重启恢复

Given `JSONModelJobStore` 中存在 `succeeded`、`queued` 或 `running` 模型任务
When 服务重启并重新加载 `data_lake/runtime/model_jobs.json`
Then 已完成任务保持原状态，未完成任务标记为 `interrupted`，提示可重新提交下载任务利用 HuggingFace cache 继续。

证据：`internal/infrastructure/runtimerepo/json_model_jobs_test.go`。

### ATDD-013 Tool schema / approval preflight

Given planner 输出 tool-call
When tool-call 进入 `GoToolExecutor.Execute`
Then 未注册 tool、未知参数、高风险审批缺失会先被 `internal/app/toolapp` 拦截，合法 tool-call 才由 `toolapp.Runner` 分发到具体 handler。

证据：`internal/app/toolapp/schema_test.go`、`internal/app/toolapp/runner_test.go` 和 `internal/app/agentruntime/service_test.go`。

### ATDD-014 ModelJob 生命周期日志查询

Given `model.download_hf` 已创建 `ModelJob`
When Web Agent Overview、`labelctl runtime job-logs`、交互式 `labelctl agent /job-logs` 或 Gateway 查询 `/api/runtime/model-jobs/{job_id}/logs`
Then 返回该 job 的生命周期日志、状态和进度；`/logs/stream` 以 NDJSON 输出已有日志和终态事件；Web 中点击 model job 能看到日志列表；交互式 CLI 可用 `/follow-job <job_id>` 跟随日志流直到终态或超时。

证据：`internal/app/agentruntime/service_test.go`、`internal/api/httpapi/runtime_handlers_test.go`、`internal/cli/labelctl/domain_commands_test.go`、`internal/cli/labelctl/runtime_chat_test.go`、`npm run build --prefix web`。

### ATDD-015 Runtime tool progress streaming

Given `/api/runtime/stream-message` 接收一个会触发 tool-call 的请求
When `SessionRunner.RunStream` 调用 `GoToolExecutor.ExecuteStream`
Then 在 `final` 事件前至少输出 `tool_progress`，并包含 tool id、status、message 和同一 session key；CLI 可以实时显示 preflight 与 handler 进度。

证据：`internal/app/toolapp/runner_test.go`、`internal/app/agentruntime/session_test.go` 和 `internal/cli/labelctl/runtime_chat_test.go`。

### ATDD-016 Runtime / Gateway error envelope

Given planner、tool 或 Gateway API 发生错误
When runtime stream 或 HTTP JSON 返回错误
Then 响应保留兼容的 `error` 字符串，同时提供 `error_envelope.code`、`message`、`source`、`retryable`；CLI 优先显示 envelope message。

证据：`internal/app/agentruntime/errors_test.go`、`internal/app/agentruntime/session_test.go`、`internal/api/httpapi/runtime_handlers_test.go` 和 `internal/cli/labelctl/runtime_chat_test.go`。

### ATDD-017 Python worker 可观测执行契约

Given Go task runner 未来会用 JSON envelope 启动 Python model worker
When 执行 `python -m agent_worker.main --health` 或 dry-run job
Then worker 输出稳定 JSON，并包含 heartbeat、ordered logs、artifact 引用、attempt/max_attempts 和 retryable；缺少 task_id 的 job 应失败且标记 non-retryable。

证据：`workers/python/agent_worker/tests/test_worker_contracts.py`。

### ATDD-018 Go ModelJob 接入 Python worker

Given `model.download_hf` 收到 `dry_run=true` 或真实下载请求，或 `model.verify_hf` / `model.smoke_locateanything` 收到 `job=true`
When Go `GoToolExecutor` 创建 `ModelJob` 并启动 `python -m agent_worker.main`
Then job 最终应把 worker heartbeat、artifact、attempt/max_attempts、retryable、stdout/stderr 摘要和 worker logs 写回现有 ModelJobStore，CLI/Web/API 读取同一份数据；当设置 `AGENT_RUNTIME_HF_DOWNLOAD_RUNNER=service` 时，可显式回退旧下载 service runner，而 `model.verify_hf` / `model.smoke_locateanything` 默认仍保持同步模式，避免打断现有 smoke 链。

证据：`internal/app/agentruntime/service_test.go`、`internal/api/httpapi/runtime_handlers_test.go`、`internal/cli/labelctl/runtime_chat_test.go`。

### ATDD-019 Runtime 命令触发 verify worker job

Given Runtime 运行在 rule planner 模式且本地已有 HuggingFace 模型目录
When 发送 `/bot-verify-hf-job nvidia/LocateAnything-3B`
Then Go fast-path 应直接生成 `model.verify_hf` 且 `job=true` 的后台 worker 任务，不等待 Python/Mimo planner；trace 应包含 `model.verify_hf`，job logs 应包含 worker heartbeat 和 artifact 引用。

证据：`ops/scripts/smoke-hf-verify-worker.ps1`、`internal/app/agentruntime/service_test.go`。

### ATDD-020 Git 安全边界

Given 完成任意测试  
When 执行安全检查  
Then 不出现真实 token、模型权重、checkpoint、HF cache。

证据：

```powershell
rg -n "tp-[A-Za-z0-9]{20,}|sk-[A-Za-z0-9_-]{20,}|tp-c3" README.md docs internal workers web ops skills -S
git status --short --ignored data_lake\models data_lake\catalog tmp
```

## 4. 验收矩阵

| 场景 | 当前状态 | 证据 |
| --- | --- | --- |
| ATDD-001 | 已覆盖 | `smoke-runtime-mvp.ps1` |
| ATDD-002 | 已覆盖 | `smoke-agent-entrypoints.ps1` |
| ATDD-003 | 已覆盖 | `smoke-runtime-mvp.ps1` |
| ATDD-004 | 已覆盖 | `smoke-runtime-mvp.ps1` |
| ATDD-005 | 已覆盖 | `smoke-runtime-mvp.ps1` |
| ATDD-006 | 已覆盖 | `smoke-mimo-planner.ps1` |
| ATDD-007 | 已覆盖 dry-run | HF dry-run |
| ATDD-008 | 已覆盖预检 | `runtime-hf-install.ps1`，当前返回 `approval_required`，不下载权重 |
| ATDD-009 | 已覆盖 | `runtime-hf-install.ps1 -StartDownload -WaitForCompletion` + `download_hf_snapshot.py --verify-only` |
| ATDD-010 | 已覆盖 | `smoke-locateanything-model.ps1` |
| ATDD-011 | 已覆盖 | `smoke-runtime-mvp.ps1` |
| ATDD-012 | 已覆盖 | `json_model_jobs_test.go` |
| ATDD-013 | 已覆盖 | `schema_test.go` + `service_test.go` |
| ATDD-014 | 已覆盖 | `service_test.go` + `runtime_handlers_test.go` + `domain_commands_test.go` |
| ATDD-015 | 已覆盖 | `runner_test.go` + `session_test.go` + `runtime_chat_test.go` |
| ATDD-016 | 已覆盖 | `errors_test.go` + `session_test.go` + `runtime_handlers_test.go` + `runtime_chat_test.go` |
| ATDD-017 | 已覆盖 | `test_worker_contracts.py` |
| ATDD-018 | 已覆盖 | `service_test.go` + `runtime_handlers_test.go` + `runtime_chat_test.go` |
| ATDD-019 | 已覆盖 | `smoke-hf-verify-worker.ps1` + `service_test.go` |
| ATDD-020 | 每次提交前执行 | rg + git status |
