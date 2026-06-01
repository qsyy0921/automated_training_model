export interface AgentSpec {
  id: string;
  name: string;
  kind: string;
  version: string;
  description?: string;
  capabilities?: string[];
  tool_ids?: string[];
  runtime?: string;
  policy_tags?: string[];
  status: string;
}

export interface AgentToolSpec {
  id: string;
  name: string;
  kind: string;
  version: string;
  description?: string;
  permission_level?: string;
  permission_scopes?: string[];
  risk_level?: string;
  approval_required?: boolean;
  sandbox_policy_id?: string;
  budget_policy_id?: string;
  runtime?: string;
  status: string;
}

export interface WorkflowStep {
  id: string;
  name: string;
  agent_id?: string;
  tool_id?: string;
  action: string;
  depends_on?: string[];
  policy_tags?: string[];
  failure_policy_id?: string;
  approval_gate_id?: string;
  timeout_seconds?: number;
}

export interface WorkflowSpec {
  id: string;
  name: string;
  version: string;
  description?: string;
  trigger?: string;
  agent_ids?: string[];
  tool_ids?: string[];
  steps?: WorkflowStep[];
  status: string;
}

export interface AgentRun {
  id: string;
  task_id: string;
  workflow_id: string;
  dataset_id?: string;
  scene?: string;
  status: string;
  params?: Record<string, string>;
  created_at: string;
  updated_at: string;
}

export interface AuditEvent {
  id: string;
  actor: string;
  action: string;
  resource_type: string;
  resource_id: string;
  details?: Record<string, string>;
  created_at: string;
}

export interface EnforcementPoint {
  id: string;
  name: string;
  stage: string;
  description?: string;
  checks?: string[];
  blocks_on?: string[];
  audit_level?: string;
}

export interface DataGovernancePolicy {
  id: string;
  name: string;
  description?: string;
  required_checks?: string[];
  versioning_rule?: string;
  lineage_rule?: string;
  training_eligibility?: string;
}

export interface ReleasePolicy {
  id: string;
  name: string;
  description?: string;
  required_gates?: string[];
  rollout_stages?: string[];
  rollback_signals?: string[];
}

export interface RuntimePolicy {
  id: string;
  name: string;
  description?: string;
  scope?: string;
  required_fields?: string[];
  limits?: string[];
}

export interface ControlSurface {
  boundaries: Array<{ id: string; name: string; owns?: string[]; does_not_own?: string[]; integration_types?: string[] }>;
  enforcement_points: EnforcementPoint[];
  data_policies: DataGovernancePolicy[];
  release_policies: ReleasePolicy[];
  runtime_policies: RuntimePolicy[];
  version_registries: Array<{ id: string; name: string; governs: string; required_metadata?: string[]; promotion_gate?: string }>;
  schema_contracts: Array<{ id: string; name: string; scope: string; required_for?: string[]; failure_mode?: string }>;
  observability: Array<{ id: string; name: string; covers?: string[]; required_for?: string[]; alerts?: string[] }>;
  budget_policies: Array<{ id: string; name: string; scope: string; limits?: string[]; kill_signals?: string[] }>;
  failure_policies: Array<{ id: string; name: string; failure_classes?: string[]; retry_rule?: string; recovery_rule?: string; compensation_rule?: string }>;
  model_capabilities: Array<{ id: string; name: string; required_fields?: string[]; routing_rule?: string }>;
  tenant_isolation: Array<{ id: string; name: string; isolates?: string[]; required_for?: string[] }>;
  active_learning: Array<{ id: string; name: string; controls?: string[]; blocks_on?: string[] }>;
  recovery_policies: Array<{ id: string; name: string; protected_state?: string[]; recovery_checks?: string[] }>;
}

export interface RuntimeStatus {
  runtime: string;
  control_plane: string;
  agent_loop: string;
  policy: string;
  entry_points: Array<{
    id: string;
    name: string;
    transport: string;
    status: string;
    endpoint?: string;
    description?: string;
  }>;
  provider_routes: Array<{
    id: string;
    use_case: string;
    provider: string;
    model: string;
    secret_ref?: string;
    boundary: string;
  }>;
  sub_agents: Array<{
    id: string;
    name: string;
    runtime: string;
    model_route: string;
    capabilities?: string[];
    status: string;
  }>;
  skill_evolution: {
    enabled_by_default: boolean;
    current_mode: string;
    controls?: string[];
  };
}

export interface ChannelStatus {
  id: string;
  name: string;
  status: string;
  runtime: string;
  inbound_endpoint?: string;
  test_endpoint?: string;
}
