package agentruntime

import (
	"testing"

	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

func TestRuntimeRouterSelectsLocalSemanticForKnownDataIntake(t *testing.T) {
	msg := channel.InboundMessage{
		ID:      "msg1",
		Channel: channel.KindQQ,
		Text:    "请帮我规划 ShanghaiTech 数据接入",
	}
	intent := ClassifyIntent(msg)
	route := NewRuntimeRouter().Select(PlanRequest{Message: msg, Intent: intent})
	if route.Mode != RouteLocalSemantic {
		t.Fatalf("expected local semantic route, got %+v for intent %+v", route, intent)
	}
}

func TestRuntimeRouterSelectsLocalControlForVerifyHFJobCommand(t *testing.T) {
	msg := channel.InboundMessage{ID: "msg1", Channel: channel.KindQQ, Text: "/bot-verify-hf-job nvidia/LocateAnything-3B"}
	intent := ClassifyIntent(msg)
	route := NewRuntimeRouter().Select(PlanRequest{Message: msg, Intent: intent})
	if route.Mode != RouteLocalControl {
		t.Fatalf("expected local control route, got %+v", route)
	}
}

func TestRuntimeRouterCanDisableLocalSemanticFastPath(t *testing.T) {
	t.Setenv("AGENT_RUNTIME_LOCAL_SEMANTIC_FASTPATH", "false")
	msg := channel.InboundMessage{ID: "msg1", Channel: channel.KindQQ, Text: "请帮我规划 ShanghaiTech 数据接入"}
	intent := ClassifyIntent(msg)
	route := NewRuntimeRouter().Select(PlanRequest{Message: msg, Intent: intent})
	if route.Mode != RouteExternalPlan {
		t.Fatalf("expected external planner route when disabled, got %+v", route)
	}
}
