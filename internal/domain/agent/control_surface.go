package agent

type BoundaryContract struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Owns             []string `json:"owns,omitempty"`
	DoesNotOwn       []string `json:"does_not_own,omitempty"`
	IntegrationTypes []string `json:"integration_types,omitempty"`
}

type VersionRegistryContract struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Governs          string   `json:"governs"`
	RequiredMetadata []string `json:"required_metadata,omitempty"`
	PromotionGate    string   `json:"promotion_gate,omitempty"`
}

type SchemaContract struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Scope       string   `json:"scope"`
	RequiredFor []string `json:"required_for,omitempty"`
	FailureMode string   `json:"failure_mode,omitempty"`
}

type ObservabilityContract struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Covers      []string `json:"covers,omitempty"`
	RequiredFor []string `json:"required_for,omitempty"`
	Alerts      []string `json:"alerts,omitempty"`
}

type BudgetPolicy struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Scope       string   `json:"scope"`
	Limits      []string `json:"limits,omitempty"`
	KillSignals []string `json:"kill_signals,omitempty"`
}

type WorkflowFailurePolicy struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	FailureClasses   []string `json:"failure_classes,omitempty"`
	RetryRule        string   `json:"retry_rule,omitempty"`
	RecoveryRule     string   `json:"recovery_rule,omitempty"`
	CompensationRule string   `json:"compensation_rule,omitempty"`
}

type ModelCapabilityContract struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	RequiredFields []string `json:"required_fields,omitempty"`
	RoutingRule    string   `json:"routing_rule,omitempty"`
}

type TenantIsolationPolicy struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Isolates    []string `json:"isolates,omitempty"`
	RequiredFor []string `json:"required_for,omitempty"`
}

type ActiveLearningPolicy struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Controls []string `json:"controls,omitempty"`
	BlocksOn []string `json:"blocks_on,omitempty"`
}

type RecoveryPolicy struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	ProtectedState []string `json:"protected_state,omitempty"`
	RecoveryChecks []string `json:"recovery_checks,omitempty"`
}

type ControlSurface struct {
	Boundaries        []BoundaryContract        `json:"boundaries"`
	EnforcementPoints []EnforcementPoint        `json:"enforcement_points"`
	DataPolicies      []DataGovernancePolicy    `json:"data_policies"`
	ReleasePolicies   []ReleasePolicy           `json:"release_policies"`
	RuntimePolicies   []RuntimePolicy           `json:"runtime_policies"`
	VersionRegistries []VersionRegistryContract `json:"version_registries"`
	SchemaContracts   []SchemaContract          `json:"schema_contracts"`
	Observability     []ObservabilityContract   `json:"observability"`
	BudgetPolicies    []BudgetPolicy            `json:"budget_policies"`
	FailurePolicies   []WorkflowFailurePolicy   `json:"failure_policies"`
	ModelCapabilities []ModelCapabilityContract `json:"model_capabilities"`
	TenantIsolation   []TenantIsolationPolicy   `json:"tenant_isolation"`
	ActiveLearning    []ActiveLearningPolicy    `json:"active_learning"`
	RecoveryPolicies  []RecoveryPolicy          `json:"recovery_policies"`
}

func DefaultControlSurface() ControlSurface {
	return ControlSurface{
		Boundaries:        DefaultBoundaryContracts(),
		EnforcementPoints: DefaultEnforcementPoints(),
		DataPolicies:      DefaultDataGovernancePolicies(),
		ReleasePolicies:   DefaultReleasePolicies(),
		RuntimePolicies:   DefaultRuntimePolicies(),
		VersionRegistries: DefaultVersionRegistryContracts(),
		SchemaContracts:   DefaultSchemaContracts(),
		Observability:     DefaultObservabilityContracts(),
		BudgetPolicies:    DefaultBudgetPolicies(),
		FailurePolicies:   DefaultWorkflowFailurePolicies(),
		ModelCapabilities: DefaultModelCapabilityContracts(),
		TenantIsolation:   DefaultTenantIsolationPolicies(),
		ActiveLearning:    DefaultActiveLearningPolicies(),
		RecoveryPolicies:  DefaultRecoveryPolicies(),
	}
}

