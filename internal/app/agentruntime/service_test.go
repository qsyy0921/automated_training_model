package agentruntime

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/app/runtimeworkflow"
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
	if plane.submitted.WorkflowID != runtimeworkflow.DefaultWorkflowID {
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
	traces := svc.ListTraces(10)
	if len(traces) != 1 {
		t.Fatalf("expected one trace, got %d", len(traces))
	}
	if len(traces[0].ToolIDs) != 1 || traces[0].ToolIDs[0] != "intake.plan" {
		t.Fatalf("expected intake.plan tool trace, got %+v", traces[0].ToolIDs)
	}
	if traces[0].Metadata["plan_id"] == "" {
		t.Fatalf("expected plan metadata, got %+v", traces[0].Metadata)
	}
	if traces[0].Metadata["workflow_id"] == "" || traces[0].Metadata["workflow_status"] == "" {
		t.Fatalf("expected workflow metadata, got %+v", traces[0].Metadata)
	}
	if traces[0].Metadata["dry_run"] != "true" {
		t.Fatalf("expected dry-run plan metadata, got %+v", traces[0].Metadata)
	}
}

func TestImageAttachmentRoutesToVisionTool(t *testing.T) {
	svc := NewService(&fakeAgentPlane{})
	out, err := svc.HandleChannelMessage(context.Background(), channel.InboundMessage{
		ID:        "msg1",
		Channel:   channel.KindQQ,
		AccountID: "default",
		Peer:      channel.Peer{Channel: channel.KindQQ, AccountID: "default", Kind: channel.PeerKindGroup, ID: "20001"},
		SenderID:  "10001",
		Text:      "check this frame",
		Attachments: []channel.Attachment{{
			ID:        "att1",
			Name:      "frame.png",
			MediaType: "image/png",
			Status:    channel.AttachmentReceived,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Text == "" {
		t.Fatal("expected vision planning reply")
	}
	traces := svc.ListTraces(10)
	if len(traces) != 1 {
		t.Fatalf("expected one trace, got %d", len(traces))
	}
	if len(traces[0].ToolIDs) != 1 || traces[0].ToolIDs[0] != "vlm.inspect" {
		t.Fatalf("expected vlm.inspect tool trace, got %+v", traces[0].ToolIDs)
	}
	if traces[0].AgentID != "vision-agent" {
		t.Fatalf("expected vision-agent trace, got %s", traces[0].AgentID)
	}
	if traces[0].Metadata["model"] != "mimo-v2.5" {
		t.Fatalf("expected vision model metadata, got %+v", traces[0].Metadata)
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
			Params: map[string]string{"workflow_id": runtimeworkflow.DefaultWorkflowID},
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

func TestHighRiskToolPreflightCanRequireApproval(t *testing.T) {
	t.Setenv("AGENT_RUNTIME_REQUIRE_HIGH_RISK_TOOL_APPROVAL", "true")
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
			Params: map[string]string{"repo_id": "nvidia/LocateAnything-3B"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "approval_required" {
		t.Fatalf("expected approval_required, got %+v", result)
	}
}

func TestToolPreflightRejectsUnknownParamsBeforeExecution(t *testing.T) {
	executor := NewGoToolExecutor(&fakeAgentPlane{}, nil)
	msg := channel.InboundMessage{
		ID:        "msg1",
		Channel:   channel.KindQQ,
		AccountID: "default",
		Peer:      channel.Peer{Channel: channel.KindQQ, AccountID: "default", Kind: channel.PeerKindDirect, ID: "10001"},
		SenderID:  "10001",
		Text:      "verify model",
	}
	session := BuildSessionContext(msg, DelegationDecision{AgentID: "planner-agent"})
	result, err := executor.Execute(context.Background(), ToolExecutionRequest{
		Message: msg,
		Session: session,
		Intent:  Intent{Kind: IntentChat},
		ToolCalls: []ToolCall{{
			ID:     "call-1",
			ToolID: "model.verify_hf",
			Params: map[string]string{"repo_id": "nvidia/LocateAnything-3B", "shell": "bad"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "preflight_failed" {
		t.Fatalf("expected preflight_failed, got %+v", result)
	}
}

func TestModelDownloadDefaultPolicyAllowsExecution(t *testing.T) {
	if modelDownloadRequiresApproval(ToolCall{Params: map[string]string{}}) {
		t.Fatal("default runtime policy should grant model download permission")
	}
}

func TestModelDownloadDefaultPolicyQueuesAsyncJob(t *testing.T) {
	executor := NewGoToolExecutor(&fakeAgentPlane{}, nil)
	started := make(chan struct{})
	release := make(chan struct{})
	executor.runHFModelTool = func(ctx context.Context, call ToolCall, verifyOnly bool) (ToolExecutionResult, error) {
		close(started)
		<-release
		return ToolExecutionResult{
			ReplyText: "fake download completed",
			Status:    "ok",
			Metadata:  map[string]string{"repo_id": call.Params["repo_id"]},
		}, nil
	}
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
			Params: map[string]string{"repo_id": "nvidia/LocateAnything-3B"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "queued" {
		t.Fatalf("expected queued, got %s", result.Status)
	}
	if !strings.Contains(result.ReplyText, "model-job-") {
		t.Fatalf("expected job id in reply, got %q", result.ReplyText)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("expected background model job to start")
	}
	close(release)
	deadline := time.After(time.Second)
	for {
		jobs := executor.ListModelJobs(10)
		if len(jobs) != 1 {
			t.Fatalf("expected one model job, got %d", len(jobs))
		}
		if jobs[0].Status == "succeeded" {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("expected job to succeed, got %+v", jobs[0])
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestModelJobCancelAndResume(t *testing.T) {
	executor := NewGoToolExecutor(&fakeAgentPlane{}, nil)
	started := make(chan struct{})
	var startOnce sync.Once
	executor.runHFModelTool = func(ctx context.Context, call ToolCall, verifyOnly bool) (ToolExecutionResult, error) {
		startOnce.Do(func() { close(started) })
		<-ctx.Done()
		return ToolExecutionResult{}, ctx.Err()
	}
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
			Params: map[string]string{"repo_id": "nvidia/LocateAnything-3B"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	jobID := result.Metadata["job_id"]
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("expected background model job to start")
	}
	canceled, err := executor.CancelModelJob(jobID)
	if err != nil {
		t.Fatal(err)
	}
	if !canceled.CancelRequested {
		t.Fatalf("expected cancel requested, got %+v", canceled)
	}
	deadline := time.After(time.Second)
	for {
		job, ok := executor.GetModelJob(jobID)
		if !ok {
			t.Fatalf("job not found: %s", jobID)
		}
		if job.Status == "canceled" {
			if !job.Resumable {
				t.Fatalf("expected canceled job to be resumable: %+v", job)
			}
			break
		}
		select {
		case <-deadline:
			t.Fatalf("expected canceled status, got %+v", job)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	resumed, err := executor.ResumeModelJob(jobID)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.ParentID != jobID || (resumed.Status != "queued" && resumed.Status != "running") {
		t.Fatalf("unexpected resumed job: %+v", resumed)
	}
	_, _ = executor.CancelModelJob(resumed.ID)
}
