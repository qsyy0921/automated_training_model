package toolapp

import (
	"fmt"
	"strings"
)

type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

type ToolCall struct {
	ID               string            `json:"id"`
	ToolID           string            `json:"tool_id"`
	SkillID          string            `json:"skill_id,omitempty"`
	MCPServer        string            `json:"mcp_server,omitempty"`
	Params           map[string]string `json:"params,omitempty"`
	RequiresApproval bool              `json:"requires_approval,omitempty"`
}

type ToolSpec struct {
	ID                 string
	Name               string
	Risk               RiskLevel
	AllowedParamKeys   []string
	RequiredParamKeys  []string
	RequiresApproval   bool
	ApprovalParamKey   string
	ApprovalParamValue string
	Description        string
}

type Catalog struct {
	byID map[string]ToolSpec
}

func NewCatalog(specs []ToolSpec) Catalog {
	byID := make(map[string]ToolSpec, len(specs))
	for _, spec := range specs {
		byID[spec.ID] = spec
	}
	return Catalog{byID: byID}
}

func DefaultCatalog() Catalog {
	return NewCatalog([]ToolSpec{
		{ID: "runtime.health", Name: "Runtime Health", Risk: RiskLow},
		{ID: "runtime.identify_actor", Name: "Identify Actor", Risk: RiskLow},
		{ID: "runtime.status", Name: "Runtime Status", Risk: RiskLow},
		{ID: "workflow.list_runs", Name: "List Workflow Runs", Risk: RiskLow},
		{
			ID:               "workflow.submit_run",
			Name:             "Submit Workflow Dry Run",
			Risk:             RiskMedium,
			AllowedParamKeys: []string{"workflow_id", "dataset_id", "dry_run", "model_repo_id", "data_root"},
		},
		{ID: "intake.quarantine", Name: "Plan Intake Quarantine", Risk: RiskLow},
		{ID: "intake.plan", Name: "Plan Data Intake", Risk: RiskLow, AllowedParamKeys: []string{"dataset_id", "data_root", "source_uri"}},
		{ID: "vlm.inspect", Name: "Plan VLM Inspection", Risk: RiskMedium, AllowedParamKeys: []string{"dataset_id", "model", "model_route", "source_uri", "image_id", "attachment_id", "attachment_name", "media_type", "local_uri"}},
		{ID: "llm.plan", Name: "LLM Plan", Risk: RiskLow},
		{
			ID:                 "model.download_hf",
			Name:               "Download HuggingFace Model",
			Risk:               RiskHigh,
			AllowedParamKeys:   []string{"repo_id", "local_dir", "manifest", "approved", "verify_only", "dry_run", "dataset_id"},
			ApprovalParamKey:   "approved",
			ApprovalParamValue: "true",
		},
		{
			ID:               "model.verify_hf",
			Name:             "Verify HuggingFace Model",
			Risk:             RiskMedium,
			AllowedParamKeys: []string{"repo_id", "local_dir", "manifest", "verify_only", "job", "dataset_id"},
		},
		{
			ID:               "model.smoke_locateanything",
			Name:             "LocateAnything Smoke",
			Risk:             RiskMedium,
			AllowedParamKeys: []string{"model_repo_id", "model_dir", "data_root", "output"},
		},
	})
}

func (c Catalog) Spec(id string) (ToolSpec, bool) {
	spec, ok := c.byID[id]
	return spec, ok
}

type PreflightPolicy struct {
	RequireExplicitApprovalForHighRisk bool
}

type PreflightResult struct {
	Allowed  bool
	Status   string
	Message  string
	Metadata map[string]string
}

func Preflight(catalog Catalog, policy PreflightPolicy, call ToolCall) PreflightResult {
	spec, ok := catalog.Spec(call.ToolID)
	if !ok {
		return PreflightResult{
			Allowed: false,
			Status:  "unsupported_tool",
			Message: fmt.Sprintf("工具 %s 尚未注册。", call.ToolID),
			Metadata: map[string]string{
				"tool_id": call.ToolID,
				"reason":  "not_registered",
			},
		}
	}
	if err := validateRequiredParams(spec, call.Params); err != nil {
		return PreflightResult{Allowed: false, Status: "preflight_failed", Message: err.Error(), Metadata: baseMetadata(spec, "missing_required_param")}
	}
	if err := validateAllowedParams(spec, call.Params); err != nil {
		return PreflightResult{Allowed: false, Status: "preflight_failed", Message: err.Error(), Metadata: baseMetadata(spec, "unknown_param")}
	}
	if needsApproval(spec, policy, call) {
		return PreflightResult{
			Allowed: false,
			Status:  "approval_required",
			Message: fmt.Sprintf("工具 %s 需要审批后执行。", call.ToolID),
			Metadata: map[string]string{
				"tool_id":  spec.ID,
				"risk":     string(spec.Risk),
				"approval": spec.ApprovalParamKey + "=" + spec.ApprovalParamValue,
			},
		}
	}
	return PreflightResult{Allowed: true, Status: "ok", Metadata: baseMetadata(spec, "allowed")}
}

func validateRequiredParams(spec ToolSpec, params map[string]string) error {
	for _, key := range spec.RequiredParamKeys {
		if strings.TrimSpace(params[key]) == "" {
			return fmt.Errorf("工具 %s 缺少必需参数 %s", spec.ID, key)
		}
	}
	return nil
}

func validateAllowedParams(spec ToolSpec, params map[string]string) error {
	if len(spec.AllowedParamKeys) == 0 || len(params) == 0 {
		return nil
	}
	allowed := map[string]bool{}
	for _, key := range commonParamKeys() {
		allowed[key] = true
	}
	for _, key := range spec.AllowedParamKeys {
		allowed[key] = true
	}
	for key := range params {
		if !allowed[key] {
			return fmt.Errorf("工具 %s 不接受参数 %s", spec.ID, key)
		}
	}
	return nil
}

func commonParamKeys() []string {
	return []string{
		"source",
		"account_id",
		"peer_kind",
		"peer_id",
		"sender_id",
		"session_key",
		"agent_id",
		"skill_id",
		"mcp_server",
		"model_route",
	}
}

func needsApproval(spec ToolSpec, policy PreflightPolicy, call ToolCall) bool {
	if call.RequiresApproval {
		return true
	}
	if !spec.RequiresApproval && !(policy.RequireExplicitApprovalForHighRisk && spec.Risk == RiskHigh) {
		return false
	}
	key := spec.ApprovalParamKey
	if key == "" {
		key = "approved"
	}
	want := spec.ApprovalParamValue
	if want == "" {
		want = "true"
	}
	return !strings.EqualFold(strings.TrimSpace(call.Params[key]), want)
}

func baseMetadata(spec ToolSpec, reason string) map[string]string {
	return map[string]string{
		"tool_id": spec.ID,
		"risk":    string(spec.Risk),
		"reason":  reason,
	}
}
