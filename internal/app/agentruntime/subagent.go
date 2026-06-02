package agentruntime

import (
	"strings"

	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

type SubAgentSpec struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Runtime      string   `json:"runtime"`
	ModelRoute   string   `json:"model_route"`
	Capabilities []string `json:"capabilities"`
	Status       string   `json:"status"`
}

type DelegationDecision struct {
	UseSubAgent          bool     `json:"use_sub_agent"`
	AgentID              string   `json:"agent_id,omitempty"`
	Reason               string   `json:"reason"`
	RequiredCapabilities []string `json:"required_capabilities,omitempty"`
	SkillID              string   `json:"skill_id,omitempty"`
	ToolID               string   `json:"tool_id,omitempty"`
	MCPServer            string   `json:"mcp_server,omitempty"`
	ModelRoute           string   `json:"model_route,omitempty"`
}

func DefaultSubAgents() []SubAgentSpec {
	return []SubAgentSpec{
		{ID: "planner-agent", Name: "Planner Agent", Runtime: "python-agent-runtime", ModelRoute: "text-planning", Capabilities: []string{"intent-refine", "workflow-plan", "tool-plan"}, Status: "planned"},
		{ID: "vision-agent", Name: "Vision Agent", Runtime: "python-agent-runtime", ModelRoute: "vision", Capabilities: []string{"image-understanding", "visual-data-check"}, Status: "planned"},
		{ID: "data-intake-agent", Name: "Data Intake Agent", Runtime: "python-agent-runtime", ModelRoute: "text-planning", Capabilities: []string{"quarantine-plan", "manifest-plan", "data-governance"}, Status: "planned"},
		{ID: "model-agent", Name: "Model Agent", Runtime: "python-worker", ModelRoute: "text-planning", Capabilities: []string{"model-download", "model-verify", "smoke-test"}, Status: "planned"},
		{ID: "training-agent", Name: "Training Agent", Runtime: "python-worker", ModelRoute: "text-planning", Capabilities: []string{"training-plan", "evaluation-plan", "artifact-report"}, Status: "planned"},
		{ID: "skill-miner-agent", Name: "Skill Miner Agent", Runtime: "python-agent-runtime", ModelRoute: "text-planning", Capabilities: []string{"trace-summary", "skill-draft", "human-approval"}, Status: "disabled-by-default"},
	}
}

func DecideSubAgent(intent Intent, msg channel.InboundMessage) DelegationDecision {
	switch intent.Kind {
	case IntentHealthCheck, IntentIdentifyActor, IntentRuntimeStatus, IntentListRuns, IntentSubmitDryRun, IntentRuntimeAbout:
		return DelegationDecision{
			UseSubAgent: false,
			Reason:      "low-risk deterministic runtime command handled by Go control plane",
			SkillID:     intent.SkillID,
			ToolID:      intent.ToolID,
		}
	case IntentModelInstall, IntentModelTest:
		return DelegationDecision{
			UseSubAgent:          true,
			AgentID:              "model-agent",
			Reason:               "model lifecycle requests need controlled worker tools and observable jobs",
			RequiredCapabilities: []string{"model-download", "model-verify", "smoke-test"},
			SkillID:              intent.SkillID,
			ToolID:               intent.ToolID,
			ModelRoute:           "text-planning",
		}
	case IntentDataIntake:
		decision := DelegationDecision{
			UseSubAgent:          true,
			AgentID:              "data-intake-agent",
			Reason:               "channel attachments must be quarantined, scanned, planned, and approved before entering the data lake",
			RequiredCapabilities: []string{"quarantine-plan", "data-governance"},
			SkillID:              "channel-data-intake",
			ToolID:               "intake.plan",
			ModelRoute:           "text-planning",
		}
		if hasVisualAttachment(msg.Attachments) {
			decision.AgentID = "vision-agent"
			decision.Reason = "visual attachments require Mimo 2.5 vision inspection before data intake planning"
			decision.RequiredCapabilities = []string{"image-understanding", "visual-data-check", "quarantine-plan"}
			decision.ToolID = "vlm.inspect"
			decision.ModelRoute = "vision"
		}
		return decision
	case IntentChat:
		return DelegationDecision{
			UseSubAgent:          true,
			AgentID:              "planner-agent",
			Reason:               "free-form user text needs LLM planning before tool execution",
			RequiredCapabilities: []string{"intent-refine", "workflow-plan", "tool-plan"},
			SkillID:              intent.SkillID,
			ToolID:               intent.ToolID,
			ModelRoute:           "text-planning",
		}
	default:
		return DelegationDecision{
			UseSubAgent: false,
			Reason:      "unknown intent is not delegated until it is classified",
		}
	}
}

func hasVisualAttachment(attachments []channel.Attachment) bool {
	for _, attachment := range attachments {
		mediaType := strings.ToLower(strings.TrimSpace(attachment.MediaType))
		name := strings.ToLower(strings.TrimSpace(attachment.Name))
		if strings.HasPrefix(mediaType, "image") || strings.Contains(mediaType, "vision") {
			return true
		}
		for _, suffix := range []string{".jpg", ".jpeg", ".png", ".webp", ".bmp"} {
			if strings.HasSuffix(name, suffix) {
				return true
			}
		}
	}
	return false
}
