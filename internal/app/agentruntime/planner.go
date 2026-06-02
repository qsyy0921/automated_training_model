package agentruntime

import (
	"context"
	"fmt"
	"strings"
)

type RulePlanner struct{}

func NewRulePlanner() *RulePlanner {
	return &RulePlanner{}
}

func (p *RulePlanner) Plan(ctx context.Context, req PlanRequest) (PlanResult, error) {
	result := PlanResult{Intent: req.Intent, Delegation: req.Delegation, Status: "planned"}
	switch req.Intent.Kind {
	case IntentHealthCheck, IntentIdentifyActor, IntentRuntimeStatus, IntentListRuns, IntentSubmitDryRun, IntentDataIntake:
		return p.toolPlan(req), nil
	case IntentChat:
		result.ReplyText = fmt.Sprintf("已收到。意图会先交给 %s 做规划；当前最小运行时已支持 /bot-status、/bot-runs 和 /bot-run dry。", req.Delegation.AgentID)
		return result, nil
	default:
		if req.Intent.Command == "/bot-help" {
			result.ReplyText = strings.Join([]string{
				"可用命令：",
				"/bot-ping",
				"/bot-me",
				"/bot-status",
				"/bot-runs",
				"/bot-run dry [dataset_id]",
				"普通文本 -> planner-agent",
				"图片/附件 -> vision-agent 或 data-intake-agent",
			}, "\n")
			return result, nil
		}
		result.ReplyText = strings.Join([]string{
			"未知命令或暂不支持的意图。",
			"发送 /bot-help 查看可用命令。",
		}, "\n")
		return result, nil
	}
}

func (p *RulePlanner) toolPlan(req PlanRequest) PlanResult {
	toolID := req.Delegation.ToolID
	if toolID == "" {
		toolID = req.Intent.ToolID
	}
	return PlanResult{
		Intent:     req.Intent,
		Delegation: req.Delegation,
		Status:     "tool_planned",
		ToolCalls: []ToolCall{{
			ID:        "call-1",
			ToolID:    toolID,
			SkillID:   req.Intent.SkillID,
			MCPServer: req.Intent.MCPServer,
			Params:    toolParams(req),
		}},
	}
}

func toolParams(req PlanRequest) map[string]string {
	params := map[string]string{
		"source":      string(req.Message.Channel),
		"account_id":  req.Message.AccountID,
		"peer_kind":   string(req.Message.Peer.Kind),
		"peer_id":     req.Message.Peer.ID,
		"sender_id":   req.Message.SenderID,
		"session_key": req.Session.Key,
		"agent_id":    req.Session.AgentID,
	}
	if req.Intent.DatasetID != "" {
		params["dataset_id"] = req.Intent.DatasetID
	}
	return params
}
