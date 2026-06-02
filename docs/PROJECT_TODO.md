# Project TODO

版本：v0.2  
日期：2026-06-02

## 近期必须做

- [x] 补齐 CLI 命令组：`dataset`、`models`、`deploy`、`logs`、`doctor`，作为现有 Gateway/API 的薄 facade，不绕过 runtime、lifecycle、task 和审计边界。
- [ ] 将 `labelctl agent` 从当前结构化 REPL 继续升级为更接近 Claude Code 的 TUI：复杂 planner 分步流式、实时工具调用进度、审批确认、会话恢复、快捷键和历史记录；当前 Go 控制命令 fast-path、本地语义 fast-path 和普通 fast chat streaming 已完成。
- [x] 增加 Gateway token auth、allowed origins 和 non-loopback 访问保护：loopback 默认开发放行，非 loopback `/api/` 必须配置并携带 token；CLI/桌面端支持 `-token`。
- [ ] 增加持久 remote profile、短期 token/RBAC、pairing code、CSRF/origin 管理操作细化。
- [x] 为 Web、CLI、桌面端、QQ Channel 增加远程连接 SDD 测试。
- [x] 将 `cmd/agentdesktop` 从只读 status scaffold 推进到最小桌面 runtime 面板，支持 status、sessions、traces、jobs、send 和 json。
- [x] 新增 `internal/domain/channel` 和 `internal/app/channelapp`，先固化 QQ Channel Adapter 边界。
- [x] 将 `internal/app/intakeapp` 从 JSON dry-run plan repository 推进到 intake workflow MVP：Channel 附件 quarantine、静态 scan、pending approval、approve/register metadata。
- [ ] 将 intake workflow MVP 推进到生产入湖：真实文件隔离区、压缩包展开/路径穿越扫描、manifest schema scan、审批队列、正式 dataset registry 写入和审计。
- [x] 将 Agent Runtime 的 LLM planner、fast chat、tool-call plan 迁移到 `workers/python/agent_runtime`，Go 只保留 Gateway/runtime shim 和受控 ToolExecutor。
- [x] 按 `REFERENCE_AGENT_RUNTIME_ALIGNMENT.md` 拆出 `SessionRunner`、`PlannerPort`、`ToolExecutorPort`，避免 `agentruntime.Service` 继续膨胀。
- [ ] 将 `skill-miner-agent` 从 draft-only 契约扩展为可人工审批的 skill 草稿生成器。
- [ ] 实现 QQ MVP：单 account、私聊文本、群聊 @Bot、`/bot-ping`、`/bot-me`、`/bot-status`、`/bot-runs`、`/bot-run dry`。
- [ ] 补真实账号群聊 @Bot 实测记录，验证 NapCat WebSocket reader 与 outbound 回发在本机登录 QQ 上可用。
- [x] 将 QQ MVP 从 HTTP webhook/test-message 扩展到可选长期 OneBot WebSocket reader：`QQ_ONEBOT_WS_ENABLED=true` 时 Gateway 主动连接 NapCat WebSocket，读取消息并在同一连接回写 `send_msg`。
- [x] 接入 NapCat outbound sender，让 `/api/channels/qq/onebot` 在环境变量开启后主动调用 OneBot `send_msg` 回发 QQ。
- [x] 接入 Mimo 本地交互式测试 provider：`mimo-v2.5-pro` 做规划，`mimo-v2.5` 做视觉数据检查，密钥只走环境变量或 SecretRef。
- [x] 默认启用常驻 `python -m agent_runtime.worker`，避免每轮 Mimo planner 都冷启动 Python；CLI 等待期间显示 `planner-agent working...` 耗时。
- [x] 为普通 fast chat 接入 `/api/runtime/stream-message` NDJSON token streaming：CLI 收到 Mimo `delta` 后立即刷屏，反向代理不支持 SSE 时退回单 delta。
- [x] 增加 Go 本地语义 fast-path：runtime self-description、已知 LocateAnything 下载和 ShanghaiTech smoke 固定流程不等待 Mimo；`AGENT_RUNTIME_LOCAL_SEMANTIC_FASTPATH=false` 可关闭并回到 Mimo planner。
- [x] 通过 Agent Runtime + Mimo planner 异步执行 `model.download_hf`，下载并校验 `nvidia/LocateAnything-3B`；如需安全模式再打开 `AGENT_RUNTIME_REQUIRE_MODEL_DOWNLOAD_APPROVAL=true`。
- [x] 将 Agent Runtime session/trace 从纯内存推进到 JSON MVP 持久化，默认写入 `data_lake/runtime`，smoke 覆盖重启恢复。
- [x] 将 `ModelJobStore` 从进程内内存推进到 JSON MVP 持久化，默认写入 `data_lake/runtime/model_jobs.json`，服务重启前未完成任务恢复为 `interrupted`。
- [x] 将 Data Intake Plan 从进程内内存推进到 JSON MVP 持久化，默认写入 `data_lake/runtime/intake/intake_plans.json`，smoke 覆盖 ShanghaiTech plan 写入和重启后保留。
- [x] 新增 `internal/app/toolapp`，固化 tool schema、参数白名单、risk level 和 approval/preflight gate。
- [x] 将 `GoToolExecutor` 的执行循环迁移到 `internal/app/toolapp.Runner`，由 runner 负责 preflight、handler dispatch、结果合并和未注册 handler 拦截。
- [x] 将 `model.download_hf` / `model.verify_hf` / `model.smoke_locateanything` 的参数规范化、路径安全、脚本执行和 smoke 解析外迁到 `internal/app/modelruntime`，`GoToolExecutor` 只保留注册、审批、异步 job 生命周期和结果适配。
- [ ] 将 `GoToolExecutor` 剩余具体工具 handler 继续迁移到独立 app/worker：`modelruntime` 后续接统一 task/model worker，`runtimeworkflow` 后续接正式 workflow/task repository，`vlm.inspect` 后续接入真实 VLM worker。
- [x] 将 `intake.plan` / `vlm.inspect` 的 dry-run Data Intake Plan 构造外迁到 `internal/app/intakeapp`，runtime 只负责 tool handler 调用和 trace metadata。
- [x] 为 JSON MVP model jobs 补齐阶段进度、生命周期日志、取消请求和手动 resume child job；CLI/Gateway 可查询详情、取消和恢复。
- [ ] 将 JSON MVP model jobs 迁移到统一 task repository，补齐逐文件字节级进度、实时日志流、取消幂等性和自动 resume 状态。
- [ ] 为 LocateAnything-3B 补齐 ShanghaiTech original 真实推理 smoke，并在结果中明确显存、依赖、权重格式的阻塞点。
- [ ] 新增 Web 默认首页 `Agent Overview`，把当前视频审核降级为 `Review Workbench` 页面。
- [ ] 拆出独立 `Task Center`、`Model Registry`、`Governance` 页面。
- [ ] 设计并实现 Go TUI 本地客户端，复用 `internal/cli/labelctl` 能力。
- [ ] 为 React 前端增加 Playwright UI smoke tests。
- [ ] 为 API 响应增加 Zod runtime schema，避免字段变更导致白屏。
- [ ] 把 `features/annotate-anomaly-event` 拆成事件表单、对象槽位、保存记录三个子模块。
- [ ] 增加统一 toast/dialog，替换 `alert/confirm`。
- [ ] 把真实 Python worker 接入 `workflowapp.ModelGateway`，支持 YOLO/BoT-SORT/SAM/YOLOWorld/VLM 自动标注任务。
- [ ] 将内存队列替换为可选 Redis Stream / NATS / RabbitMQ adapter，保留 in-memory 开发模式。
- [ ] 为数据集、标注版本、tracking 版本建立显式 version model。
- [ ] 增加任务日志、进度、artifact URI、失败原因的统一 task schema。
- [ ] 增加任务中心页面，展示自动标注、训练、评估、部署任务。
- [ ] 为对象级异常事件增加导出格式：中文标注 JSONL、英文映射 JSONL、训练用 object-event table。
- [ ] 为 tracking 删除增加网页端恢复入口，目前已有 CSV 备份但缺少恢复 UI。
- [ ] 为 Manifest 模式设计大数据索引格式：video table、frame table、box table、track table、annotation table。
- [ ] 引入 Postgres / DuckDB / MinIO 的生产型存储 adapter。

