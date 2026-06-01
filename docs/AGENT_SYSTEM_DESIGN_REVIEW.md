# Agent System Design Review

## Conclusion

The architecture has been refactored from a single platform overview into a
set of enforceable boundaries and contracts. The most important change is that
agent runtime concerns and training lifecycle concerns no longer share an
implicit control plane.

## Issues Addressed

- Runtime/training coupling: split into Agent Serving Platform and Model/Data
  Training Platform.
- Safety as sidecar: converted into forced enforcement points across ingress,
  tool calls, execution, model calls, egress, training ingest, and release.
- Training data risk: added curation policy with authorization, redaction,
  quality checks, split isolation, versioning, and lineage.
- Release risk: added promotion gate, staged rollout, rollback signals, and
  approval requirements.
- Skill self-modification risk: skill changes now belong to a version registry
  and promotion gate before release.
- Subagent risk: added delegation policy with reduced context, bounded depth,
  bounded fanout, and parent approval for writes.
- Memory risk: added memory lifecycle policy covering retention, deletion,
  vector cleanup, and separation from training data.
- Tool permission risk: tool specs now include scopes, risk level, approval,
  sandbox, and budget policy.
- Terminal risk: terminal execution policy requires scoped filesystem access,
  audited command metadata, timeout, and sandbox policy.
- Observability gap: added trace, cost, lineage, and incident response
  contracts in addition to audit.
- Cost risk: added runtime and training budget policies.
- Workflow failure ambiguity: added retry, hold, stop, recovery, and
  compensation policy contracts.
- Data lineage gap: lineage is now part of data governance and observability.
- Model abstraction risk: provider routing uses capability declarations and
  policy, not a provider name.
- Human review gap: approval gates are available across runtime, data,
  training, and release.
- Versioning gap: prompt, policy, workflow, tool, skill, and evaluation suite
  registries are explicit.
- Schema drift risk: request, workflow node, tool, dataset, and audit schemas
  are named contracts.
- Active learning bias: active learning policy includes sampling, label
  consistency, split isolation, drift, and contamination checks.
- Multi-tenant gap: tenant isolation now covers data, memory, vector indexes,
  secrets, audit, model access, and budget.
- Recovery gap: recovery policy names protected state and rebuild checks.

## Remaining Implementation Risk

The current implementation is still an MVP skeleton. The contract surfaces are
now in code and API, but production durability still requires a real durable
queue, persisted policy store, worker runner, secret backend, lineage catalog,
and deployment controller.

## Next Engineering Steps

1. Replace JSON repository and in-memory task queue with durable adapters.
2. Add policy enforcement middleware before tool/model/worker execution.
3. Add lineage manifests for every dataset, derived artifact, run, checkpoint,
   evaluation report, and release event.
4. Add a worker runner with sandbox, filesystem scope, timeout, and command
   audit.
5. Add frontend views for lineage, release gates, run logs, and policy holds.
