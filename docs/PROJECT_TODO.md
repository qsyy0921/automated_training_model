# Project TODO

版本：v0.2  
日期：2026-06-01

## 近期必须做

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

- [ ] 为 `lifecycleapp` 增加真实 repository，保存任务状态而不是只依赖内存队列。
- [ ] 为 `providerapp` 增加加密 secret store。
- [ ] 补充 CLI：数据集注册、任务提交、导出标注、检查服务健康。
- [ ] 增加 OpenAPI 文档生成。
- [ ] 增加 authn/authz 边界，先支持单机 token，再扩展组织/项目权限。

## 研究任务待办

- [ ] 使用新 merge tracking 数据重建训练 targets。
- [ ] 对 object query 检测模型做完整可视化误差分析。
- [ ] 在检测 recall 稳定后重新开启 Q_track / MOTR-lite。
- [ ] 设计 anomaly query 训练数据格式和评估协议。
