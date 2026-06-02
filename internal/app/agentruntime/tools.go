package agentruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/app/intakeapp"
	"github.com/qsyy0921/automated_training_model/internal/app/modelruntime"
	"github.com/qsyy0921/automated_training_model/internal/app/runtimeworkflow"
	"github.com/qsyy0921/automated_training_model/internal/app/toolapp"
	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

type GoToolExecutor struct {
	now            func() time.Time
	modelJobs      ModelJobStore
	toolRunner     *toolapp.Runner[ToolExecutionRequest]
	intake         *intakeapp.Service
	models         *modelruntime.Service
	workflows      *runtimeworkflow.Service
	runHFModelTool func(context.Context, ToolCall, bool) (ToolExecutionResult, error)
	jobMu          sync.Mutex
	jobCancels     map[string]context.CancelFunc
}

func NewGoToolExecutor(agents AgentControlPlane, now func() time.Time) *GoToolExecutor {
	if now == nil {
		now = time.Now
	}
	executor := &GoToolExecutor{
		now:        now,
		modelJobs:  NewModelJobStore(now),
		intake:     intakeapp.NewService(intakeapp.NewMemoryRepository(), nil, intakeapp.NewDryRunPlanner(now)),
		models:     modelruntime.NewService(),
		workflows:  runtimeworkflow.NewService(agents),
		jobCancels: map[string]context.CancelFunc{},
	}
	executor.runHFModelTool = executor.runHFModelViaService
	executor.toolRunner = toolapp.NewRunner[ToolExecutionRequest](toolapp.DefaultCatalog(), toolPreflightPolicyFromEnv)
	executor.registerToolHandlers()
	return executor
}

func NewGoToolExecutorWithModelJobs(agents AgentControlPlane, now func() time.Time, modelJobs ModelJobStore) *GoToolExecutor {
	executor := NewGoToolExecutor(agents, now)
	if modelJobs != nil {
		executor.modelJobs = modelJobs
	}
	return executor
}

func NewGoToolExecutorWithStores(agents AgentControlPlane, now func() time.Time, modelJobs ModelJobStore, intakeRepo intakeapp.Repository) *GoToolExecutor {
	executor := NewGoToolExecutorWithModelJobs(agents, now, modelJobs)
	if intakeRepo != nil {
		executor.intake = intakeapp.NewService(intakeRepo, nil, intakeapp.NewDryRunPlanner(executor.now))
	}
	return executor
}

func (e *GoToolExecutor) Execute(ctx context.Context, req ToolExecutionRequest) (ToolExecutionResult, error) {
	return e.toolRunner.Execute(ctx, req, req.ToolCalls)
}

func toolPreflightPolicyFromEnv() toolapp.PreflightPolicy {
	return toolapp.PreflightPolicy{
		RequireExplicitApprovalForHighRisk: strings.EqualFold(strings.TrimSpace(os.Getenv("AGENT_RUNTIME_REQUIRE_HIGH_RISK_TOOL_APPROVAL")), "true"),
	}
}

