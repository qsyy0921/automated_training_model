package agentruntime

import (
	"context"
	"testing"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

type fakePlanner struct {
	got PlanRequest
}

func (f *fakePlanner) Plan(ctx context.Context, req PlanRequest) (PlanResult, error) {
	f.got = req
	return PlanResult{
		Intent:     req.Intent,
		Delegation: req.Delegation,
		ToolCalls:  []ToolCall{{ID: "call-1", ToolID: "runtime.health"}},
	}, nil
}

type fakeTools struct {
	got ToolExecutionRequest
}

func (f *fakeTools) Execute(ctx context.Context, req ToolExecutionRequest) (ToolExecutionResult, error) {
	f.got = req
	return ToolExecutionResult{ReplyText: "ok"}, nil
}

func TestSessionRunnerPassesPlanToToolExecutor(t *testing.T) {
	planner := &fakePlanner{}
	tools := &fakeTools{}
	svc := NewServiceWithPorts(planner, tools, func() time.Time { return time.Unix(0, 0) })

	out, err := svc.HandleChannelMessage(context.Background(), channel.InboundMessage{
		ID:        "msg1",
		Channel:   channel.KindQQ,
		AccountID: "default",
		Peer:      channel.Peer{Channel: channel.KindQQ, AccountID: "default", Kind: channel.PeerKindDirect, ID: "10001"},
		SenderID:  "10001",
		Text:      "/bot-ping",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Text != "ok" {
		t.Fatalf("unexpected reply: %s", out.Text)
	}
	if planner.got.Session.Key != "agent:go-control-plane:qq:direct:10001" {
		t.Fatalf("unexpected planner session key: %s", planner.got.Session.Key)
	}
	if tools.got.Session.Key != planner.got.Session.Key {
		t.Fatalf("tool executor did not receive session key")
	}
	if tools.got.ToolCalls[0].ToolID != "runtime.health" {
		t.Fatalf("unexpected tool call: %s", tools.got.ToolCalls[0].ToolID)
	}
}

func TestDefaultSessionKeyFallsBackToSender(t *testing.T) {
	key := DefaultSessionKey("planner-agent", channel.InboundMessage{
		Channel:  channel.KindQQ,
		SenderID: "sender-1",
		Peer:     channel.Peer{Kind: channel.PeerKindDirect},
	})
	if key != "agent:planner-agent:qq:direct:sender-1" {
		t.Fatalf("unexpected session key: %s", key)
	}
}