## 前端架构待办

- [ ] 固化前端 import boundary 检查，防止 shared/entities 反向依赖 widgets/pages。
- [ ] 增加 Storybook 或轻量组件预览页，沉淀设计系统。
- [ ] 增加键盘快捷键说明和冲突保护。
- [ ] 增加任务状态实时刷新和失败重试入口。
- [ ] 增加模型注册、部署中心、评估中心页面。

## 后端架构待办

- [ ] 为 `lifecycleapp` 增加真实 task repository，保存任务状态而不是只依赖内存队列。
- [ ] 为 `providerapp` 增加加密 secret store。
- [ ] 补充 CLI：数据集注册、任务提交、导出标注、检查服务健康。
- [ ] 增加 OpenAPI 文档生成。
- [ ] 增加 authn/authz 边界，先支持单机 token，再扩展组织/项目权限。

## 研究任务待办

- [ ] 使用新 merge tracking 数据重建训练 targets。
- [ ] 对 object query 检测模型做完整可视化误差分析。
- [ ] 在检测 recall 稳定后重新开启 Q_track / MOTR-lite。
- [ ] 设计 anomaly query 训练数据格式和评估协议。
- [ ] Replace the in-memory workflow queue with Redis/NATS and persist workflow state across server restarts.
- [ ] Wire Go workflow tasks to Python worker runners with heartbeat, logs, retries, and artifact manifests.
- [ ] Add data/model lineage catalogs for dataset -> labels -> training run -> checkpoint -> evaluation report.
