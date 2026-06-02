package toolapp

import "testing"

func TestPreflightAllowsRegisteredToolWithCommonAuditParams(t *testing.T) {
	result := Preflight(DefaultCatalog(), PreflightPolicy{}, ToolCall{
		ToolID: "workflow.submit_run",
		Params: map[string]string{
			"workflow_id": defaultWorkflowForTest(),
			"source":      "qq",
			"session_key": "agent:planner-agent:qq:direct:10001",
		},
	})
	if !result.Allowed {
		t.Fatalf("expected workflow tool to pass preflight, got %+v", result)
	}
}

func TestPreflightRejectsUnknownParam(t *testing.T) {
	result := Preflight(DefaultCatalog(), PreflightPolicy{}, ToolCall{
		ToolID: "model.verify_hf",
		Params: map[string]string{"repo_id": "nvidia/LocateAnything-3B", "shell": "rm -rf ."},
	})
	if result.Allowed || result.Status != "preflight_failed" {
		t.Fatalf("expected unknown param rejection, got %+v", result)
	}
}

func TestPreflightAllowsVisionAttachmentContext(t *testing.T) {
	result := Preflight(DefaultCatalog(), PreflightPolicy{}, ToolCall{
		ToolID: "vlm.inspect",
		Params: map[string]string{
			"image_id":        "qq-img-001",
			"attachment_name": "frame.jpg",
			"model_route":     "vision",
			"source":          "qq",
			"session_key":     "agent:vision-agent:qq:group:10001",
		},
	})
	if !result.Allowed {
		t.Fatalf("expected vision attachment context to pass preflight, got %+v", result)
	}
}

func TestPreflightAllowsLocateAnythingPlannerContext(t *testing.T) {
	result := Preflight(DefaultCatalog(), PreflightPolicy{}, ToolCall{
		ToolID: "model.smoke_locateanything",
		Params: map[string]string{
			"model_repo_id": "nvidia/LocateAnything-3B",
			"model_dir":     "F:/automated_training_model/data_lake/models/huggingface/nvidia/LocateAnything-3B",
			"data_root":     "F:/automated_training_model/data_lake/raw/datasets/shanghaitech/original",
		},
	})
	if !result.Allowed {
		t.Fatalf("expected LocateAnything planner context to pass preflight, got %+v", result)
	}
}

func TestPreflightAllowsSkillRoutingContext(t *testing.T) {
	result := Preflight(DefaultCatalog(), PreflightPolicy{}, ToolCall{
		ToolID: "intake.plan",
		Params: map[string]string{
			"skill_id":    "automated-training-data-lake",
			"model_route": "text-planning",
			"source":      "qq",
		},
	})
	if !result.Allowed {
		t.Fatalf("expected skill routing context to pass preflight, got %+v", result)
	}
}

func TestPreflightCanRequireHighRiskApproval(t *testing.T) {
	result := Preflight(DefaultCatalog(), PreflightPolicy{RequireExplicitApprovalForHighRisk: true}, ToolCall{
		ToolID: "model.download_hf",
		Params: map[string]string{"repo_id": "nvidia/LocateAnything-3B"},
	})
	if result.Allowed || result.Status != "approval_required" {
		t.Fatalf("expected approval_required, got %+v", result)
	}
	approved := Preflight(DefaultCatalog(), PreflightPolicy{RequireExplicitApprovalForHighRisk: true}, ToolCall{
		ToolID: "model.download_hf",
		Params: map[string]string{"repo_id": "nvidia/LocateAnything-3B", "approved": "true"},
	})
	if !approved.Allowed {
		t.Fatalf("expected approved high-risk tool to pass, got %+v", approved)
	}
}

func TestPreflightRejectsUnregisteredTool(t *testing.T) {
	result := Preflight(DefaultCatalog(), PreflightPolicy{}, ToolCall{ToolID: "shell.exec"})
	if result.Allowed || result.Status != "unsupported_tool" {
		t.Fatalf("expected unsupported tool rejection, got %+v", result)
	}
}

func defaultWorkflowForTest() string {
	return "data-to-deployment-lifecycle"
}
