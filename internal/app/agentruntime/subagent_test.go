package agentruntime

import (
	"testing"

	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

func TestDecideSubAgentSkipsDeterministicCommands(t *testing.T) {
	msg := channel.InboundMessage{Text: "/bot-status"}
	intent := ClassifyIntent(msg)
	decision := DecideSubAgent(intent, msg)
	if decision.UseSubAgent {
		t.Fatalf("expected Go control plane handling, got %+v", decision)
	}
	if decision.ToolID != "runtime.status" {
		t.Fatalf("unexpected tool: %s", decision.ToolID)
	}
}

func TestDecideSubAgentUsesVisionForImages(t *testing.T) {
	msg := channel.InboundMessage{
		Text:        "看一下这张图",
		Attachments: []channel.Attachment{{ID: "att1", Name: "frame.png", MediaType: "image"}},
	}
	intent := ClassifyIntent(msg)
	decision := DecideSubAgent(intent, msg)
	if !decision.UseSubAgent {
		t.Fatalf("expected delegation")
	}
	if decision.AgentID != "vision-agent" {
		t.Fatalf("expected vision-agent, got %s", decision.AgentID)
	}
	if decision.ModelRoute != "vision" {
		t.Fatalf("expected vision route, got %s", decision.ModelRoute)
	}
}

func TestDecideSubAgentUsesPlannerForChat(t *testing.T) {
	msg := channel.InboundMessage{Text: "帮我创建一个训练 dry-run"}
	intent := ClassifyIntent(msg)
	decision := DecideSubAgent(intent, msg)
	if !decision.UseSubAgent || decision.AgentID != "planner-agent" {
		t.Fatalf("unexpected decision: %+v", decision)
	}
}
