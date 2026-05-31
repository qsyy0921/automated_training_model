# Project TODO

版本：v0.1  
日期：2026-05-31

## 近期必须做

- [ ] 把真实 Python worker 接入 `workflowapp.ModelGateway`，支持 YOLO/BoT-SORT/SAM/YOLOWorld/VLM 自动标注任务。
- [ ] 将内存队列替换为可选 Redis Stream / NATS / RabbitMQ 适配，保留 in-memory 作为开发模式。
- [ ] 为数据集、标注版本、tracking 版本建立显式 version model。
- [ ] 增加任务日志、进度、artifact URI、失败原因的统一 task schema。
- [ ] 增加前端任务中心页面，展示自动标注、训练、评估、部署任务。
- [ ] 为对象级异常事件增加导出格式：中文标注 JSONL、英文映射 JSONL、训练用 object-event table。
- [ ] 为 tracking 删除增加恢复入口，目前已有 CSV 备份但缺少网页端恢复。
- [ ] 为 Manifest 模式设计大数据索引格式：video table、frame table、box table、track table、annotation table。
- [ ] 引入 Postgres / DuckDB / MinIO 的生产型存储适配。

## 前端架构待办

- [ ] 继续拆分 `features/anomaly-annotation`，把 event form、object slots、annotation list 分成子模块。
- [ ] 增加统一 toast/dialog 组件，替换散落的 `alert/confirm`。
- [ ] 增加键盘快捷键说明和冲突保护。
- [ ] 增加设计系统文档，固定二次元风主题、状态色、按钮、表单、卡片和轨迹颜色规范。
- [ ] 增加端到端 UI smoke test。
- [ ] 评估何时迁移到 Vite + TypeScript + React；当前先保持原生 ES Modules。

## 后端架构待办

- [ ] 为 `lifecycleapp` 增加真实 repository，保存任务状态而不是只依赖内存队列。
- [ ] 为 `providerapp` 增加加密 secret store。
- [ ] 补充 CLI：数据集注册、任务提交、导出标注、检查服务健康。
- [ ] 增加 OpenAPI 文档生成。
- [ ] 增加 authn/authz 边界，先支持单机 token，再扩展组织/项目权限。

## 研究任务待办

- [ ] 使用新 merge tracking 数据重建训练 targets。
- [ ] 对 object query 检测模型做完整可视化误差分析。
- [ ] 在检测 recall 稳定后再重新开启 Q_track / MOTR-lite。
- [ ] 设计 anomaly query 训练数据格式和评估协议。