func (e *GoToolExecutor) registerToolHandlers() {
	e.toolRunner.Register("runtime.health", func(ctx context.Context, req ToolExecutionRequest, call ToolCall) (ToolExecutionResult, error) {
		return ToolExecutionResult{ReplyText: "pong", Status: "ok"}, nil
	})
	e.toolRunner.Register("runtime.identify_actor", func(ctx context.Context, req ToolExecutionRequest, call ToolCall) (ToolExecutionResult, error) {
		return ToolExecutionResult{
			ReplyText: fmt.Sprintf("channel=%s account=%s peer=%s:%s sender=%s", req.Message.Channel, req.Message.AccountID, req.Message.Peer.Kind, req.Message.Peer.ID, req.Message.SenderID),
			Status:    "ok",
		}, nil
	})
	e.toolRunner.Register("runtime.status", func(ctx context.Context, req ToolExecutionRequest, call ToolCall) (ToolExecutionResult, error) {
		return ToolExecutionResult{
			ReplyText: fmt.Sprintf("Agent Gateway online. channel=%s account=%s runtime=ready agent_loop=python-agent-runtime text_model=mimo-v2.5-pro vision_model=mimo-v2.5 time=%s", req.Message.Channel, req.Message.AccountID, e.now().Format(time.RFC3339)),
			Status:    "ok",
		}, nil
	})
	e.toolRunner.Register("workflow.list_runs", func(ctx context.Context, req ToolExecutionRequest, call ToolCall) (ToolExecutionResult, error) {
		return e.listRuns(ctx)
	})
	e.toolRunner.Register("workflow.submit_run", func(ctx context.Context, req ToolExecutionRequest, call ToolCall) (ToolExecutionResult, error) {
		return e.submitWorkflowRun(ctx, req, call)
	})
	e.toolRunner.Register("intake.quarantine", func(ctx context.Context, req ToolExecutionRequest, call ToolCall) (ToolExecutionResult, error) {
		return ToolExecutionResult{
			ReplyText: fmt.Sprintf("已生成数据接入隔离计划：attachments=%d session=%s", len(req.Message.Attachments), req.Session.Key),
			Status:    "planned",
		}, nil
	})
	e.toolRunner.Register("intake.plan", func(ctx context.Context, req ToolExecutionRequest, call ToolCall) (ToolExecutionResult, error) {
		return e.planDataIntake(ctx, req, call)
	})
	e.toolRunner.Register("vlm.inspect", func(ctx context.Context, req ToolExecutionRequest, call ToolCall) (ToolExecutionResult, error) {
		return e.planVisionInspection(ctx, req, call)
	})
	e.toolRunner.Register("llm.plan", func(ctx context.Context, req ToolExecutionRequest, call ToolCall) (ToolExecutionResult, error) {
		return ToolExecutionResult{
			ReplyText: fmt.Sprintf("已进入 planner-agent 会话：%s；可通过 AGENT_RUNTIME_PLANNER=python 启用 Python/Mimo planner。", req.Session.Key),
			Status:    "planned",
		}, nil
	})
	e.toolRunner.Register("model.download_hf", func(ctx context.Context, req ToolExecutionRequest, call ToolCall) (ToolExecutionResult, error) {
		return e.downloadHFModel(ctx, call)
	})
	e.toolRunner.Register("model.verify_hf", func(ctx context.Context, req ToolExecutionRequest, call ToolCall) (ToolExecutionResult, error) {
		return e.verifyHFModel(ctx, call)
	})
	e.toolRunner.Register("model.smoke_locateanything", func(ctx context.Context, req ToolExecutionRequest, call ToolCall) (ToolExecutionResult, error) {
		return e.smokeLocateAnythingModel(ctx, call)
	})
}

func (e *GoToolExecutor) planDataIntake(ctx context.Context, req ToolExecutionRequest, call ToolCall) (ToolExecutionResult, error) {
	workflow, err := e.intake.PrepareWorkflowFromMessage(ctx, req.Message, intakeapp.PlanOptions{
		Mode:        intakeapp.PlanModeData,
		DatasetName: intakeDatasetName(req, call),
	})
	if err != nil {
		return ToolExecutionResult{}, err
	}
	plan := workflow.Plan
	return ToolExecutionResult{
		ReplyText: fmt.Sprintf("已生成 Data Intake Workflow：workflow=%s plan=%s agent=%s dataset=%s status=%s attachments=%d risk=%s；正式入湖前需要人工审批。", workflow.ID, plan.ID, req.Delegation.AgentID, plan.DatasetName, workflow.Status, len(workflow.Attachments), plan.RiskLevel),
		Status:    "planned",
		Metadata:  intakeWorkflowMetadata(workflow, req.Message),
	}, nil
}

func (e *GoToolExecutor) planVisionInspection(ctx context.Context, req ToolExecutionRequest, call ToolCall) (ToolExecutionResult, error) {
	workflow, err := e.intake.PrepareWorkflowFromMessage(ctx, req.Message, intakeapp.PlanOptions{
		Mode:        intakeapp.PlanModeVision,
		DatasetName: intakeDatasetName(req, call),
		ModelRoute:  "vision",
		Model:       "mimo-v2.5",
	})
	if err != nil {
		return ToolExecutionResult{}, err
	}
	plan := workflow.Plan
	metadata := intakeWorkflowMetadata(workflow, req.Message)
	metadata["model_route"] = "vision"
	metadata["model"] = "mimo-v2.5"
	return ToolExecutionResult{
		ReplyText: fmt.Sprintf("已生成视觉数据检查 Workflow：workflow=%s plan=%s agent=vision-agent route=mimo-v2.5 status=%s attachments=%d；当前 MVP 只做 quarantine/scan/plan/approval 边界，不自动写入 Data Lake。", workflow.ID, plan.ID, workflow.Status, len(workflow.Attachments)),
		Status:    "planned",
		Metadata:  metadata,
	}, nil
}

