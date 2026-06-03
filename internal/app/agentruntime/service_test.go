package agentruntime

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/app/modelruntime"
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

func TestTextDataIntakePlanningUsesFastPathAndTrace(t *testing.T) {
	svc := NewService(&fakeAgentPlane{})
	out, err := svc.HandleChannelMessage(context.Background(), channel.InboundMessage{
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
	if out.Text == "" {
		t.Fatal("expected intake planning reply")
	}
	traces := svc.ListTraces(10)
	if len(traces) != 1 {
		t.Fatalf("expected one trace, got %d", len(traces))
	}
	if traces[0].AgentID != "data-intake-agent" {
		t.Fatalf("expected data-intake-agent trace, got %s", traces[0].AgentID)
	}
	if len(traces[0].ToolIDs) != 1 || traces[0].ToolIDs[0] != "intake.plan" {
		t.Fatalf("expected intake.plan tool trace, got %+v", traces[0].ToolIDs)
	}
	if traces[0].Metadata["dataset_name"] != "shanghaitech-original" {
		t.Fatalf("expected shanghaitech dataset metadata, got %+v", traces[0].Metadata)
	}
	if traces[0].Metadata["source_uri"] != "message://qq/msg1" {
		t.Fatalf("expected synthetic text source metadata, got %+v", traces[0].Metadata)
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
	if modelruntime.NewService().DownloadRequiresApproval(map[string]string{}) {
		t.Fatal("default runtime policy should grant model download permission")
	}
}

func TestModelDownloadDefaultPolicyQueuesAsyncJob(t *testing.T) {
	executor := NewGoToolExecutor(&fakeAgentPlane{}, nil)
	started := make(chan struct{})
	executor.runHFWorkerJob = func(ctx context.Context, req modelruntime.WorkerJobRequest) (modelruntime.WorkerJobResult, error) {
		close(started)
		return modelruntime.WorkerJobResult{
			TaskID:      req.TaskID,
			Status:      "completed",
			Message:     "fake download completed",
			Retryable:   false,
			Attempt:     1,
			MaxAttempts: 3,
			Heartbeat:   &modelruntime.WorkerHeartbeat{At: "2026-06-03T00:00:00Z", Status: "completed", Message: "done"},
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
	if result.Metadata["execution_path"] != "python-worker" {
		t.Fatalf("expected python-worker path, got %+v", result.Metadata)
	}
	if !strings.Contains(result.ReplyText, "model-job-") {
		t.Fatalf("expected job id in reply, got %q", result.ReplyText)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("expected background model job to start")
	}
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
	executor.runHFWorkerJob = func(ctx context.Context, req modelruntime.WorkerJobRequest) (modelruntime.WorkerJobResult, error) {
		startOnce.Do(func() { close(started) })
		<-ctx.Done()
		return modelruntime.WorkerJobResult{}, ctx.Err()
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

func TestModelDownloadCanFallbackToServiceRunner(t *testing.T) {
	t.Setenv("AGENT_RUNTIME_HF_DOWNLOAD_RUNNER", "service")
	executor := NewGoToolExecutor(&fakeAgentPlane{}, nil)
	started := make(chan struct{})
	release := make(chan struct{})
	executor.runHFModelTool = func(ctx context.Context, call ToolCall, verifyOnly bool) (ToolExecutionResult, error) {
		close(started)
		<-release
		return ToolExecutionResult{
			ReplyText: "fake service download completed",
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
	if result.Metadata["execution_path"] != "service" {
		t.Fatalf("expected service fallback metadata, got %+v", result.Metadata)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("expected service background model job to start")
	}
	close(release)
}

func TestModelDownloadDryRunQueuesPythonWorkerJob(t *testing.T) {
	executor := NewGoToolExecutor(&fakeAgentPlane{}, nil)
	started := make(chan struct{})
	executor.runHFWorkerJob = func(ctx context.Context, req modelruntime.WorkerJobRequest) (modelruntime.WorkerJobResult, error) {
		close(started)
		return modelruntime.WorkerJobResult{
			TaskID:      req.TaskID,
			Status:      "completed",
			Message:     "worker dry-run completed",
			Retryable:   false,
			Attempt:     1,
			MaxAttempts: 1,
			Heartbeat:   &modelruntime.WorkerHeartbeat{At: "2026-06-03T00:00:00Z", Status: "completed", Message: "done"},
			Artifacts:   []modelruntime.WorkerArtifact{{Name: "plan", URI: "artifact://dry-run/" + req.TaskID, Kind: "dry-run-plan"}},
			Logs:        []modelruntime.WorkerLog{{At: "2026-06-03T00:00:00Z", Level: "info", Message: "worker accepted"}},
			Stdout:      "{\"status\":\"completed\"}",
		}, nil
	}
	msg := channel.InboundMessage{
		ID:        "msg1",
		Channel:   channel.KindQQ,
		AccountID: "default",
		Peer:      channel.Peer{Channel: channel.KindQQ, AccountID: "default", Kind: channel.PeerKindDirect, ID: "10001"},
		SenderID:  "10001",
		Text:      "download model dry-run",
	}
	session := BuildSessionContext(msg, DelegationDecision{AgentID: "model-agent"})
	result, err := executor.Execute(context.Background(), ToolExecutionRequest{
		Message: msg,
		Session: session,
		Intent:  Intent{Kind: IntentChat},
		ToolCalls: []ToolCall{{
			ID:     "call-1",
			ToolID: "model.download_hf",
			Params: map[string]string{"repo_id": "nvidia/LocateAnything-3B", "dry_run": "true"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "queued" || result.Metadata["execution_path"] != "python-worker" {
		t.Fatalf("unexpected result: %+v", result)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("expected python worker job to start")
	}
	deadline := time.After(time.Second)
	for {
		jobs := executor.ListModelJobs(10)
		if len(jobs) != 1 {
			t.Fatalf("expected one model job, got %d", len(jobs))
		}
		job := jobs[0]
		if job.Status == "succeeded" {
			if job.WorkerHeartbeat == nil || job.WorkerHeartbeat.Status != "completed" {
				t.Fatalf("expected worker heartbeat, got %+v", job)
			}
			if len(job.Artifacts) != 1 || job.Attempt != 1 || job.MaxAttempts != 1 {
				t.Fatalf("expected worker artifacts and retry metadata, got %+v", job)
			}
			if !strings.Contains(job.Stdout, "\"completed\"") {
				t.Fatalf("expected stdout summary, got %+v", job)
			}
			return
		}
		select {
		case <-deadline:
			t.Fatalf("expected worker job to succeed, got %+v", job)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestModelVerifyCanQueuePythonWorkerJob(t *testing.T) {
	executor := NewGoToolExecutor(&fakeAgentPlane{}, nil)
	started := make(chan struct{})
	executor.runHFWorkerJob = func(ctx context.Context, req modelruntime.WorkerJobRequest) (modelruntime.WorkerJobResult, error) {
		close(started)
		return modelruntime.WorkerJobResult{
			TaskID:      req.TaskID,
			Status:      "completed",
			Message:     "worker verify completed",
			Retryable:   false,
			Attempt:     1,
			MaxAttempts: 1,
			Heartbeat:   &modelruntime.WorkerHeartbeat{At: "2026-06-03T00:00:00Z", Status: "completed", Message: "verified"},
			Artifacts:   []modelruntime.WorkerArtifact{{Name: "manifest", URI: "artifact://verify/" + req.TaskID, Kind: "manifest"}},
			Logs:        []modelruntime.WorkerLog{{At: "2026-06-03T00:00:00Z", Level: "info", Message: "worker verify accepted"}},
			Stdout:      "{\"complete\":true}",
		}, nil
	}
	msg := channel.InboundMessage{
		ID:        "msg1",
		Channel:   channel.KindQQ,
		AccountID: "default",
		Peer:      channel.Peer{Channel: channel.KindQQ, AccountID: "default", Kind: channel.PeerKindDirect, ID: "10001"},
		SenderID:  "10001",
		Text:      "verify model",
	}
	session := BuildSessionContext(msg, DelegationDecision{AgentID: "model-agent"})
	result, err := executor.Execute(context.Background(), ToolExecutionRequest{
		Message: msg,
		Session: session,
		Intent:  Intent{Kind: IntentChat},
		ToolCalls: []ToolCall{{
			ID:     "call-1",
			ToolID: "model.verify_hf",
			Params: map[string]string{"repo_id": "nvidia/LocateAnything-3B", "job": "true"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "queued" || result.Metadata["execution_path"] != "python-worker" || result.Metadata["verify_only"] != "true" {
		t.Fatalf("unexpected verify result: %+v", result)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("expected python verify worker job to start")
	}
	deadline := time.After(time.Second)
	for {
		jobs := executor.ListModelJobs(10)
		if len(jobs) != 1 {
			t.Fatalf("expected one model job, got %d", len(jobs))
		}
		job := jobs[0]
		if job.Status == "succeeded" {
			if job.Kind != "model.verify_hf" || !job.VerifyOnly {
				t.Fatalf("expected verify job kind/flag, got %+v", job)
			}
			if job.WorkerHeartbeat == nil || job.WorkerHeartbeat.Message != "verified" {
				t.Fatalf("expected verify heartbeat, got %+v", job)
			}
			if len(job.Artifacts) != 1 || !strings.Contains(job.Stdout, "\"complete\":true") {
				t.Fatalf("expected verify artifacts/stdout, got %+v", job)
			}
			return
		}
		select {
		case <-deadline:
			t.Fatalf("expected verify worker job to succeed, got %+v", job)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestBotVerifyHFJobCommandQueuesWorkerJobThroughRuntime(t *testing.T) {
	plane := &fakeAgentPlane{}
	executor := NewGoToolExecutor(plane, nil)
	started := make(chan struct{})
	executor.runHFWorkerJob = func(ctx context.Context, req modelruntime.WorkerJobRequest) (modelruntime.WorkerJobResult, error) {
		close(started)
		return modelruntime.WorkerJobResult{
			TaskID:      req.TaskID,
			Status:      "completed",
			Message:     "worker verify completed",
			Retryable:   false,
			Attempt:     1,
			MaxAttempts: 1,
			Heartbeat:   &modelruntime.WorkerHeartbeat{At: "2026-06-03T00:00:00Z", Status: "completed", Message: "verified"},
		}, nil
	}
	svc := NewServiceWithPorts(NewRulePlanner(), executor, time.Now)
	reply, err := svc.HandleChannelMessage(context.Background(), channel.InboundMessage{
		ID:        "msg1",
		Channel:   channel.KindQQ,
		AccountID: "default",
		Peer:      channel.Peer{Channel: channel.KindQQ, AccountID: "default", Kind: channel.PeerKindDirect, ID: "10001"},
		SenderID:  "10001",
		Text:      "/bot-verify-hf-job nvidia/LocateAnything-3B",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(reply.Text, "job=") {
		t.Fatalf("expected queued job reply, got %q", reply.Text)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("expected runtime verify worker job to start")
	}
	deadline := time.After(time.Second)
	for {
		jobs := svc.ListModelJobs(10)
		if len(jobs) != 1 {
			t.Fatalf("expected one model job, got %d", len(jobs))
		}
		job := jobs[0]
		if job.Status == "succeeded" {
			if job.Kind != "model.verify_hf" || !job.VerifyOnly {
				t.Fatalf("expected verify worker job, got %+v", job)
			}
			traces := svc.ListTraces(10)
			if len(traces) != 1 || len(traces[0].ToolIDs) != 1 || traces[0].ToolIDs[0] != "model.verify_hf" {
				t.Fatalf("expected verify trace, got %+v", traces)
			}
			return
		}
		select {
		case <-deadline:
			t.Fatalf("expected verify worker job to succeed, got %+v", job)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestLocateAnythingSmokeCanQueuePythonWorkerJob(t *testing.T) {
	executor := NewGoToolExecutor(&fakeAgentPlane{}, nil)
	started := make(chan struct{})
	executor.runHFWorkerJob = func(ctx context.Context, req modelruntime.WorkerJobRequest) (modelruntime.WorkerJobResult, error) {
		close(started)
		return modelruntime.WorkerJobResult{
			TaskID:      req.TaskID,
			Status:      "completed",
			Message:     "worker smoke completed",
			Retryable:   false,
			Attempt:     1,
			MaxAttempts: 1,
			Heartbeat:   &modelruntime.WorkerHeartbeat{At: "2026-06-03T00:00:00Z", Status: "completed", Message: "smoked"},
			Artifacts:   []modelruntime.WorkerArtifact{{Name: "report", URI: "artifact://smoke/" + req.TaskID, Kind: "smoke-report"}},
			Logs:        []modelruntime.WorkerLog{{At: "2026-06-03T00:00:00Z", Level: "info", Message: "worker smoke accepted"}},
			Stdout:      "{\"status\":\"ok\",\"completed\":{\"model_load\":true,\"real_inference\":false}}",
		}, nil
	}
	msg := channel.InboundMessage{
		ID:        "msg1",
		Channel:   channel.KindQQ,
		AccountID: "default",
		Peer:      channel.Peer{Channel: channel.KindQQ, AccountID: "default", Kind: channel.PeerKindDirect, ID: "10001"},
		SenderID:  "10001",
		Text:      "smoke model",
	}
	session := BuildSessionContext(msg, DelegationDecision{AgentID: "model-agent"})
	result, err := executor.Execute(context.Background(), ToolExecutionRequest{
		Message: msg,
		Session: session,
		Intent:  Intent{Kind: IntentChat},
		ToolCalls: []ToolCall{{
			ID:     "call-1",
			ToolID: "model.smoke_locateanything",
			Params: map[string]string{
				"model_dir": "data_lake/models/artifacts/huggingface/nvidia/LocateAnything-3B",
				"data_root": "data_lake/raw/datasets/shanghaitech/original",
				"output":    "data_lake/catalog/models/nvidia_LocateAnything-3B.smoke.json",
				"job":       "true",
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "queued" || result.Metadata["execution_path"] != "python-worker" {
		t.Fatalf("unexpected smoke result: %+v", result)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("expected python smoke worker job to start")
	}
	deadline := time.After(time.Second)
	for {
		jobs := executor.ListModelJobs(10)
		if len(jobs) != 1 {
			t.Fatalf("expected one model job, got %d", len(jobs))
		}
		job := jobs[0]
		if job.Status == "succeeded" {
			if job.Kind != "model.smoke_locateanything" {
				t.Fatalf("expected smoke job kind, got %+v", job)
			}
			if job.WorkerHeartbeat == nil || job.WorkerHeartbeat.Message != "smoked" {
				t.Fatalf("expected smoke heartbeat, got %+v", job)
			}
			if len(job.Artifacts) != 1 || !strings.Contains(job.Stdout, "\"model_load\":true") {
				t.Fatalf("expected smoke artifacts/stdout, got %+v", job)
			}
			return
		}
		select {
		case <-deadline:
			t.Fatalf("expected smoke worker job to succeed, got %+v", job)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestTrainingDryRunCanQueuePythonWorkerJob(t *testing.T) {
	executor := NewGoToolExecutor(&fakeAgentPlane{}, nil)
	started := make(chan struct{})
	executor.runHFWorkerJob = func(ctx context.Context, req modelruntime.WorkerJobRequest) (modelruntime.WorkerJobResult, error) {
		close(started)
		return modelruntime.WorkerJobResult{
			TaskID:      req.TaskID,
			Status:      "completed",
			Message:     "training dry-run completed",
			Retryable:   false,
			Attempt:     1,
			MaxAttempts: 1,
			Heartbeat:   &modelruntime.WorkerHeartbeat{At: "2026-06-03T00:00:00Z", Status: "completed", Message: "planned"},
			Artifacts:   []modelruntime.WorkerArtifact{{Name: "plan", URI: "artifact://dry-run/" + req.TaskID, Kind: "dry-run-plan", Metadata: map[string]string{"target_task": "detection", "model_family": "yolo11n"}}},
			Logs:        []modelruntime.WorkerLog{{At: "2026-06-03T00:00:00Z", Level: "info", Message: "training dry-run accepted"}},
			Stdout:      "{\"status\":\"completed\"}",
		}, nil
	}
	msg := channel.InboundMessage{
		ID:        "msg1",
		Channel:   channel.KindQQ,
		AccountID: "default",
		Peer:      channel.Peer{Channel: channel.KindQQ, AccountID: "default", Kind: channel.PeerKindDirect, ID: "10001"},
		SenderID:  "10001",
		Text:      "train model",
	}
	session := BuildSessionContext(msg, DelegationDecision{AgentID: "training-agent"})
	result, err := executor.Execute(context.Background(), ToolExecutionRequest{
		Message: msg,
		Session: session,
		Intent:  Intent{Kind: IntentTrainingDryRun, DatasetID: "shanghaitech-original"},
		ToolCalls: []ToolCall{{
			ID:     "call-1",
			ToolID: "training.run",
			Params: map[string]string{
				"dataset_id":   "shanghaitech-original",
				"target_task":  "detection",
				"model_family": "yolo11n",
				"dry_run":      "true",
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "queued" || result.Metadata["execution_path"] != "python-worker" {
		t.Fatalf("unexpected training result: %+v", result)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("expected python training worker job to start")
	}
	deadline := time.After(time.Second)
	for {
		jobs := executor.ListModelJobs(10)
		if len(jobs) != 1 {
			t.Fatalf("expected one model job, got %d", len(jobs))
		}
		job := jobs[0]
		if job.Status == "succeeded" {
			if job.Kind != "training.run" {
				t.Fatalf("expected training job kind, got %+v", job)
			}
			if job.WorkerHeartbeat == nil || job.WorkerHeartbeat.Message != "planned" {
				t.Fatalf("expected training heartbeat, got %+v", job)
			}
			if len(job.Artifacts) != 1 || job.Artifacts[0].Metadata["model_family"] != "yolo11n" {
				t.Fatalf("expected training artifacts/metadata, got %+v", job)
			}
			return
		}
		select {
		case <-deadline:
			t.Fatalf("expected training worker job to succeed, got %+v", job)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestBotTrainDryCommandQueuesWorkerJobThroughRuntime(t *testing.T) {
	plane := &fakeAgentPlane{}
	executor := NewGoToolExecutor(plane, nil)
	started := make(chan struct{})
	executor.runHFWorkerJob = func(ctx context.Context, req modelruntime.WorkerJobRequest) (modelruntime.WorkerJobResult, error) {
		close(started)
		return modelruntime.WorkerJobResult{
			TaskID:      req.TaskID,
			Status:      "completed",
			Message:     "training dry-run completed",
			Retryable:   false,
			Attempt:     1,
			MaxAttempts: 1,
			Heartbeat:   &modelruntime.WorkerHeartbeat{At: "2026-06-03T00:00:00Z", Status: "completed", Message: "planned"},
		}, nil
	}
	svc := NewServiceWithPorts(NewRulePlanner(), executor, time.Now)
	reply, err := svc.HandleChannelMessage(context.Background(), channel.InboundMessage{
		ID:        "msg1",
		Channel:   channel.KindQQ,
		AccountID: "default",
		Peer:      channel.Peer{Channel: channel.KindQQ, AccountID: "default", Kind: channel.PeerKindDirect, ID: "10001"},
		SenderID:  "10001",
		Text:      "/bot-train-dry shanghaitech-original detection yolo11n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(reply.Text, "job=") {
		t.Fatalf("expected queued job reply, got %q", reply.Text)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("expected runtime training worker job to start")
	}
	deadline := time.After(time.Second)
	for {
		jobs := svc.ListModelJobs(10)
		if len(jobs) != 1 {
			t.Fatalf("expected one model job, got %d", len(jobs))
		}
		job := jobs[0]
		if job.Status == "succeeded" {
			if job.Kind != "training.run" {
				t.Fatalf("expected training worker job, got %+v", job)
			}
			traces := svc.ListTraces(10)
			if len(traces) != 1 || len(traces[0].ToolIDs) != 1 || traces[0].ToolIDs[0] != "training.run" {
				t.Fatalf("expected training trace, got %+v", traces)
			}
			return
		}
		select {
		case <-deadline:
			t.Fatalf("expected training worker job to succeed, got %+v", job)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestRecentModelJobLogsAndTerminalStatus(t *testing.T) {
	job := ModelJob{ID: "job1", Status: "succeeded", Logs: []ModelJobLog{
		{At: time.Unix(1, 0), Level: "info", Message: "one"},
		{At: time.Unix(2, 0), Level: "info", Message: "two"},
		{At: time.Unix(3, 0), Level: "info", Message: "three"},
	}}
	logs := RecentModelJobLogs(job, 2)
	if len(logs) != 2 || logs[0].Message != "two" || logs[1].Message != "three" {
		t.Fatalf("unexpected recent logs: %+v", logs)
	}
	if !IsTerminalModelJobStatus("succeeded") || !IsTerminalModelJobStatus("interrupted") || IsTerminalModelJobStatus("running") {
		t.Fatal("unexpected terminal status classification")
	}
}