func DefaultBoundaryContracts() []BoundaryContract {
	return []BoundaryContract{
		{
			ID:               "agent-serving-platform",
			Name:             "Agent Serving Platform",
			Owns:             []string{"entry routing", "sessions", "tool orchestration", "runtime policy", "approval", "audit", "result egress"},
			DoesNotOwn:       []string{"training data eligibility", "model promotion", "dataset version freezing"},
			IntegrationTypes: []string{"workflow request", "versioned artifact reference", "audit event"},
		},
		{
			ID:               "model-data-training-platform",
			Name:             "Model/Data Training Platform",
			Owns:             []string{"data curation", "dataset versions", "training runs", "evaluation reports", "release promotion", "rollback metadata"},
			DoesNotOwn:       []string{"interactive session state", "runtime tool approval", "terminal execution policy"},
			IntegrationTypes: []string{"curated dataset version", "model artifact version", "evaluation report", "promotion event"},
		},
	}
}

func DefaultVersionRegistryContracts() []VersionRegistryContract {
	return []VersionRegistryContract{
		{ID: "prompt-registry", Name: "Prompt Template Registry", Governs: "prompt templates", RequiredMetadata: []string{"owner", "version", "input_schema", "eval_suite", "release_status"}, PromotionGate: "model-and-skill-promotion"},
		{ID: "policy-registry", Name: "Policy Registry", Governs: "runtime and data policies", RequiredMetadata: []string{"owner", "version", "scope", "test_cases", "rollback_version"}, PromotionGate: "model-and-skill-promotion"},
		{ID: "workflow-registry", Name: "Workflow Registry", Governs: "workflow definitions", RequiredMetadata: []string{"owner", "version", "step_schema", "failure_policy", "approval_gates"}, PromotionGate: "model-and-skill-promotion"},
		{ID: "tool-contract-registry", Name: "Tool Contract Registry", Governs: "tool schemas and permissions", RequiredMetadata: []string{"owner", "version", "input_schema", "output_schema", "permission_scopes", "sandbox_policy"}, PromotionGate: "model-and-skill-promotion"},
		{ID: "skill-registry", Name: "Skill Registry", Governs: "skills and reusable procedures", RequiredMetadata: []string{"owner", "version", "tests", "permissions", "rollback_version"}, PromotionGate: "model-and-skill-promotion"},
		{ID: "evaluation-registry", Name: "Evaluation Suite Registry", Governs: "offline and online evaluation cases", RequiredMetadata: []string{"owner", "version", "dataset_version", "metrics", "thresholds"}, PromotionGate: "model-and-skill-promotion"},
	}
}

func DefaultSchemaContracts() []SchemaContract {
	return []SchemaContract{
		{ID: "request-schema", Name: "User Request Schema", Scope: "entry", RequiredFor: []string{"gateway", "session runner", "planner"}, FailureMode: "reject or ask for clarification"},
		{ID: "workflow-node-schema", Name: "Workflow Node IO Schema", Scope: "workflow", RequiredFor: []string{"orchestrator", "worker", "replay"}, FailureMode: "hold run and emit schema error"},
		{ID: "tool-schema", Name: "Tool Parameter And Result Schema", Scope: "tools", RequiredFor: []string{"tool registry", "pre-call guard", "worker"}, FailureMode: "block tool call"},
		{ID: "dataset-sample-schema", Name: "Dataset Sample Schema", Scope: "training", RequiredFor: []string{"curation", "training", "evaluation"}, FailureMode: "quarantine sample"},
		{ID: "audit-event-schema", Name: "Audit Event Schema", Scope: "governance", RequiredFor: []string{"every enforced path"}, FailureMode: "block high-risk write"},
	}
}

