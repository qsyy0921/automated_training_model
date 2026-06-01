# 代码架构

版本：v0.1  
日期：2026-06-02

这份文档把当前代码结构对齐到三张架构图。

## 1. CLI-first Agent 主入口

```text
cmd/labelctl/main.go
  -> internal/cli/labelctl
```

职责：

- 解析 CLI 命令。
- 调用 Go 控制面 API。
- 支持 `agent ask`、`agent run`、`agent auto`。
- 支持 LLM action planner。
- 默认面向 `data-to-deployment-lifecycle`。

`cmd/labelctl/main.go` 只保留进程入口，CLI Agent 逻辑放在 `internal/cli/labelctl`。

## 2. Go 控制面

```text
internal/api/httpapi
  REST API / DTO adapter

internal/app
  agentapp       Agent 控制面用例
  lifecycleapp   训练/评估/部署生命周期用例
  workflowapp    队列和模型网关端口
  datasetapp     数据集接入用例
  annotationapp  标注与审核用例
  mediaapp       视频、帧、tracking 查询用例

internal/domain
  agent          Agent / Tool / Workflow / Governance / Control Surface
  dataset        数据源和数据集
  annotation     对象级异常事件与审核
  workflow       任务状态
  modelregistry  模型注册
  deployment     部署请求
```

控制面负责稳定业务、状态、注册表、治理、审计和 API，不直接绑定 Python 或 PyTorch。

## 3. 生命周期 Agent 层

代码中的默认 Agent 和工作流在：

```text
internal/domain/agent/defaults.go
```

当前默认 Agent：

- data-collection-agent
- governance-agent
- vlm-label-agent
- training-agent
- evaluation-agent
- release-agent
- deployment-agent
- monitoring-agent
- report-agent

主工作流：

```text
data-to-deployment-lifecycle
```

## 4. 治理与安全控制面

```text
internal/domain/agent/governance.go
internal/domain/agent/control_surface.go
internal/app/agentapp/service.go
internal/api/httpapi/agent_handlers.go
```

当前提供：

- enforcement points
- data governance policies
- release policies
- runtime policies
- version registries
- schema contracts
- budget policies
- failure policies
- tenant isolation
- recovery policies

当前状态：contract 和 API 已存在，下一步要把它们接入工具/模型/worker 调用前的强制 preflight。

## 5. 执行与能力层

```text
workers/python/agent_worker
  Python worker JSON envelope

internal/infrastructure/modelgateway
  ModelGateway adapter

internal/infrastructure/queue
  当前 in-memory queue

web/src/shared/wasm
crates/tracking-math
  Rust/WASM 热点计算
```

当前 Python worker 只完成契约和 dry-run，真实训练/评估/标注执行还没有接入。

## 6. 数据与状态层

```text
data_lake/                 本地运行数据，不进 Git
internal/infrastructure/agentrepo
internal/infrastructure/modelrepo
internal/infrastructure/datasetrepo
internal/infrastructure/mergecsv
```

当前 MVP 使用 JSON repository 和本地文件系统。下一步生产化方向：

- PostgreSQL：注册表、任务状态、审批、审计。
- MinIO / S3：大对象、模型 artifact、数据集版本。
- Redis / NATS：队列、事件流、worker heartbeat。
- Lineage catalog：dataset -> labels -> training run -> checkpoint -> eval report -> deployment。

## 7. Web 控制台

```text
web/src/widgets/agent-control-panel
web/src/shared/api/client.ts
web/src/entities/agent/model.ts
```

Web 当前定位：

- 展示 Agent / Tool / Workflow registry。
- 提交 dry-run workflow。
- 展示治理强制路径。
- 展示 run 和 audit。

Web 不承担主 Agent 对话和自动化执行。

## 8. 当前架构风险

1. `workflowapp.ModelGateway` 当前接的是 noop gateway。
2. `queue.MemoryQueue` 不持久化，重启丢任务。
3. Python worker 非 dry-run 未执行真实任务。
4. 治理 contract 还没有接到强制 preflight。
5. Lineage catalog 和 artifact manifest 还没有成为硬约束。
