# Automated Training Model SDD

版本：v0.3  
日期：2026-06-02  
定位：从数据采集到模型部署的 CLI-first Agent 助手

## 1. 目标

Automated Training Model 的目标是把“小模型训练到部署”做成一个可审计、可恢复、可扩展的 Agent 闭环，而不是只提供一个视频标注页面。

核心链路：

```text
数据采集 -> 数据治理 -> 标注/复核 -> 数据集版本 -> 训练 -> 评估 -> 发布 -> 部署 -> 监控反馈
```

## 2. 架构原则

1. **CLI-first Agent 是主入口。**  
   CLI 负责对话、规划、工作流提交、治理查询和自动化执行。

2. **Go 是稳定控制面。**  
   Go 负责 Gateway、会话、注册表、工作流、状态、审计、治理、模型注册和部署控制。

3. **Web 是控制台和审核面。**  
   Web 负责可视化、人工复核、审批、运行状态和数据浏览，不重写主 Agent 对话面。

4. **Python 是模型执行层。**  
   Tracking、segmentation、VLM、训练、评估和报告生成放到 Python worker。

5. **Rust/WASM 只做性能热点。**  
   例如 IoU、轨迹片段计算、mask/RLE、大文件解析，不承担业务编排。

## 3. 分层架构

```text
入口层
  CLI / TUI
  Web 控制台
  API / Webhook
  Cron / CI / 脚本

会话与路由层
  Gateway
  Session Runner
  Task Router
  Command Guards
  Context Manager

Agent 核心层
  Planner
  Workflow Orchestrator
  Tool Executor
  Memory
  Subagent Delegation

生命周期 Agent 层
  Data Collection Agent
  Governance Agent
  Label / Review Agent
  Training Agent
  Evaluation Agent
  Release Agent
  Deployment Agent
  Monitoring Agent

执行与能力层
  Go control tools
  Python workers
  Rust/WASM acceleration
  Model gateway
  Deployment controller
  Monitoring collector

数据与状态层
  Data Lake
  Dataset Registry
  Model Registry
  Artifact Store
  Audit Store
  Lineage Catalog
  Queue Store
```

## 4. 核心域边界

### Agent Serving Platform

负责入口、会话、路由、规划、工具调用、运行时审批、沙箱、结果过滤和审计。

### Model/Data Training Platform

负责数据治理、数据集版本、训练运行、评估报告、模型产物、发布门禁、灰度发布和回滚元数据。

两个域只通过显式契约连接：

- workflow request
- dataset version
- model artifact version
- evaluation report
- promotion event
- audit event

## 5. 强制治理路径

每次请求和每个生命周期节点都必须经过治理检查：

```text
入口检查 -> 工具预检 -> 模型预检 -> 执行预检 -> 结果出站检查
```

治理覆盖：

- Auth / Tenant Scope
- Schema Registry
- Policy Registry
- Approval Queue
- Sandbox Policy
- Budget / Quota
- Audit / Trace
- Rollback / Recovery

当前代码已经有控制面 contract 和 API，下一阶段需要把这些 contract 变成真实 preflight enforcement。

## 6. 默认工作流

主工作流：`data-to-deployment-lifecycle`

```text
collect
  -> profile
  -> govern_data
  -> curate
  -> label_or_review
  -> train
  -> evaluate
  -> release
  -> deploy
  -> monitor
  -> report
```

辅助工作流：

- `agent-serving-request`
- `dataset-to-tracking`
- `human-loop-autolabel`

## 7. 当前实现状态

已完成：

- Go DDD / 六边形基础结构。
- Agent、Tool、Workflow、Run、Audit domain model。
- Agent control surface 和治理 API。
- CLI Agent 基础命令和 LLM action planner。
- Web Agent 控制台。
- Python worker JSON envelope。
- 模型注册 JSON repository。
- 三张 imagegen 架构图和 Mermaid 源图。

尚未完成：

- Durable queue。
- 真实 Python worker runner。
- Policy preflight enforcement。
- Artifact manifest。
- Lineage catalog。
- Run log stream。
- Production secret store。
- Deployment controller 的真实实现。

## 8. 当前保留的 SDD 文档

当前有用的 SDD/架构入口以 `docs/SDD_INDEX.md` 为准。旧的 v0.1 长文档已经删除，避免和当前 CLI-first Agent 架构冲突。
