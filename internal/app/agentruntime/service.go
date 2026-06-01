package agentruntime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/domain/agent"
	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

const defaultWorkflowID = "data-to-deployment-lifecycle"

type AgentControlPlane interface {
	SubmitWorkflowRun(ctx context.Context, req agent.RunRequest) (agent.WorkflowRun, error)
	ListRuns(ctx context.Context) ([]agent.WorkflowRun, error)
}

type Service struct {
	agents AgentControlPlane
	now    func() time.Time
}

func NewService(agents AgentControlPlane) *Service {
	return &Service{agents: agents, now: time.Now}
}

func (s *Service) HandleChannelMessage(ctx context.Context, msg channel.InboundMessage) (channel.OutboundMessage, error) {
	text := strings.TrimSpace(msg.Text)
	intent := ClassifyIntent(msg)
	delegation := DecideSubAgent(intent, msg)
	reply := channel.OutboundMessage{
		Channel:   msg.Channel,
		AccountID: msg.AccountID,
		Peer:      msg.Peer,
		ReplyToID: msg.ID,
	}
	if text == "" && len(msg.Attachments) == 0 {
		reply.Text = "已收到空消息。"
		return reply, nil
	}

	if strings.HasPrefix(text, "/") {
		out, err := s.handleCommand(ctx, msg, intent)
		if err != nil {
			return channel.OutboundMessage{}, err
		}
		out.Channel = msg.Channel
		out.AccountID = msg.AccountID
		out.Peer = msg.Peer
		out.ReplyToID = msg.ID
		return out, nil
	}

	if intent.Kind == IntentDataIntake {
		reply.Text = fmt.Sprintf("已收到 %d 个附件。下一步由 %s 进入隔离区并生成 Data Intake Plan；正式入湖前需要审批。", len(msg.Attachments), delegation.AgentID)
		return reply, nil
	}

	reply.Text = fmt.Sprintf("已收到。意图会先交给 %s 做规划；当前最小运行时已支持 /bot-status、/bot-runs 和 /bot-run dry。", delegation.AgentID)
	return reply, nil
}

func (s *Service) handleCommand(ctx context.Context, msg channel.InboundMessage, intent Intent) (channel.OutboundMessage, error) {
	switch intent.Kind {
	case IntentHealthCheck:
		return channel.OutboundMessage{Text: "pong"}, nil
	case IntentIdentifyActor:
		return channel.OutboundMessage{Text: fmt.Sprintf("channel=%s account=%s peer=%s:%s sender=%s", msg.Channel, msg.AccountID, msg.Peer.Kind, msg.Peer.ID, msg.SenderID)}, nil
	case IntentRuntimeStatus:
		return channel.OutboundMessage{Text: fmt.Sprintf("Agent Gateway online. channel=%s account=%s runtime=ready agent_loop=python-agent-runtime text_model=mimo-v2.5-pro vision_model=mimo-v2.5 time=%s", msg.Channel, msg.AccountID, s.now().Format(time.RFC3339))}, nil
	case IntentListRuns:
		runs, err := s.agents.ListRuns(ctx)
		if err != nil {
			return channel.OutboundMessage{}, err
		}
		if len(runs) == 0 {
			return channel.OutboundMessage{Text: "暂无 Agent run。"}, nil
		}
		limit := 5
		if len(runs) < limit {
			limit = len(runs)
		}
		lines := []string{"最近 Agent runs:"}
		for i := 0; i < limit; i++ {
			run := runs[i]
			lines = append(lines, fmt.Sprintf("- %s workflow=%s status=%s task=%s", run.ID, run.WorkflowID, run.Status, run.TaskID))
		}
		return channel.OutboundMessage{Text: strings.Join(lines, "\n")}, nil
	case IntentSubmitDryRun:
		datasetID := "workspace-dataset"
		if intent.DatasetID != "" {
			datasetID = intent.DatasetID
		}
		run, err := s.agents.SubmitWorkflowRun(ctx, agent.RunRequest{
			WorkflowID: defaultWorkflowID,
			DatasetID:  datasetID,
			DryRun:     true,
			Params: map[string]string{
				"source":     string(msg.Channel),
				"account_id": msg.AccountID,
				"peer_kind":  string(msg.Peer.Kind),
				"peer_id":    msg.Peer.ID,
				"sender_id":  msg.SenderID,
			},
		})
		if err != nil {
			return channel.OutboundMessage{}, err
		}
		return channel.OutboundMessage{Text: fmt.Sprintf("已提交 dry-run：run=%s task=%s workflow=%s dataset=%s", run.ID, run.TaskID, run.WorkflowID, run.DatasetID)}, nil
	default:
		if intent.Command == "/bot-help" {
			return channel.OutboundMessage{Text: strings.Join([]string{
				"可用命令：",
				"/bot-ping",
				"/bot-me",
				"/bot-status",
				"/bot-runs",
				"/bot-run dry [dataset_id]",
				"普通文本 -> planner-agent",
				"图片/附件 -> vision-agent 或 data-intake-agent",
			}, "\n")}, nil
		}
		return channel.OutboundMessage{Text: strings.Join([]string{
			"未知命令或暂不支持的意图。",
			"发送 /bot-help 查看可用命令。",
		}, "\n")}, nil
	}
}
