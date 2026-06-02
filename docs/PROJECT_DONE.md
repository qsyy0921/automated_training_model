# Project Done

版本：v0.2  
日期：2026-06-01

## 已完成

- [x] 将项目定位从单一视频标注工具扩展为“小模型训练到部署”的工程平台。
- [x] 建立 Go 主后端 DDD / 六边形基础目录。
- [x] 将项目迁移到独立根目录 `F:\automated_training_model`，避免继续受旧 ShanghaiTech 任务目录影响。
- [x] 配置 GitHub remote：`https://github.com/qsyy0921/automated_training_model.git`。
- [x] 实现 `labelserver` Go HTTP 服务。
- [x] 实现 merge/csv tracking 数据读取。
- [x] 实现帧图读取和 bbox overlay 数据 API。
- [x] 实现帧级异常 mask 自动切分为异常片段。
- [x] 实现 tracking 审核、删除预览、保存删除队列。
- [x] 实现彻底删除 tracking CSV 行并自动备份。
- [x] 实现对象级异常事件标注：视频 -> 异常片段 -> 异常事件 -> 多个相关对象。
- [x] 实现数据接入三种方式：本地目录、zip 上传、manifest 注册。
- [x] 实现数据集激活，并可切换当前 media/annotation repository。
- [x] 预留 Provider/API Key 查询边界。
- [x] 增加 `autolabel/training/evaluation/modelregistry/deployment` 领域模型。
- [x] 增加 `lifecycleapp`，统一自动标注、训练、评估、模型注册、部署任务入口。
- [x] 增加 lifecycle HTTP API：
  - `POST /api/autolabel/jobs`
  - `POST /api/training/runs`
  - `POST /api/evaluation/runs`
  - `POST /api/models/register`
  - `POST /api/deployments`
- [x] 增加独立模型仓库 `internal/infrastructure/modelrepo`，模型注册元数据默认持久化到 `data_lake/models/models.json`。
  - `GET /api/tasks/{id}`
  - `DELETE /api/tasks/{id}`
- [x] 修复项目级 PowerShell UTF-8 脚本，降低中文乱码概率。
- [x] 为项目 PowerShell 脚本统一接入 UTF-8 初始化，并增加 `ops/scripts/encoding-doctor.ps1` 检查中文文档读取。
- [x] 增加 sub-agent 决策契约、runtime status API、CLI/桌面/QQ 最小入口和测试 SDD。
- [x] 固化 sub-agent 使用时机：确定性低风险命令不委派；视觉附件走 `vision-agent`；zip/manifest/文件入湖走 `data-intake-agent`；自由文本走 `planner-agent`；高风险写操作不能绕过 approval gate。
- [x] 增加 QQ/NapCat OneBot outbound sender，可通过环境变量开启真实 `send_msg` 回发。
- [x] 拆分 Agent Runtime 为 `Service`、`SessionRunner`、`PlannerPort`、`ToolExecutorPort`、`RuntimeStore` 端口和内存开发实现，并新增 `/api/runtime/sessions`、`/api/runtime/traces` 可观测入口。
- [x] 删除中断留下的不完整 `LocateAnything-3B` 下载目录，确认模型权重残留不进入 Git。
- [x] 新增 Mimo Agent 安装提示词，明确 Codex 只维护 prompt/tool contract，模型下载必须由 Agent Runtime 调用 Mimo 规划后通过受控工具执行。
- [x] 前端从原生 ES Modules 迁移到 Vite + React + TypeScript。
- [x] 前端按 FSD / 前端 DDD 拆分：
  - `app`
  - `pages`
  - `widgets`
  - `features`
  - `entities`
  - `shared`
- [x] 前端接入 TanStack Query 管理服务端状态。
- [x] 前端接入 Zustand 管理 UI/draft 状态。
- [x] 增加平台型工作台视觉风格和集中设计 tokens。
- [x] Go 服务支持优先服务 `web/dist` 构建产物。

## 当前限制

