# Project TODO

版本：v0.2  
日期：2026-06-01

## 近期必须做

- [ ] 补齐 CLI 命令组：`dataset`、`models`、`deploy`、`logs`、`doctor`。
- [ ] 增加 Gateway token auth、remote profile、allowed origins 和 non-loopback 访问保护。
- [x] 为 Web、CLI、桌面端、QQ Channel 增加远程连接 SDD 测试。
- [ ] 新增 `internal/domain/channel` 和 `internal/app/channelapp`，先固化 QQ Channel Adapter 边界。
- [ ] 将当前 runtime dry-run `intake.plan` 迁移到 `internal/app/intakeapp` 持久化实现，支持 Channel 附件 quarantine、scan、Data Intake Plan、approve/register workflow。
- [ ] 将 Agent Runtime 的 LLM planner、skill resolver、tool-call plan 迁移到 `workers/python/agent_runtime`，Go 只保留 Gateway/runtime shim。
- [x] 按 `REFERENCE_AGENT_RUNTIME_ALIGNMENT.md` 拆出 `SessionRunner`、`PlannerPort`、`ToolExecutorPort`，避免 `agentruntime.Service` 继续膨胀。
- [ ] 将 `skill-miner-agent` 从 draft-only 契约扩展为可人工审批的 skill 草稿生成器。
- [ ] 实现 QQ MVP：单 account、私聊文本、群聊 @Bot、`/bot-ping`、`/bot-me`、`/bot-status`、`/bot-runs`、`/bot-run dry`。
- [ ] 将 QQ MVP 从 HTTP webhook/test-message 扩展到长期 OneBot WebSocket reader，并补真实账号群聊 @Bot 实测记录。
- [x] 接入 NapCat outbound sender，让 `/api/channels/qq/onebot` 在环境变量开启后主动调用 OneBot `send_msg` 回发 QQ。
- [x] 接入 Mimo 本地交互式测试 provider：`mimo-v2.5-pro` 做规划，`mimo-v2.5` 做视觉数据检查，密钥只走环境变量或 SecretRef。
- [ ] 通过 Agent Runtime + Mimo planner 异步执行 `model.download_hf`，下载并校验 `nvidia/LocateAnything-3B`；如需安全模式再打开 `AGENT_RUNTIME_REQUIRE_MODEL_DOWNLOAD_APPROVAL=true`。
- [ ] 将 `ModelJobStore` 从进程内内存迁移到统一 task repository，保留下载进度、日志、取消和恢复状态。
- [ ] 为 LocateAnything-3B 补齐加载 smoke 和 ShanghaiTech original 真实推理 smoke，并在结果中明确显存、依赖、权重格式的阻塞点。
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
- [ ] Replace the in-memory workflow queue with Redis/NATS and persist agent run state across server restarts.
- [ ] Wire Go workflow tasks to Python worker runners with heartbeat, logs, retries, and artifact manifests.
- [ ] Add data/model lineage catalogs for dataset -> labels -> training run -> checkpoint -> evaluation report.