func DefaultObservabilityContracts() []ObservabilityContract {
	return []ObservabilityContract{
		{ID: "runtime-trace", Name: "Runtime Trace", Covers: []string{"session", "agent", "tool", "model call", "approval"}, RequiredFor: []string{"debug", "audit", "replay"}, Alerts: []string{"tool failure spike", "approval bypass attempt"}},
		{ID: "cost-and-budget", Name: "Cost And Budget Metrics", Covers: []string{"model calls", "worker runtime", "subagents", "training jobs"}, RequiredFor: []string{"budget enforcement"}, Alerts: []string{"cost spike", "retry storm", "subagent fanout"}},
		{ID: "data-lineage", Name: "Data Lineage", Covers: []string{"source", "derived label", "dataset version", "model artifact", "evaluation report"}, RequiredFor: []string{"training", "release", "deletion"}, Alerts: []string{"missing lineage", "split leakage"}},
		{ID: "incident-response", Name: "Incident Response", Covers: []string{"policy violations", "security events", "runtime regressions", "release failures"}, RequiredFor: []string{"production operations"}, Alerts: []string{"policy violation", "rollback signal"}},
	}
}

func DefaultBudgetPolicies() []BudgetPolicy {
	return []BudgetPolicy{
		{ID: "runtime-budget", Name: "Runtime Budget", Scope: "user/session/workflow", Limits: []string{"max_tool_calls", "max_model_calls", "max_runtime_seconds", "max_subagent_depth"}, KillSignals: []string{"budget_exceeded", "retry_storm"}},
		{ID: "training-budget", Name: "Training Budget", Scope: "workspace/run", Limits: []string{"max_gpu_hours", "max_dataset_size", "max_retry_count"}, KillSignals: []string{"gpu_quota_exceeded", "training_timeout"}},
	}
}

func DefaultWorkflowFailurePolicies() []WorkflowFailurePolicy {
	return []WorkflowFailurePolicy{
		{ID: "retry-idempotent-then-hold", Name: "Retry Idempotent Then Hold", FailureClasses: []string{"transient", "timeout"}, RetryRule: "bounded exponential backoff for idempotent steps", RecoveryRule: "resume from last committed node", CompensationRule: "none for read-only or idempotent writes"},
		{ID: "hold-for-approval", Name: "Hold For Approval", FailureClasses: []string{"policy", "approval", "uncertain-output"}, RetryRule: "no automatic retry", RecoveryRule: "human or policy decision required", CompensationRule: "discard uncommitted output"},
		{ID: "stop-no-compensation", Name: "Stop Without Compensation", FailureClasses: []string{"data-governance", "release-gate", "lineage"}, RetryRule: "no automatic retry", RecoveryRule: "fix input contract and resubmit", CompensationRule: "block downstream promotion"},
	}
}

func DefaultModelCapabilityContracts() []ModelCapabilityContract {
	return []ModelCapabilityContract{
		{
			ID:   "generic-model-capabilities",
			Name: "Generic Model Capability Declaration",
			RequiredFields: []string{
				"supports_tools",
				"supports_vision",
				"supports_json_schema",
				"max_context_tokens",
				"streaming_support",
				"latency_class",
				"cost_class",
				"data_policy",
				"safety_profile",
			},
			RoutingRule: "route by declared capability and policy, not by provider name",
		},
	}
}

func DefaultTenantIsolationPolicies() []TenantIsolationPolicy {
	return []TenantIsolationPolicy{
		{
			ID:          "workspace-tenant-isolation",
			Name:        "Workspace And Tenant Isolation",
			Isolates:    []string{"data lake paths", "memory", "vector indexes", "secrets", "audit logs", "model access", "cost budgets"},
			RequiredFor: []string{"external entrypoints", "shared workers", "training data curation"},
		},
	}
}

func DefaultActiveLearningPolicies() []ActiveLearningPolicy {
	return []ActiveLearningPolicy{
		{
			ID:       "active-learning-quality-control",
			Name:     "Active Learning Quality Control",
			Controls: []string{"sampling balance", "label consistency", "reviewer quality", "train/validation/test split isolation", "drift monitoring", "evaluation contamination checks"},
			BlocksOn: []string{"unreviewed automatic labels", "split leakage", "duplicate heavy samples", "unknown consent"},
		},
	}
}

func DefaultRecoveryPolicies() []RecoveryPolicy {
	return []RecoveryPolicy{
		{
			ID:             "durable-control-plane-recovery",
			Name:           "Durable Control Plane Recovery",
			ProtectedState: []string{"task queue", "workflow runs", "audit log", "catalog metadata", "model artifacts", "secret references"},
			RecoveryChecks: []string{"restart can resume queued runs", "audit append remains intact", "vector index can rebuild from source", "artifact catalog has checksums"},
		},
	}
}
