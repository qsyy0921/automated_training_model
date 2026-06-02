package runtimeworkflow

import (
	"context"
	"fmt"
	"strings"

	"github.com/qsyy0921/automated_training_model/internal/domain/agent"
	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

const DefaultWorkflowID = "data-to-deployment-lifecycle"

type ControlPlane interface {
	SubmitWorkflowRun(ctx context.Context, req agent.RunRequest) (agent.WorkflowRun, error)
	ListRuns(ctx context.Context) ([]agent.WorkflowRun, error)
}

type Service struct {
	agents ControlPlane
}

type SessionRef struct {
	Key     string
	AgentID string
}

type SubmitDryRunRequest struct {
	Message    channel.InboundMessage
	Session    SessionRef
	WorkflowID string
	DatasetID  string
	DryRun     bool
	Params     map[string]string
}

type ToolResult struct {
	ReplyText string
	Status    string
}

func NewService(agents ControlPlane) *Service {
	return &Service{agents: agents}
}

func (s *Service) ListRuns(ctx context.Context, limit int) (ToolResult, error) {
	runs, err := s.agents.ListRuns(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	if len(runs) == 0 {
		return ToolResult{ReplyText: "暂无 Agent run。", Status: "ok"}, nil
	}
	if limit <= 0 {
		limit = 5
	}
	if len(runs) < limit {
		limit = len(runs)
	}
	lines := []string{"最近 Agent runs:"}
	for i := 0; i < limit; i++ {
		run := runs[i]
		lines = append(lines, fmt.Sprintf("- %s workflow=%s status=%s task=%s", run.ID, run.WorkflowID, run.Status, run.TaskID))
	}
	return ToolResult{ReplyText: strings.Join(lines, "\n"), Status: "ok"}, nil
}

func (s *Service) SubmitDryRun(ctx context.Context, req SubmitDryRunRequest) (ToolResult, error) {
	if !req.DryRun {
		return ToolResult{}, fmt.Errorf("workflow.submit_run requires dry_run=true or explicit /bot-run dry intent")
	}
	workflowID := strings.TrimSpace(req.WorkflowID)
	if workflowID == "" {
		workflowID = DefaultWorkflowID
	}
	datasetID := strings.TrimSpace(req.DatasetID)
	if datasetID == "" {
		datasetID = "workspace-dataset"
	}
	run, err := s.agents.SubmitWorkflowRun(ctx, agent.RunRequest{
		WorkflowID: workflowID,
		DatasetID:  datasetID,
		DryRun:     true,
		Params:     mergeParams(channelParams(req.Message, req.Session), req.Params),
	})
	if err != nil {
		return ToolResult{}, err
	}
	return ToolResult{
		ReplyText: fmt.Sprintf("已提交 dry-run：run=%s task=%s workflow=%s dataset=%s", run.ID, run.TaskID, run.WorkflowID, run.DatasetID),
		Status:    "ok",
	}, nil
}

func channelParams(msg channel.InboundMessage, session SessionRef) map[string]string {
	return map[string]string{
		"source":      string(msg.Channel),
		"account_id":  msg.AccountID,
		"peer_kind":   string(msg.Peer.Kind),
		"peer_id":     msg.Peer.ID,
		"sender_id":   msg.SenderID,
		"session_key": session.Key,
		"agent_id":    session.AgentID,
	}
}

func mergeParams(base map[string]string, extra map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(extra))
	for key, value := range base {
		out[key] = value
	}
	for key, value := range extra {
		if strings.TrimSpace(value) != "" {
			out[key] = value
		}
	}
	return out
}