func intakeDatasetName(req ToolExecutionRequest, call ToolCall) string {
	if value := strings.TrimSpace(call.Params["dataset_id"]); value != "" {
		return value
	}
	if value := strings.TrimSpace(req.Intent.DatasetID); value != "" {
		return value
	}
	return ""
}

func intakePlanMetadata(plan channel.DataIntakePlan, msg channel.InboundMessage) map[string]string {
	planJSON, _ := json.Marshal(plan)
	metadata := map[string]string{
		"plan_id":          plan.ID,
		"dataset_name":     plan.DatasetName,
		"attachment_count": strconv.Itoa(len(msg.Attachments)),
		"risk_level":       plan.RiskLevel,
		"dry_run":          strconv.FormatBool(plan.DryRun),
		"approval":         strings.Join(plan.RequiredApprovals, ","),
		"plan_json":        string(planJSON),
	}
	if source := firstAttachmentSource(msg.Attachments); source != "" {
		metadata["source_uri"] = source
	}
	return metadata
}

func intakeWorkflowMetadata(workflow intakeapp.IntakeWorkflow, msg channel.InboundMessage) map[string]string {
	metadata := intakePlanMetadata(workflow.Plan, msg)
	if metadata["source_uri"] == "" {
		if source := firstAttachmentSource(workflow.Attachments); source != "" {
			metadata["source_uri"] = source
		}
	}
	workflowJSON, _ := json.Marshal(workflow)
	accepted := 0
	rejected := 0
	for _, report := range workflow.ScanReports {
		if report.Accepted {
			accepted++
		} else {
			rejected++
		}
	}
	metadata["workflow_id"] = workflow.ID
	metadata["workflow_status"] = string(workflow.Status)
	metadata["scan_accepted"] = strconv.Itoa(accepted)
	metadata["scan_rejected"] = strconv.Itoa(rejected)
	metadata["approval_required"] = strconv.FormatBool(workflow.ApprovalRequired)
	metadata["workflow_json"] = string(workflowJSON)
	return metadata
}

func firstAttachmentSource(attachments []channel.Attachment) string {
	for _, attachment := range attachments {
		if value := strings.TrimSpace(attachment.LocalURI); value != "" {
			return value
		}
		if value := strings.TrimSpace(attachment.SourceURI); value != "" {
			return value
		}
	}
	return ""
}

func (e *GoToolExecutor) downloadHFModel(ctx context.Context, call ToolCall) (ToolExecutionResult, error) {
	if e.models.DownloadRequiresApproval(call.Params) {
		repoID := strings.TrimSpace(call.Params["repo_id"])
		if repoID == "" {
			repoID = "nvidia/LocateAnything-3B"
		}
		return ToolExecutionResult{
			ReplyText: fmt.Sprintf("已生成 HuggingFace 模型下载预检：repo=%s。当前服务端启用了下载审批，真实下载需带 approved=true。", repoID),
			Status:    "approval_required",
		}, nil
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("AGENT_RUNTIME_HF_DOWNLOAD_SYNC")), "true") {
		return e.runHFModelTool(ctx, call, false)
	}
	return e.enqueueHFModelDownload(call)
}

func (e *GoToolExecutor) verifyHFModel(ctx context.Context, call ToolCall) (ToolExecutionResult, error) {
	return e.runHFModelTool(ctx, call, true)
}

type hfModelRequest struct {
	RepoID     string
	LocalDir   string
	Manifest   string
	VerifyOnly bool
}

func prepareHFModelRequest(call ToolCall, verifyOnly bool) (hfModelRequest, error) {
	req, err := modelruntime.PrepareHFModelRequest(call.Params, verifyOnly)
	if err != nil {
		return hfModelRequest{}, err
	}
	return hfModelRequest{RepoID: req.RepoID, LocalDir: req.LocalDir, Manifest: req.Manifest, VerifyOnly: req.VerifyOnly}, nil
}

func (e *GoToolExecutor) enqueueHFModelDownload(call ToolCall) (ToolExecutionResult, error) {
	req, err := prepareHFModelRequest(call, false)
	if err != nil {
		return ToolExecutionResult{}, err
	}
	return e.enqueueHFModelDownloadRequest(req, "", call)
}

