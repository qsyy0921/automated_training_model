package agent

type EnforcementPoint struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Stage       string   `json:"stage"`
	Description string   `json:"description,omitempty"`
	Checks      []string `json:"checks,omitempty"`
	BlocksOn    []string `json:"blocks_on,omitempty"`
	AuditLevel  string   `json:"audit_level,omitempty"`
}

type DataGovernancePolicy struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Description         string   `json:"description,omitempty"`
	AllowedSources      []string `json:"allowed_sources,omitempty"`
	RequiredChecks      []string `json:"required_checks,omitempty"`
	VersioningRule      string   `json:"versioning_rule,omitempty"`
	LineageRule         string   `json:"lineage_rule,omitempty"`
	TrainingEligibility string   `json:"training_eligibility,omitempty"`
}

type ReleasePolicy struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Description     string   `json:"description,omitempty"`
	RequiredGates   []string `json:"required_gates,omitempty"`
	RolloutStages   []string `json:"rollout_stages,omitempty"`
	RollbackSignals []string `json:"rollback_signals,omitempty"`
}

type RuntimePolicy struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Description    string   `json:"description,omitempty"`
	Scope          string   `json:"scope,omitempty"`
	RequiredFields []string `json:"required_fields,omitempty"`
	Limits         []string `json:"limits,omitempty"`
}

func DefaultEnforcementPoints() []EnforcementPoint {
	return []EnforcementPoint{
		{
			ID:          "request-entry",
			Name:        "Request Entry Guard",
			Stage:       "ingress",
			Description: "Authenticates the caller, binds tenant scope, scans user input, and records the request.",
			Checks:      []string{"auth", "tenant-scope", "input-risk", "policy-match"},
			BlocksOn:    []string{"unauthorized", "unsafe-input", "unknown-scope"},
			AuditLevel:  "full",
		},
		{
			ID:          "tool-pre-call",
			Name:        "Tool Pre-call Guard",
			Stage:       "runtime",
			Description: "Validates tool schema, permission scopes, approval gates, data access, and budget.",
			Checks:      []string{"schema", "permission-scope", "approval", "data-scope", "budget"},
			BlocksOn:    []string{"schema-mismatch", "scope-denied", "approval-required", "budget-exceeded"},
			AuditLevel:  "full",
		},
		{
			ID:          "execution-preflight",
			Name:        "Execution Preflight",
			Stage:       "execution",
			Description: "Applies sandbox, filesystem, network, resource, and secret-access policies before code or worker execution.",
			Checks:      []string{"sandbox", "filesystem", "network", "resource-limit", "secret-scope"},
			BlocksOn:    []string{"unsafe-command", "forbidden-path", "network-denied", "secret-denied"},
			AuditLevel:  "full",
		},
		{
			ID:          "model-pre-call",
			Name:        "Model Pre-call Guard",
			Stage:       "model",
			Description: "Redacts sensitive context, validates model capability requirements, and enforces data retention policy.",
			Checks:      []string{"context-redaction", "capability-match", "retention-policy", "cost-limit"},
			BlocksOn:    []string{"sensitive-context", "capability-mismatch", "policy-denied"},
			AuditLevel:  "summary",
		},
		{
			ID:          "result-egress",
			Name:        "Result Egress Guard",
			Stage:       "egress",
			Description: "Filters outputs, validates citations or artifact links, and records result lineage.",
			Checks:      []string{"output-filter", "privacy-check", "result-schema", "lineage-write"},
			BlocksOn:    []string{"unsafe-output", "invalid-schema", "missing-lineage"},
			AuditLevel:  "full",
		},
		{
			ID:          "training-ingest",
			Name:        "Training Ingest Gate",
			Stage:       "training",
			Description: "Allows data into training only after authorization, redaction, quality, deduplication, and lineage checks.",
			Checks:      []string{"consent", "redaction", "quality", "dedupe", "split-isolation", "lineage"},
			BlocksOn:    []string{"no-consent", "quality-failed", "split-leakage", "lineage-missing"},
			AuditLevel:  "full",
		},
		{
			ID:          "release-promotion",
			Name:        "Release Promotion Gate",
			Stage:       "release",
			Description: "Promotes model, workflow, prompt, policy, or skill versions only after tests, approval, staged rollout, and rollback readiness.",
			Checks:      []string{"offline-eval", "safety-eval", "regression", "approval", "canary", "rollback-ready"},
			BlocksOn:    []string{"eval-regression", "approval-missing", "rollback-missing"},
			AuditLevel:  "full",
		},
	}
}

func DefaultDataGovernancePolicies() []DataGovernancePolicy {
	return []DataGovernancePolicy{
		{
			ID:                  "runtime-to-training-curation",
			Name:                "Runtime To Training Curation",
			Description:         "Runtime traces may become training data only through explicit curation and versioned lineage.",
			AllowedSources:      []string{"human-reviewed-labels", "approved-runtime-feedback", "approved-derived-artifacts"},
			RequiredChecks:      []string{"consent", "tenant-isolation", "redaction", "dedupe", "label-quality", "split-isolation"},
			VersioningRule:      "freeze every accepted dataset snapshot before training",
			LineageRule:         "record source artifact, label source, tool version, workflow version, and reviewer decision",
			TrainingEligibility: "manual or policy approval required",
		},
	}
}

func DefaultReleasePolicies() []ReleasePolicy {
	return []ReleasePolicy{
		{
			ID:          "model-and-skill-promotion",
			Name:        "Model And Skill Promotion",
			Description: "Shared promotion path for models, skills, prompts, policies, and workflows.",
			RequiredGates: []string{
				"offline-evaluation",
				"safety-evaluation",
				"regression-suite",
				"human-approval",
				"artifact-lineage",
				"rollback-plan",
			},
			RolloutStages:   []string{"dev", "staging", "canary", "production"},
			RollbackSignals: []string{"quality-regression", "error-rate-spike", "cost-spike", "policy-violation"},
		},
	}
}

func DefaultRuntimePolicies() []RuntimePolicy {
	return []RuntimePolicy{
		{
			ID:             "subagent-delegation",
			Name:           "Subagent Delegation",
			Description:    "Delegated tasks get a reduced context, explicit permission scope, bounded fanout, and parent verification.",
			Scope:          "subagents",
			RequiredFields: []string{"parent_run_id", "delegation_reason", "permission_scope", "context_manifest", "budget"},
			Limits:         []string{"max_depth=2", "max_fanout=4", "parent_approval_for_write"},
		},
		{
			ID:             "terminal-execution",
			Name:           "Terminal Execution",
			Description:    "Shell and process execution must run with audited commands, explicit working directory, bounded resources, and sandbox policy.",
			Scope:          "terminal",
			RequiredFields: []string{"working_directory", "command_hash", "sandbox_policy_id", "timeout_seconds", "filesystem_scope"},
			Limits:         []string{"no_implicit_secrets", "no_unscoped_network", "no_unapproved_delete"},
		},
		{
			ID:             "memory-lifecycle",
			Name:           "Memory Lifecycle",
			Description:    "Session memory, long-term memory, vector indexes, and training data remain isolated with deletion and retention rules.",
			Scope:          "memory",
			RequiredFields: []string{"tenant_id", "workspace_id", "source", "retention_class", "deletion_group"},
			Limits:         []string{"no_training_without_curation", "delete_embedding_with_source", "conflict_requires_review"},
		},
	}
}
