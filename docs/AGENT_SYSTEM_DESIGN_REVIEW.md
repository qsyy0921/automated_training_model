# Agent 系统设计审核

## 结论

当前方案适合作为第一阶段骨架：控制面、执行层、前端、数据湖已经分边界，后续替换队列、存储、Python worker 不需要推翻现有结构。

## 主要风险

- P1：当前队列仍是内存队列，服务重启后 run task 状态会丢失。下一阶段需要 Redis/NATS 或持久化任务表。
- P1：Python worker 还没有被 Go runner 真正调度，只完成了 job envelope 契约。下一阶段需要本地进程 runner 或 Docker runner。
- P1：数据湖 catalog 还只是轻量 JSON，没有强制 lineage 关系。训练闭环上线前需要记录 dataset -> derived labels -> train run -> checkpoint -> eval report。
- P2：权限策略当前是 metadata 字段，还没有 enforce。接入远程 Bot/Webhook 之前必须增加 policy check。
- P2：前端 Agent 控制台是 MVP，只展示和提交 dry-run，缺 workflow DAG、日志流和 artifact 链路。

## 通过点

- Go 后端使用 domain/app/infrastructure/api 分层，没有把 Python、文件系统、HTTP 细节放进 domain。
- Agent、Tool、Workflow 都以 registry 管理，符合 `Hermes` 和 `OpenClaw` 的可扩展方向。
- Workflow 提交只依赖 `ModelGateway` 端口，后续切队列或 worker backend 成本较低。
- 前端新增 panel 没有侵入视频播放器和标注逻辑，标注工作流仍保持原路径。
- 大文件仍在 `data_lake`，Git 只提交代码、文档、skill 和小型 manifest。

## 下一步优先级

1. 增加 durable queue + worker runner。
2. 给 Python worker 接入一个真实 dry-run command runner，并输出 artifact manifest。
3. 建立 `data_lake/catalog/lineage`，把训练消费和产出数据串起来。
4. 在前端增加 run log 和 artifact 链接。
5. 引入策略检查，至少覆盖模型下载、数据写入、远程入口三类权限。