func (e *GoToolExecutor) enqueueHFModelDownloadRequest(req hfModelRequest, parentID string, call ToolCall) (ToolExecutionResult, error) {
	created := e.now()
	job := e.modelJobs.Create(ModelJob{
		ID:              fmt.Sprintf("model-job-%d", created.UnixNano()),
		ParentID:        parentID,
		Kind:            "model.download_hf",
		RepoID:          req.RepoID,
		LocalDir:        req.LocalDir,
		Manifest:        req.Manifest,
		Status:          "queued",
		Message:         "queued by Agent Runtime",
		ProgressPercent: 0,
		Resumable:       true,
		Logs:            []ModelJobLog{{At: created, Level: "info", Message: "queued by Agent Runtime"}},
	})
	go e.runHFModelJob(job.ID, call)
	return ToolExecutionResult{
		ReplyText: fmt.Sprintf("HuggingFace 模型下载任务已排队：job=%s repo=%s。可通过 /api/runtime/model-jobs 或 `labelctl runtime model-jobs` 查看状态。", job.ID, req.RepoID),
		Status:    "queued",
		Metadata: map[string]string{
			"job_id":    job.ID,
			"repo_id":   req.RepoID,
			"local_dir": req.LocalDir,
			"manifest":  req.Manifest,
		},
	}, nil
}

func (e *GoToolExecutor) runHFModelJob(jobID string, call ToolCall) {
	started := e.now()
	e.modelJobs.Update(jobID, func(job *ModelJob) {
		job.Status = "running"
		job.StartedAt = &started
		job.Message = "running HuggingFace snapshot download"
		job.ProgressPercent = 10
		job.Logs = appendModelJobLog(job.Logs, started, "info", job.Message)
	})
	ctx, cancel := context.WithTimeout(context.Background(), modelruntime.HFDownloadTimeout())
	e.setModelJobCancel(jobID, cancel)
	defer cancel()
	defer e.clearModelJobCancel(jobID)
	result, err := e.runHFModelTool(ctx, call, false)
	finished := e.now()
	e.modelJobs.Update(jobID, func(job *ModelJob) {
		job.FinishedAt = &finished
		if ctx.Err() == context.Canceled || job.CancelRequested {
			job.Status = "canceled"
			job.Message = "download canceled"
			job.Resumable = true
			job.Logs = appendModelJobLog(job.Logs, finished, "warn", job.Message)
			return
		}
		if err != nil {
			job.Status = "failed"
			job.Error = err.Error()
			job.Message = "download failed"
			job.Resumable = true
			job.Logs = appendModelJobLog(job.Logs, finished, "error", err.Error())
			return
		}
		job.Status = "succeeded"
		job.Message = result.ReplyText
		job.Metadata = result.Metadata
		job.ProgressPercent = 100
		job.Resumable = false
		job.Logs = appendModelJobLog(job.Logs, finished, "info", result.ReplyText)
	})
}

func (e *GoToolExecutor) ListModelJobs(limit int) []ModelJob {
	return e.modelJobs.List(limit)
}

func (e *GoToolExecutor) ListIntakeWorkflows(ctx context.Context, limit int) ([]intakeapp.IntakeWorkflow, error) {
	return e.intake.ListWorkflows(ctx, limit)
}

func (e *GoToolExecutor) GetIntakeWorkflow(ctx context.Context, id string) (intakeapp.IntakeWorkflow, bool, error) {
	return e.intake.GetWorkflow(ctx, id)
}

func (e *GoToolExecutor) ApproveIntakeWorkflow(ctx context.Context, id string, by string, note string) (intakeapp.IntakeWorkflow, error) {
	return e.intake.ApproveWorkflow(ctx, id, by, note)
}

func (e *GoToolExecutor) RegisterIntakeWorkflow(ctx context.Context, id string, by string) (intakeapp.IntakeWorkflow, error) {
	return e.intake.RegisterWorkflow(ctx, id, by)
}

func (e *GoToolExecutor) GetModelJob(id string) (ModelJob, bool) {
	return e.modelJobs.Get(id)
}

func (e *GoToolExecutor) CancelModelJob(id string) (ModelJob, error) {
	job, ok := e.modelJobs.Get(id)
	if !ok {
		return ModelJob{}, fmt.Errorf("model job not found: %s", id)
	}
	if job.Status != "queued" && job.Status != "running" {
		return job, fmt.Errorf("model job %s cannot be canceled from status %s", id, job.Status)
	}
	now := e.now()
	e.modelJobs.Update(id, func(job *ModelJob) {
		job.CancelRequested = true
		job.Message = "cancel requested"
		job.Resumable = true
		job.Logs = appendModelJobLog(job.Logs, now, "warn", "cancel requested")
	})
	e.cancelRunningModelJob(id)
	updated, _ := e.modelJobs.Get(id)
	return updated, nil
}

