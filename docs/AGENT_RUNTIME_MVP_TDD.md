# Agent Runtime MVP TDD

版本：v0.1  
日期：2026-06-02

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
| Sub-agent | `internal/app/agentruntime/subagent_test.go` | 确定性命令不委托、文本/视觉/数据附件委托、模型固定流程委托 `model-agent` |
| Planner selection | `internal/app/agentruntime/python_planner_test.go` | `AGENT_RUNTIME_USE_MIMO=true` 自动选择 PythonPlanner，`AGENT_RUNTIME_PLANNER=rule` 显式覆盖 |
| Python worker transport | `internal/app/agentruntime/python_planner_test.go` | 默认启用常驻 Python worker，`AGENT_RUNTIME_PYTHON_WORKER=false` 回退 spawn，并在 runtime status 暴露 transport |
| Local control fast-path | `internal/app/agentruntime/session_test.go` | `/bot-ping`、runtime self-description 等控制/身份问题即使配置外部 planner，也由 Go 本地规则规划，不调用 Python/Mimo |
| Local semantic fast-path | `internal/app/agentruntime/session_test.go` | 已知 LocateAnything 安装/ShanghaiTech 测试固定流程由 Go 直接生成受控工具链，可用 `AGENT_RUNTIME_LOCAL_SEMANTIC_FASTPATH=false` 关闭 |
| Mandatory tool guard | `internal/app/agentruntime/session_test.go` | data-intake / vision 附件场景下，外部 Mimo planner 缺少必需 tool-call 或扩展额外工具链时回退本地计划并保留 sub-agent delegation |
| Session Runner | `internal/app/agentruntime/service_test.go` | workflow dry-run、附件 data intake trace、vision trace、model download policy |
| Model Jobs | `internal/app/agentruntime/service_test.go` | 异步下载排队、取消请求、`canceled/resumable` 状态、手动 resume child job |
| Tool schema/preflight | `internal/app/toolapp/schema_test.go` | 注册工具、参数白名单、高风险审批、未注册工具拦截 |
| Tool runner | `internal/app/toolapp/runner_test.go` | preflight 先于 handler、handler dispatch、结果合并、缺失 handler 拦截、handler error |
| Runtime workflow app | `internal/app/runtimeworkflow/service_test.go` | `workflow.submit_run` dry-run guard、RunRequest 构造、`workflow.list_runs` 回复格式 |
| Model runtime app | `internal/app/modelruntime/service_test.go` | HuggingFace 默认参数、目录逃逸拦截、LocateAnything smoke 默认路径、下载审批开关、smoke JSON 解析 |
| Runtime Store | `internal/infrastructure/runtimerepo/json_store_test.go`、`json_model_jobs_test.go`、`internal/infrastructure/intakerepo/json_repository_test.go` | session/trace JSON 持久化、model job 恢复和 interrupted/resumable 标记、intake plan/workflow JSON 恢复 |
| Intake workflow | `internal/app/intakeapp/workflow_test.go` | quarantine、静态 scan、pending approval、reject unsafe metadata、approve 后 register |
| Gateway middleware | `internal/infrastructure/middleware/middleware_test.go` | loopback 默认放行、非 loopback 无 token 拒绝、Bearer token 放行、强制 loopback token、health public |
| Channel domain | `internal/domain/channel/*_test.go` | approval policy |
| QQ adapter | `internal/infrastructure/qqbot/*_test.go` | OneBot normalize/outbound envelope；fake OneBot WebSocket reader 读取 message event 并回写 `send_msg` |
| CLI agent | `internal/cli/labelctl/runtime_chat_test.go`、`internal/cli/labelctl/domain_commands_test.go`、`labelctl agent` smoke | PowerShell BOM 输入归一化、trace metadata 渲染、交互式 `/status`、`/doctor`、`/ping` 和自然语言消息进入同一 Runtime；dataset/models/deploy/logs/doctor 领域命令组路由到正确 Gateway API 并携带 token |

命令：

```powershell
. .\ops\scripts\resolve-go.ps1
$go = Resolve-Go
& $go test ./...
```

## 4. Python Runtime 测试

| 模块 | 当前测试 |
| --- | --- |
| Python 语法和 import | `python -m compileall workers\python\agent_runtime` |
| Fast chat / Go intent metadata 分流 | `python -m unittest discover -s workers\python\agent_runtime\tests` |
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
python -m compileall workers\python\agent_runtime
python -m unittest discover -s workers\python\agent_runtime\tests
cd F:\automated_training_model\web
npm run build
cd F:\automated_training_model
powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\smoke-agent-entrypoints.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\smoke-runtime-mvp.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\smoke-runtime-mvp.ps1 -UseMimoPlanner
powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\smoke-locateanything-model.ps1
rg -n "tp-[A-Za-z0-9]{20,}|sk-[A-Za-z0-9_-]{20,}|tp-c3" README.md docs internal workers web ops skills -S
git status --short --ignored data_lake\models data_lake\catalog tmp
```

## 10. 当前测试缺口

- ModelJob 逐文件字节级进度、实时日志流和自动 resume 测试。
- `modelruntime` 接入统一 task/model worker 和 workflow repository 后的集成测试。
- QQ OneBot WebSocket reader 长连接测试。
- Mimo 启用后的 fast-path smoke：`/bot-ping`、`/bot-status`、`你好你是谁`、已知 LocateAnything 安装请求应保持 Go 本地即时返回或排队，不等待 Python/Mimo planner。
- Gateway auth 集成 smoke：非 loopback 模拟、CLI `-token`、桌面端 `-token` 和前端 token profile。
- ShanghaiTech original 真实推理 smoke。
- Python worker heartbeat/log/retry/artifact 测试。
