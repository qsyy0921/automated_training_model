# Agent Runtime MVP SDD

版本：v0.1  
日期：2026-06-02

## 1. 目标

本 SDD 定义 `automated_training_model` 当前阶段的 Agent Runtime MVP。目标不是把视频审核页继续做大，而是把 Web、CLI、桌面端、QQ/NapCat 四类入口统一接入同一个 Agent Runtime，并让 Agent 能围绕“小模型从数据到部署”的流程完成可审计的规划、工具调用和结果追踪。

## 2. 范围

MVP 必须覆盖：

- 四入口统一：Web、CLI、桌面端、QQ/NapCat 都进入同一个 runtime。
- Mimo 模型路由：文本规划走 `mimo-v2.5-pro`，视觉理解走 `mimo-v2.5`。
- 离线规则命令：`/bot-ping`、`/bot-me`、`/bot-status`、`/bot-runs`、`/bot-run dry` 不依赖模型即可测试。
- Sub-agent 决策：普通文本、视觉附件、数据附件分别进入不同 agent 角色。
- ToolExecutor：所有副作用都通过工具出口，不能让 channel 或 UI 直接写数据湖、下载模型或提交训练。
- Runtime trace：每次会话、意图、工具调用、错误、metadata 都可通过 API/CLI/Web 观察。
- HuggingFace 下载 skill：支持 dry-run、远端清单、断点续传、verify-only 和 Git 排除边界。
- ShanghaiTech original 数据 smoke：能识别数据源、生成 data intake plan trace，并明确真实推理阻塞点。

## 3. 非目标

当前 MVP 不承诺：

- 完整训练、评估、压缩、发布和线上监控闭环已经真实运行。
- QQ OneBot WebSocket 长连接 reader 已完成；当前先用 webhook/test-message。
- `ModelJobStore` 已具备生产级进度日志、取消和自动恢复；当前只有 JSON MVP 持久化，重启前未完成任务会恢复为 `interrupted`。
- LocateAnything-3B 已完成真实 ShanghaiTech 推理；当前只完成下载、verify-only 和模型加载 smoke。
- skill 自进化默认启用；当前只允许 draft-only，并需要人工审批。

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
| Session Runner | `session.go` | session key、planner 调用、tool 调用、trace 写入 | 不直接下载模型或写数据 |
| PlannerPort | `planner.go`、`python_planner.go` | 规则计划和 Python/Mimo 计划 | 不执行副作用 |
| Sub-agent Router | `subagent.go` | 决定是否委托 planner/vision/data-intake/training/skill-miner | 不绕过 approval |
| ToolExecutor | `tools.go` | 工具执行、preflight、model job、workflow dry-run | 后续要拆到 `toolapp`，避免继续膨胀 |
| Runtime Store | `store.go`、`model_jobs.go`、`internal/infrastructure/runtimerepo` | sessions、traces、model jobs | session/trace 和 model jobs 默认 JSON 持久化；后续迁移到 task repository |
| Python Runtime | `workers/python/agent_runtime` | Mimo planner、guard plan、VLM 路由 | 不保存密钥到仓库 |
| Skills | `skills/*` | 可复用操作说明和脚本 | 不提交权重或 token |

## 6. Sub-agent 使用规则

| 输入 | 是否使用 sub-agent | Agent | 原因 |
| --- | --- | --- | --- |
| `/bot-ping`、`/bot-status` 等确定性命令 | 否 | Go control plane | 低风险、离线可测 |
| 普通自然语言 | 是 | `planner-agent` | 需要意图细化和 tool-call plan |
| 图片、截图、异常帧 | 是 | `vision-agent` | 需要 `mimo-v2.5` 视觉路由 |
| zip、manifest、目录索引、数据附件 | 是 | `data-intake-agent` | 需要 quarantine、scan、dry-run intake plan 和审批 |
| 训练、评估、部署长流程 | 是 | `training-agent` / future release agent | 需要任务生命周期、日志、artifact |
| 成功 trace 总结 skill | 是但默认关闭 | `skill-miner-agent` | 只能生成草稿，人工审批后启用 |

## 7. Mimo 和密钥边界

- Mimo 配置从 `C:\Users\10495\Desktop\mimo.txt` 读取，或整理为本机环境变量。
- 文本规划默认：`mimo-v2.5-pro`。
- 视觉理解默认：`mimo-v2.5`。
- API Key 只能放在服务端环境变量或本机 secret 文件中，不能进入 Git、前端 bundle、runtime trace 或 channel payload。
- 测试脚本只能输出模型名、HTTP 状态和摘要，不能打印 token。

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

## 9. 当前可验收证据

| 证据 | 命令 |
| --- | --- |
| Go 单元/集成测试 | `go test ./...` |
| Python runtime 编译 | `python -m compileall workers\python\agent_runtime` |
| Web 构建 | `npm run build` |
| 四入口 smoke | `powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\smoke-agent-entrypoints.ps1` |
| Runtime MVP smoke | `powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\smoke-runtime-mvp.ps1` |
| Mimo API smoke | `powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\smoke-mimo-api.ps1` |
| Mimo planner smoke | `powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\smoke-mimo-planner.ps1` |
| HF dry-run | `python skills\huggingface-model-downloader\scripts\download_hf_snapshot.py --repo-id nvidia/LocateAnything-3B --local-dir data_lake\models\artifacts\huggingface\nvidia\LocateAnything-3B --manifest data_lake\catalog\models\nvidia_LocateAnything-3B.download.json --dry-run` |
| HF real download | `powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\runtime-hf-install.ps1 -StartDownload -WaitForCompletion` |
| HF verify-only | `python skills\huggingface-model-downloader\scripts\download_hf_snapshot.py --repo-id nvidia/LocateAnything-3B --local-dir data_lake\models\artifacts\huggingface\nvidia\LocateAnything-3B --manifest data_lake\catalog\models\nvidia_LocateAnything-3B.download.json --verify-only` |
| LocateAnything load smoke | `powershell -NoProfile -ExecutionPolicy Bypass -File .\ops\scripts\smoke-locateanything-model.ps1` |

## 10. 未完成项

- ShanghaiTech original 真实推理。
- model job 进度日志、取消和自动 resume。
- ToolExecutor 拆成 `internal/app/toolapp`。
- QQ OneBot WebSocket 长连接 reader。
- Python worker heartbeat、logs、retries、artifacts。