func (e *GoToolExecutor) ResumeModelJob(id string) (ModelJob, error) {
	job, ok := e.modelJobs.Get(id)
	if !ok {
		return ModelJob{}, fmt.Errorf("model job not found: %s", id)
	}
	switch job.Status {
	case "failed", "interrupted", "canceled":
	default:
		return job, fmt.Errorf("model job %s cannot be resumed from status %s", id, job.Status)
	}
	if job.Kind != "model.download_hf" {
		return job, fmt.Errorf("model job %s kind %s does not support resume", id, job.Kind)
	}
	call := ToolCall{ID: "resume-" + id, ToolID: "model.download_hf", Params: map[string]string{
		"repo_id":   job.RepoID,
		"local_dir": job.LocalDir,
		"manifest":  job.Manifest,
	}}
	req, err := prepareHFModelRequest(call, false)
	if err != nil {
		return ModelJob{}, err
	}
	result, err := e.enqueueHFModelDownloadRequest(req, id, call)
	if err != nil {
		return ModelJob{}, err
	}
	newID := result.Metadata["job_id"]
	resumed, ok := e.modelJobs.Get(newID)
	if !ok {
		return ModelJob{}, fmt.Errorf("resumed model job not found: %s", newID)
	}
	return resumed, nil
}

func (e *GoToolExecutor) setModelJobCancel(id string, cancel context.CancelFunc) {
	e.jobMu.Lock()
	defer e.jobMu.Unlock()
	e.jobCancels[id] = cancel
}

func (e *GoToolExecutor) clearModelJobCancel(id string) {
	e.jobMu.Lock()
	defer e.jobMu.Unlock()
	delete(e.jobCancels, id)
}

func (e *GoToolExecutor) cancelRunningModelJob(id string) {
	e.jobMu.Lock()
	cancel := e.jobCancels[id]
	e.jobMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (e *GoToolExecutor) runHFModelViaService(ctx context.Context, call ToolCall, verifyOnly bool) (ToolExecutionResult, error) {
	result, err := e.models.RunHFModelTool(ctx, call.Params, verifyOnly)
	if err != nil {
		return ToolExecutionResult{}, err
	}
	return ToolExecutionResult{
		ReplyText: result.ReplyText,
		Status:    result.Status,
		Metadata:  result.Metadata,
	}, nil
}

func (e *GoToolExecutor) smokeLocateAnythingModel(ctx context.Context, call ToolCall) (ToolExecutionResult, error) {
	result, err := e.models.SmokeLocateAnything(ctx, call.Params)
	if err != nil {
		return ToolExecutionResult{}, err
	}
	return ToolExecutionResult{
		ReplyText: result.ReplyText,
		Status:    result.Status,
		Metadata:  result.Metadata,
	}, nil
}

func (e *GoToolExecutor) listRuns(ctx context.Context) (ToolExecutionResult, error) {
	result, err := e.workflows.ListRuns(ctx, 5)
	if err != nil {
		return ToolExecutionResult{}, err
	}
	return ToolExecutionResult{ReplyText: result.ReplyText, Status: result.Status}, nil
}

func (e *GoToolExecutor) submitWorkflowRun(ctx context.Context, req ToolExecutionRequest, call ToolCall) (ToolExecutionResult, error) {
	datasetID := "workspace-dataset"
	if req.Intent.DatasetID != "" {
		datasetID = req.Intent.DatasetID
	}
	if value := strings.TrimSpace(call.Params["dataset_id"]); value != "" {
		datasetID = value
	}
	result, err := e.workflows.SubmitDryRun(ctx, runtimeworkflow.SubmitDryRunRequest{
		Message:    req.Message,
		Session:    runtimeworkflow.SessionRef{Key: req.Session.Key, AgentID: req.Session.AgentID},
		WorkflowID: call.Params["workflow_id"],
		DatasetID:  datasetID,
		DryRun:     req.Intent.Kind == IntentSubmitDryRun || strings.EqualFold(call.Params["dry_run"], "true"),
		Params:     call.Params,
	})
	if err != nil {
		return ToolExecutionResult{}, err
	}
	return ToolExecutionResult{ReplyText: result.ReplyText, Status: result.Status}, nil
}
