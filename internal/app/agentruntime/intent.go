package agentruntime

import (
	"strings"

	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

type IntentKind string

const (
	IntentUnknown       IntentKind = "unknown"
	IntentHealthCheck   IntentKind = "health_check"
	IntentIdentifyActor IntentKind = "identify_actor"
	IntentRuntimeStatus IntentKind = "runtime_status"
	IntentListRuns      IntentKind = "list_runs"
	IntentSubmitDryRun  IntentKind = "submit_dry_run"
	IntentDataIntake    IntentKind = "data_intake"
	IntentChat          IntentKind = "chat"
)

type Intent struct {
	Kind       IntentKind        `json:"kind"`
	RawText    string            `json:"raw_text,omitempty"`
	Command    string            `json:"command,omitempty"`
	Args       []string          `json:"args,omitempty"`
	DatasetID  string            `json:"dataset_id,omitempty"`
	SkillID    string            `json:"skill_id,omitempty"`
	ToolID     string            `json:"tool_id,omitempty"`
	MCPServer  string            `json:"mcp_server,omitempty"`
	Confidence float64           `json:"confidence,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

func ClassifyIntent(msg channel.InboundMessage) Intent {
	text := strings.TrimSpace(msg.Text)
	if len(msg.Attachments) > 0 {
		return Intent{
			Kind:       IntentDataIntake,
			RawText:    text,
			SkillID:    "channel-data-intake",
			ToolID:     "intake.plan",
			Confidence: 1,
		}
	}
	if text == "" {
		return Intent{Kind: IntentUnknown, Confidence: 1}
	}
	if !strings.HasPrefix(text, "/") {
		return Intent{
			Kind:       IntentChat,
			RawText:    text,
			SkillID:    "agent-conversation",
			ToolID:     "llm.plan",
			Confidence: 0.7,
		}
	}
	fields := strings.Fields(text)
	command := strings.ToLower(fields[0])
	intent := Intent{RawText: text, Command: command, Args: fields[1:], Confidence: 1}
	switch command {
	case "/bot-ping":
		intent.Kind = IntentHealthCheck
		intent.ToolID = "runtime.health"
	case "/bot-me":
		intent.Kind = IntentIdentifyActor
		intent.ToolID = "runtime.identify_actor"
	case "/bot-status":
		intent.Kind = IntentRuntimeStatus
		intent.ToolID = "runtime.status"
	case "/bot-runs":
		intent.Kind = IntentListRuns
		intent.ToolID = "workflow.list_runs"
	case "/bot-run":
		if len(fields) >= 2 && strings.ToLower(fields[1]) == "dry" {
			intent.Kind = IntentSubmitDryRun
			intent.SkillID = "data-to-deployment-lifecycle"
			intent.ToolID = "workflow.submit_run"
			if len(fields) >= 3 {
				intent.DatasetID = fields[2]
			}
		}
	default:
		intent.Kind = IntentUnknown
	}
	return intent
}
