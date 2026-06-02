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

type emptyPlanner struct {
	got PlanRequest
}

func (f *emptyPlanner) Plan(ctx context.Context, req PlanRequest) (PlanResult, error) {
	f.got = req
	return PlanResult{Intent: req.Intent, Delegation: DelegationDecision{AgentID: "planner-agent"}, Status: "planned"}, nil
}

type multiToolPlanner struct {
	got PlanRequest
}

func (f *multiToolPlanner) Plan(ctx context.Context, req PlanRequest) (PlanResult, error) {
	f.got = req
	return PlanResult{
		Intent:     req.Intent,
		Delegation: DelegationDecision{AgentID: "planner-agent"},
		Status:     "planned",
		ToolCalls: []ToolCall{
			{ID: "call-1", ToolID: "vlm.inspect"},
			{ID: "call-2", ToolID: "intake.plan", Params: map[string]string{"attachment_id": "att1"}},
		},
	}, nil
}

type fakeTools struct {
	got ToolExecutionRequest
}

func (f *fakeTools) Execute(ctx context.Context, req ToolExecutionRequest) (ToolExecutionResult, error) {
	f.got = req
	return ToolExecutionResult{ReplyText: "ok"}, nil
}

type streamingFakeTools struct {
	got ToolExecutionRequest
}

func (f *streamingFakeTools) Execute(ctx context.Context, req ToolExecutionRequest) (ToolExecutionResult, error) {
	f.got = req
	return ToolExecutionResult{ReplyText: "ok", Status: "ok"}, nil
}

func (f *streamingFakeTools) ExecuteStream(ctx context.Context, req ToolExecutionRequest, emit func(RuntimeStreamEvent)) (ToolExecutionResult, error) {
	f.got = req
	if emit != nil {
		emit(RuntimeStreamEvent{Type: "tool_progress", ToolID: "runtime.health", ToolIDs: []string{"runtime.health"}, Status: "running", Message: "tool_start: running tool handler"})
	}
	return ToolExecutionResult{ReplyText: "ok", Status: "ok"}, nil
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
		Text:      "帮我规划一个小模型训练任务",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Text != "ok" {
		t.Fatalf("unexpected reply: %s", out.Text)
	}
	if planner.got.Session.Key != "agent:planner-agent:qq:direct:10001" {
		t.Fatalf("unexpected planner session key: %s", planner.got.Session.Key)
	}
	if tools.got.Session.Key != planner.got.Session.Key {
		t.Fatalf("tool executor did not receive session key")
	}
	if tools.got.ToolCalls[0].ToolID != "runtime.health" {
		t.Fatalf("unexpected tool call: %s", tools.got.ToolCalls[0].ToolID)
	}
}

