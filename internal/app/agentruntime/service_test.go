package agentruntime

import (
	"context"
	"testing"

	"github.com/qsyy0921/automated_training_model/internal/domain/agent"
	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

type fakeAgentPlane struct {
	runs      []agent.WorkflowRun
	submitted agent.RunRequest
}

func (f *fakeAgentPlane) SubmitWorkflowRun(ctx context.Context, req agent.RunRequest) (agent.WorkflowRun, error) {
	f.submitted = req
	run := agent.WorkflowRun{ID: "run_test", TaskID: "task_test", WorkflowID: req.WorkflowID, DatasetID: req.DatasetID, Status: "queued"}
	f.runs = append([]agent.WorkflowRun{run}, f.runs...)
	return run, nil
}

func (f *fakeAgentPlane) ListRuns(ctx context.Context) ([]agent.WorkflowRun, error) {
	return f.runs, nil
}

func TestBotRunDrySubmitsWorkflow(t *testing.T) {
	plane := &fakeAgentPlane{}
	svc := NewService(plane)
	out, err := svc.HandleChannelMessage(context.Background(), channel.InboundMessage{
		ID:        "msg1",
		Channel:   channel.KindQQ,
		AccountID: "default",
		Peer:      channel.Peer{Channel: channel.KindQQ, AccountID: "default", Kind: channel.PeerKindDirect, ID: "10001"},
		SenderID:  "10001",
		Text:      "/bot-run dry shanghaitech-original",
	})
	if err != nil {
		t.Fatal(err)
	}
	if plane.submitted.WorkflowID != defaultWorkflowID {
		t.Fatalf("unexpected workflow: %s", plane.submitted.WorkflowID)
	}
	if plane.submitted.DatasetID != "shanghaitech-original" {
		t.Fatalf("unexpected dataset: %s", plane.submitted.DatasetID)
	}
	if !plane.submitted.DryRun {
		t.Fatal("expected dry-run")
	}
	if out.Text == "" {
		t.Fatal("expected reply text")
	}
	snapshot := svc.Snapshot(10)
	if snapshot.SessionCount != 1 {
		t.Fatalf("expected one runtime session, got %d", snapshot.SessionCount)
	}
	if snapshot.TraceCount != 1 {
		t.Fatalf("expected one runtime trace, got %d", snapshot.TraceCount)
	}
}

func TestAttachmentMessageDoesNotWriteData(t *testing.T) {
	svc := NewService(&fakeAgentPlane{})
	out, err := svc.HandleChannelMessage(context.Background(), channel.InboundMessage{
		ID:          "msg1",
		Channel:     channel.KindQQ,
		AccountID:   "default",
		Peer:        channel.Peer{Channel: channel.KindQQ, AccountID: "default", Kind: channel.PeerKindGroup, ID: "20001"},
		SenderID:    "10001",
		Attachments: []channel.Attachment{{ID: "att1", Name: "sample.zip"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Text == "" {
		t.Fatal("expected intake planning reply")
	}
}

func TestWorkflowSubmitToolRequiresDryRunPreflight(t *testing.T) {
	plane := &fakeAgentPlane{}
	executor := NewGoToolExecutor(plane, nil)
	msg := channel.InboundMessage{
		ID:        "msg1",
		Channel:   channel.KindQQ,
		AccountID: "default",
		Peer:      channel.Peer{Channel: channel.KindQQ, AccountID: "default", Kind: channel.PeerKindDirect, ID: "10001"},
		SenderID:  "10001",
		Text:      "please run workflow",
	}
	session := BuildSessionContext(msg, DelegationDecision{AgentID: "planner-agent"})
	_, err := executor.Execute(context.Background(), ToolExecutionRequest{
		Message: msg,
		Session: session,
		Intent:  Intent{Kind: IntentChat},
		ToolCalls: []ToolCall{{
			ID:     "call-1",
			ToolID: "workflow.submit_run",
			Params: map[string]string{"workflow_id": defaultWorkflowID},
		}},
	})
	if err == nil {
		t.Fatal("expected dry-run preflight error")
	}
	if plane.submitted.WorkflowID != "" {
		t.Fatal("workflow should not be submitted when dry-run preflight fails")
	}
}

func TestModelDownloadCanBeRestrictedByServerPolicy(t *testing.T) {
	t.Setenv("AGENT_RUNTIME_REQUIRE_MODEL_DOWNLOAD_APPROVAL", "true")
	executor := NewGoToolExecutor(&fakeAgentPlane{}, nil)
	msg := channel.InboundMessage{
		ID:        "msg1",
		Channel:   channel.KindQQ,
		AccountID: "default",
		Peer:      channel.Peer{Channel: channel.KindQQ, AccountID: "default", Kind: channel.PeerKindDirect, ID: "10001"},
		SenderID:  "10001",
		Text:      "download model",
	}
	session := BuildSessionContext(msg, DelegationDecision{AgentID: "planner-agent"})
	result, err := executor.Execute(context.Background(), ToolExecutionRequest{
		Message: msg,
		Session: session,
		Intent:  Intent{Kind: IntentChat},
		ToolCalls: []ToolCall{{
			ID:     "call-1",
			ToolID: "model.download_hf",
			Params: map[string]string{
				"repo_id":   "nvidia/LocateAnything-3B",
				"local_dir": "data_lake/models/artifacts/huggingface/nvidia/LocateAnything-3B",
				"manifest":  "data_lake/catalog/models/nvidia_LocateAnything-3B.download.json",
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "approval_required" {
		t.Fatalf("expected approval_required, got %s", result.Status)
	}
}

func TestModelDownloadDefaultPolicyAllowsExecution(t *testing.T) {
	if modelDownloadRequiresApproval(ToolCall{Params: map[string]string{}}) {
		t.Fatal("default runtime policy should grant model download permission")
	}
}
