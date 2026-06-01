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