func TestSessionRunnerStreamsToolProgress(t *testing.T) {
	planner := &fakePlanner{}
	tools := &streamingFakeTools{}
	svc := NewServiceWithPorts(planner, tools, func() time.Time { return time.Unix(0, 0) })
	events := []RuntimeStreamEvent{}
	out, err := svc.HandleChannelMessageStream(context.Background(), channel.InboundMessage{
		ID:        "msg1",
		Channel:   channel.KindQQ,
		AccountID: "default",
		Peer:      channel.Peer{Channel: channel.KindQQ, AccountID: "default", Kind: channel.PeerKindDirect, ID: "10001"},
		SenderID:  "10001",
		Text:      "帮我检查运行时",
	}, func(event RuntimeStreamEvent) {
		events = append(events, event)
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Text != "ok" {
		t.Fatalf("unexpected reply: %s", out.Text)
	}
	foundProgress := false
	for _, event := range events {
		if event.Type == "tool_progress" && event.ToolID == "runtime.health" && event.Session == "agent:planner-agent:qq:direct:10001" {
			foundProgress = true
		}
	}
	if !foundProgress {
		t.Fatalf("expected tool_progress event with session, got %+v", events)
	}
}

func TestRuntimeAboutUsesLocalFastPath(t *testing.T) {
	planner := &fakePlanner{}
	tools := &fakeTools{}
	svc := NewServiceWithPorts(planner, tools, func() time.Time { return time.Unix(0, 0) })

	out, err := svc.HandleChannelMessage(context.Background(), channel.InboundMessage{
		ID:        "msg1",
		Channel:   channel.KindQQ,
		AccountID: "default",
		Peer:      channel.Peer{Channel: channel.KindQQ, AccountID: "default", Kind: channel.PeerKindDirect, ID: "10001"},
		SenderID:  "10001",
		Text:      "你好，你是谁",
	})
	if err != nil {
		t.Fatal(err)
	}
	if planner.got.Message.ID != "" {
		t.Fatalf("runtime about should not call external planner, got %+v", planner.got)
	}
	if tools.got.Message.ID != "" {
		t.Fatalf("runtime about should not call tool executor, got %+v", tools.got)
	}
	if out.Text == "" || out.Text == "ok" {
		t.Fatalf("unexpected runtime about reply: %q", out.Text)
	}
}

func TestKnownModelInstallUsesLocalSemanticPlan(t *testing.T) {
	planner := &fakePlanner{}
	tools := &fakeTools{}
	svc := NewServiceWithPorts(planner, tools, func() time.Time { return time.Unix(0, 0) })

	_, err := svc.HandleChannelMessage(context.Background(), channel.InboundMessage{
		ID:        "msg1",
		Channel:   channel.KindQQ,
		AccountID: "default",
		Peer:      channel.Peer{Channel: channel.KindQQ, AccountID: "default", Kind: channel.PeerKindDirect, ID: "10001"},
		SenderID:  "10001",
		Text:      "请帮我下载 HuggingFace nvidia/LocateAnything-3B 模型",
	})
	if err != nil {
		t.Fatal(err)
	}
	if planner.got.Message.ID != "" {
		t.Fatalf("known model install should use local semantic plan, got %+v", planner.got)
	}
	if tools.got.Delegation.AgentID != "model-agent" {
		t.Fatalf("expected model-agent delegation, got %+v", tools.got.Delegation)
	}
	if len(tools.got.ToolCalls) != 1 || tools.got.ToolCalls[0].ToolID != "model.download_hf" {
		t.Fatalf("unexpected model install tool calls: %+v", tools.got.ToolCalls)
	}
}

func TestKnownDataIntakePlanningUsesLocalSemanticPlan(t *testing.T) {
	planner := &fakePlanner{}
	tools := &fakeTools{}
	svc := NewServiceWithPorts(planner, tools, func() time.Time { return time.Unix(0, 0) })

	_, err := svc.HandleChannelMessage(context.Background(), channel.InboundMessage{
		ID:        "msg1",
		Channel:   channel.KindQQ,
		AccountID: "default",
		Peer:      channel.Peer{Channel: channel.KindQQ, AccountID: "default", Kind: channel.PeerKindDirect, ID: "10001"},
		SenderID:  "10001",
		Text:      "请帮我规划 ShanghaiTech 数据接入",
	})
	if err != nil {
		t.Fatal(err)
	}
	if planner.got.Message.ID != "" {
		t.Fatalf("known data intake planning should use local semantic plan, got %+v", planner.got)
	}
	if tools.got.Delegation.AgentID != "data-intake-agent" {
		t.Fatalf("expected data-intake-agent delegation, got %+v", tools.got.Delegation)
	}
	if len(tools.got.ToolCalls) != 1 || tools.got.ToolCalls[0].ToolID != "intake.plan" {
		t.Fatalf("unexpected data intake tool calls: %+v", tools.got.ToolCalls)
	}
}

func TestControlIntentUsesLocalFastPath(t *testing.T) {
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
	if planner.got.Message.ID != "" {
		t.Fatalf("control intent should not call external planner, got %+v", planner.got)
	}
	if tools.got.ToolCalls[0].ToolID != "runtime.health" {
		t.Fatalf("unexpected tool call: %s", tools.got.ToolCalls[0].ToolID)
	}
}

func TestDataIntakeIntentEnforcesMandatoryLocalPlan(t *testing.T) {
	planner := &emptyPlanner{}
	tools := &fakeTools{}
	svc := NewServiceWithPorts(planner, tools, func() time.Time { return time.Unix(0, 0) })

	out, err := svc.HandleChannelMessage(context.Background(), channel.InboundMessage{
		ID:        "msg1",
		Channel:   channel.KindQQ,
		AccountID: "default",
		Peer:      channel.Peer{Channel: channel.KindQQ, AccountID: "default", Kind: channel.PeerKindDirect, ID: "10001"},
		SenderID:  "10001",
		Text:      "请为 ShanghaiTech original 创建数据接入计划",
		Attachments: []channel.Attachment{{
			ID:        "att1",
			Name:      "shanghaitech-original.manifest",
			MediaType: "application/x-directory",
			SourceURI: "F:\\automated_training_model\\data_lake\\raw\\datasets\\shanghaitech\\original",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Text != "ok" {
		t.Fatalf("unexpected reply: %s", out.Text)
	}
	if planner.got.Intent.Kind != IntentDataIntake {
		t.Fatalf("expected external planner to see data_intake, got %+v", planner.got.Intent)
	}
	if tools.got.Delegation.AgentID != "data-intake-agent" {
		t.Fatalf("mandatory data intake plan should preserve data-intake-agent, got %+v", tools.got.Delegation)
	}
	if len(tools.got.ToolCalls) != 1 || tools.got.ToolCalls[0].ToolID != "intake.plan" {
		t.Fatalf("expected mandatory intake.plan fallback, got %+v", tools.got.ToolCalls)
	}
}

func TestVisionIntentRejectsPlannerExpandedToolChain(t *testing.T) {
	planner := &multiToolPlanner{}
	tools := &fakeTools{}
	svc := NewServiceWithPorts(planner, tools, func() time.Time { return time.Unix(0, 0) })

	_, err := svc.HandleChannelMessage(context.Background(), channel.InboundMessage{
		ID:        "msg1",
		Channel:   channel.KindQQ,
		AccountID: "default",
		Peer:      channel.Peer{Channel: channel.KindQQ, AccountID: "default", Kind: channel.PeerKindGroup, ID: "20001"},
		SenderID:  "10001",
		Text:      "请检查这张异常帧",
		Attachments: []channel.Attachment{{
			ID:        "att1",
			Name:      "frame_001.png",
			MediaType: "image/png",
			Status:    channel.AttachmentReceived,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if planner.got.Delegation.AgentID != "vision-agent" {
		t.Fatalf("expected external planner to see vision delegation, got %+v", planner.got.Delegation)
	}
	if tools.got.Delegation.AgentID != "vision-agent" {
		t.Fatalf("mandatory vision plan should preserve vision-agent, got %+v", tools.got.Delegation)
	}
	if len(tools.got.ToolCalls) != 1 || tools.got.ToolCalls[0].ToolID != "vlm.inspect" {
		t.Fatalf("expected exact vlm.inspect fallback, got %+v", tools.got.ToolCalls)
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
