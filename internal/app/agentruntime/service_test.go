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
