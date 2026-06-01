package channel

import "strings"

func SessionKey(agentID string, peer Peer) string {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		agentID = "main"
	}
	return "agent:" + agentID + ":" + string(peer.Channel) + ":" + string(peer.Kind) + ":" + strings.TrimSpace(peer.ID)
}

func CanUseRestrictedTool(policy ToolPolicy) bool {
	return policy == ToolPolicyRestricted || policy == ToolPolicyFull
}

func CanUseDestructiveTool(policy ToolPolicy) bool {
	return policy == ToolPolicyFull
}

func RequiresApproval(plan DataIntakePlan) bool {
	if !plan.DryRun {
		return true
	}
	if strings.EqualFold(plan.RiskLevel, "high") {
		return true
	}
	return len(plan.RequiredApprovals) > 0
}