- [ ] lifecycle 任务目前仍通过 in-memory/noop gateway 模拟排队，尚未真正调度 Python worker。
- [ ] Zod 只作为依赖接入，API runtime schema 尚未完整覆盖。
- [ ] 前端仍有少量 `alert/confirm`，后续需要统一 toast/dialog。
- [ ] 数据版本、标注版本目前只有边界设计；模型版本已有 JSON 元数据仓库，但还未接入真实训练 artifact 生命周期。
- [x] Added Agent Control Plane MVP: agent/tool/workflow registries, workflow run API, audit API, and default automated-training agents.
- [x] Added web Agent Control Panel for registry visibility and dry-run workflow submission.
- [x] Added Python worker contract skeleton under `workers/python/agent_worker`.
- [x] Added repo-local data lake ingest skill under `skills/automated-training-data-lake`.
- [x] 接入 Python Agent Runtime 的 Mimo planner wrapper：文本规划默认 `mimo-v2.5-pro`，视觉路由默认 `mimo-v2.5`，密钥只从本机环境变量 / `C:\Users\10495\Desktop\mimo.txt` 加载。
- [x] 新增 `ops/scripts/load-mimo-env.ps1` 和 `ops/scripts/smoke-mimo-api.ps1`，完成 `mimo-v2.5-pro` 与 `mimo-v2.5` API smoke；脚本不打印 API Key。
- [x] 新增 `ops/scripts/smoke-mimo-planner.ps1`，验证 LocateAnything 安装请求输出 `model.download_hf`，ShanghaiTech dry-run 请求输出 `model.verify_hf` + `model.smoke_locateanything` + `workflow.submit_run`。
- [x] 新增 repo-local HuggingFace 模型下载 skill：`skills/huggingface-model-downloader`，覆盖依赖、token、HF_HOME/cache、断点续传、manifest、校验和 Git 排除要求。
- [x] 完成 `nvidia/LocateAnything-3B` 下载 skill dry-run：确认默认本地目录、manifest 路径和不提交权重的边界。
- [x] 验证 Mimo planner 对 LocateAnything-3B 安装请求会输出 `model.download_hf` tool-call plan，而不是直接输出 shell 命令。
- [x] 调整 `model.download_hf` 权限策略：本机开发默认授予 Agent Runtime 执行权限；如需收紧，设置 `AGENT_RUNTIME_REQUIRE_MODEL_DOWNLOAD_APPROVAL=true` 后才要求 `approved=true`。
- [x] 验证 ShanghaiTech original 数据目录存在，并完成 `model.verify_hf` + `model.smoke_locateanything` + `workflow.submit_run(dry_run=true)` 的测试计划生成。
- [x] 前端 Agent Overview 接入 runtime status、sessions、traces 和入口测试面板；CLI 接入 runtime status/sessions/traces/send；桌面端复用 Gateway runtime snapshot。
- [x] 将前端 `wasm:build` 改为 `powershell -NoProfile`，避免 Conda PowerShell profile 的 GBK 乱码异常污染 `npm run build` 输出。
- [x] 分析并修正 Agent Runtime 长任务阻塞问题：`model.download_hf` 默认改为创建异步 `ModelJob`，入口立即返回 `queued/job_id`，新增 `/api/runtime/model-jobs`、`labelctl runtime model-jobs` 和 Web Model Jobs 面板。
- [x] 在 `AGENT_RUNTIME_SDD.md` 记录 2026-06-02 长任务问题结论、参考 OpenClaw/cc/Hermes 的对齐原则、新执行契约和 ART-007 至 ART-011 验收项。
- [x] 新增 `ops/scripts/smoke-runtime-mvp.ps1`：自动验证 Web/Gateway、CLI、桌面端、QQ test-message/OneBot 进入同一个 Agent Runtime，并检查 `planner-agent`、`vision-agent`、`data-intake-agent` trace。
- [x] 更新 README、`ENTRYPOINTS_TEST_SDD.md` 和 `SUB_AGENT_STRATEGY_SDD.md`，记录四入口闭环、`runtime model-jobs`、Mimo 本机配置和什么时候使用 sub-agent。
- [x] 将附件类 `data_intake` 从纯文本回复推进为 ToolExecutor 计划：数据附件生成 `intake.plan` trace metadata，视觉附件生成 `vlm.inspect` trace metadata；ShanghaiTech original 数据源 smoke 可在 runtime trace 中看到 `dataset_name` 和 `source_uri`。
- [x] 新增 Agent Runtime MVP 的 SDD / ATDD / TDD 文档，明确四入口、Sub-agent、Mimo、HuggingFace、ShanghaiTech、测试矩阵和未完成项。
- [x] 增强 HuggingFace downloader dry-run / verify-only：先读取远端文件清单，记录 `remote_file_count`、`remote_total_bytes`，verify-only 可识别缺失或大小不匹配文件。
- [x] 新增 `ops/scripts/runtime-hf-install.ps1`，用于通过 Agent Runtime + Mimo 触发 LocateAnything-3B 安装预检；默认审批模式返回 `approval_required`，不会下载权重。
- [x] 通过 Agent Runtime + Mimo planner 真实执行 `model.download_hf`，完成 `nvidia/LocateAnything-3B` 下载；本地路径为 `data_lake/models/artifacts/huggingface/nvidia/LocateAnything-3B`，权重仍在 ignored 的 `data_lake/` 下。
- [x] 完成 LocateAnything-3B `verify-only` 校验：远端文件数 38、远端总字节 7,795,875,224、`complete=true`、`missing_files=[]`。
- [x] 加固 Mimo planner contract：禁止未知 tool kind（例如 `tool.id`），Mimo 不稳定时对 chat/data_intake 回退到受控 guard plan，避免 unsupported tool 进入 Go ToolExecutor。
- [x] `smoke-runtime-mvp.ps1 -UseMimoPlanner` 已通过，覆盖 Mimo 模式下的 planner-agent、vision-agent、data-intake-agent 和 ShanghaiTech original source trace。
- [x] 修复 runtime smoke / HF install 脚本的进程清理：测试结束后按 `-addr` 清理 `go run` 派生的 `labelserver.exe`，避免端口残留。
- [x] 新增 `workers/python/agent_worker/locateanything_smoke.py` 和 `ops/scripts/smoke-locateanything-model.ps1`，通过 Runtime 触发 LocateAnything-3B 可用性 smoke。
- [x] 完成 LocateAnything-3B 模型加载 smoke：`AutoConfig`、`AutoProcessor`、safetensors shard 和 `AutoModel.from_pretrained` 均通过，参数量 3,517,975,280；当前 CPU-only，真实 ShanghaiTech 推理仍未完成。
- [x] 修复 agent repository bootstrap：旧 `data_lake/agents/workflows.json` 已有部分 workflow 时，也会补齐缺失的默认 workflow/tool/agent，不覆盖已有条目。
- [x] 新增 `internal/infrastructure/runtimerepo.JSONRuntimeStore`，将 Agent Runtime session/trace/meta 默认持久化到 `data_lake/runtime`。
- [x] 增强 `smoke-runtime-mvp.ps1`：使用独立 `tmp/runtime-smoke-*` store，发送四入口消息后重启 labelserver，并验证 `/api/runtime/sessions` 与 `/api/runtime/traces` 可恢复。
