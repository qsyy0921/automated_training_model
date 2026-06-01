# Automated Training Model Agent 系统设计

本设计把项目从“页面上的一个小功能”调整为 **从数据采集到模型部署的全流程 Agent 助手**。核心是 CLI-first Agent，Go 后端承接稳定控制面，Web 只是控制台、审核和运维界面。

系统拆成两个核心域：

- **Agent Serving Platform**：入口、会话、路由、规划、工具调用、运行时审批、沙箱、结果过滤和审计。
- **Model/Data Training Platform**：数据治理、数据集版本、训练运行、评估报告、模型产物、发布门禁、灰度发布和回滚元数据。

两者只通过显式契约连接：工作流请求、数据集版本、模型 artifact 版本、评估报告、发布事件和审计事件。不共享隐藏运行时状态。

## 当前架构图

架构图维护在 `docs/AGENT_ARCHITECTURE_DIAGRAMS.md`，包含 Mermaid 源图和 imagegen 视觉版：

- 总体分层架构
- CLI-first 运行时架构
- 数据到部署闭环
- 强制治理路径

## P0 Decisions Implemented

1. **Runtime and training boundaries are separate.**
   Agent serving owns session/runtime concerns. Training owns curation,
   dataset versions, evaluation, release, and rollback.

2. **Governance is a forced path.**
   The new control surface exposes ingress, tool pre-call, execution
   preflight, model pre-call, result egress, training ingest, and release
   promotion enforcement points.

3. **Runtime data cannot enter training directly.**
   `runtime-to-training-curation` requires consent, tenant isolation,
   redaction, deduplication, label quality, split isolation, version freezing,
   and lineage.

4. **Tools and workers carry risk metadata.**
   Tool specs now include permission scopes, risk level, approval requirement,
   sandbox policy, and budget policy.

5. **Release is gated.**
   Promotion requires offline evaluation, safety evaluation, regression tests,
   human approval, artifact lineage, staged rollout, and rollback readiness.

## P1 Decisions Implemented

1. **Version registries are explicit.**
   Prompt templates, policies, workflow definitions, tool contracts, skills,
   and evaluation suites have registry contracts.

2. **Schema contracts are explicit.**
   User requests, workflow node IO, tool parameters, dataset samples, and audit
   events have named schema contracts and failure modes.

3. **Observability is broader than audit.**
   Runtime traces, cost metrics, data lineage, and incident response are part of
   the control surface.

4. **Workflow failure semantics are named.**
   The default framework includes idempotent retry, hold-for-approval, and
   stop-without-compensation policies.

5. **Memory, terminal, and subagent policies are isolated.**
   Runtime policies define reduced context, bounded delegation, explicit shell
   execution scope, and memory lifecycle rules.

## P2 Decisions Implemented As Contracts

1. **Budget control** is represented by runtime and training budget policies.
2. **Model abstraction** uses capability declarations, not provider names.
3. **Active learning** is guarded by sampling, label consistency, split
   isolation, drift monitoring, and contamination checks.
4. **Tenant isolation** covers data lake paths, memory, vector indexes, secrets,
   audit logs, model access, and cost budgets.
5. **Recovery policy** protects queue state, workflow runs, audit logs, catalog
   metadata, model artifacts, and secret references.

These P2 items are not full distributed infrastructure yet. They are now
first-class contracts so the implementation can grow without changing the
domain model.

## Current Code Surface

- `internal/domain/agent`: agent, tool, workflow, run, audit, governance, and
  control-surface domain models.
- `internal/app/agentapp`: application service and repository ports.
- `internal/infrastructure/agentrepo`: JSON repository for local MVP state.
- `internal/api/httpapi`: REST API for registries, runs, audit, and governance.
- `workers/python/agent_worker`: stable JSON worker envelope.
- `cmd/labelctl`: CLI for API inspection, skill execution, and generic LLM
  agent planning.
- `web/src/widgets/agent-control-panel`: frontend operator view for agents,
  workflows, runs, audit, and governance.

## Governance API

```text
GET /api/governance/control-surface
GET /api/governance/enforcement-points
GET /api/governance/data-policies
GET /api/governance/release-policies
GET /api/governance/runtime-policies
```

CLI:

```powershell
go run .\cmd\labelctl governance all
go run .\cmd\labelctl governance enforcement
go run .\cmd\labelctl governance data
go run .\cmd\labelctl governance release
go run .\cmd\labelctl governance runtime
```

## Default Workflows

- `data-to-deployment-lifecycle`: primary CLI-driven lifecycle workflow from
  data collection through governance, curation, labeling/review, training,
  evaluation, release, deployment, monitoring, and feedback.
- `agent-serving-request`: runtime request guard, planning, tool preflight,
  sandboxed execution, result filtering, and audit.
- `dataset-to-tracking`: perception artifact generation for review.
- `human-loop-autolabel`: perception, label proposal, data curation, human
  review, training, evaluation, release gate, and report.

The workflow names remain product-facing, but tools and model adapters are
generic. Concrete model choices belong in local adapters, skills, or runtime
configuration, not in the control-plane architecture.
