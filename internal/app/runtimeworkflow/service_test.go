package runtimeworkflow

import (
	"context"
	"testing"

	"github.com/qsyy0921/automated_training_model/internal/domain/agent"
	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

type fakeControlPlane struct {
	submitted agent.RunRequest
	runs      []agent.WorkflowRun
}

func (f *fakeControlPlane) SubmitWorkflowRun(ctx context.Context, req agent.RunRequest) (agent.WorkflowRun, error) {
	f.submitted = req
	return agent.WorkflowRun{ID: "run-1", WorkflowID: req.WorkflowID, DatasetID: req.DatasetID, TaskID: "task-1"}, nil
}

func (f *fakeControlPlane) ListRuns(ctx context.Context) ([]agent.WorkflowRun, error) {
	return f.runs, nil
}

func TestSubmitDryRunRequiresDryRun(t *testing.T) {
	plane := &fakeControlPlane{}
	svc := NewService(plane)

	_, err := svc.SubmitDryRun(context.Background(), SubmitDryRunRequest{
		Message: channel.InboundMessage{Channel: channel.KindQQ},
		Session: SessionRef{Key: "session-1", AgentID: "planner-agent"},
	})
	if err == nil {
		t.Fatal("expected dry-run guard error")
	}
	if plane.submitted.WorkflowID != "" {
		t.Fatalf("workflow should not be submitted: %+v", plane.submitted)
	}
}

func TestSubmitDryRunBuildsRunRequest(t *testing.T) {
	plane := &fakeControlPlane{}
	svc := NewService(plane)

	result, err := svc.SubmitDryRun(context.Background(), SubmitDryRunRequest{
		Message: channel.InboundMessage{
			Channel:   channel.KindQQ,
			AccountID: "default",
			Peer:      channel.Peer{Kind: channel.PeerKindDirect, ID: "10001"},
			SenderID:  "10001",
		},
		Session:   SessionRef{Key: "agent:planner-agent:qq:direct:10001", AgentID: "planner-agent"},
		DatasetID: "shanghaitech-original",
		DryRun:    true,
		Params:    map[string]string{"model_repo_id": "nvidia/LocateAnything-3B"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" {
		t.Fatalf("unexpected status: %s", result.Status)
	}
	if plane.submitted.WorkflowID != DefaultWorkflowID {
		t.Fatalf("unexpected workflow id: %s", plane.submitted.WorkflowID)
	}
	if plane.submitted.DatasetID != "shanghaitech-original" || !plane.submitted.DryRun {
		t.Fatalf("unexpected run request: %+v", plane.submitted)
	}
	if plane.submitted.Params["session_key"] == "" || plane.submitted.Params["model_repo_id"] != "nvidia/LocateAnything-3B" {
		t.Fatalf("missing params: %+v", plane.submitted.Params)
	}
}

func TestListRunsFormatsRecentRuns(t *testing.T) {
	svc := NewService(&fakeControlPlane{runs: []agent.WorkflowRun{{
		ID: "run-1", WorkflowID: DefaultWorkflowID, DatasetID: "dataset-1", TaskID: "task-1",
	}}})

	result, err := svc.ListRuns(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" || result.ReplyText == "" {
		t.Fatalf("unexpected result: %+v", result)
	}
}
