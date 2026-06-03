package agentruntime

import (
	"context"
	"encoding/json"
	"errors"
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
	workerRunner   modelWorkerRunner
	workflows      *runtimeworkflow.Service
	runHFModelTool func(context.Context, ToolCall, bool) (ToolExecutionResult, error)
	runHFWorkerJob func(context.Context, modelruntime.WorkerJobRequest, func(modelruntime.WorkerRuntimeEvent)) (modelruntime.WorkerJobResult, error)
	jobMu          sync.Mutex
	jobCancels     map[string]context.CancelFunc
}

type hfWorkerJobOptions struct {
	Kind          string
	Action        string
	DryRun        bool
	Resumable     bool
	ReplyLabel    string
	QueuedMessage string
}

type modelWorkerRunner interface {
	Run(context.Context, modelruntime.WorkerJobRequest, func(modelruntime.WorkerRuntimeEvent)) (modelruntime.WorkerJobResult, error)
}

func NewGoToolExecutor(agents AgentControlPlane, now func() time.Time) *GoToolExecutor {
	if now == nil {
		now = time.Now
	}
	executor := &GoToolExecutor{
		now:          now,
		modelJobs:    NewModelJobStore(now),
		intake:       intakeapp.NewService(intakeapp.NewMemoryRepository(), nil, intakeapp.NewDryRunPlanner(now)),
		models:       modelruntime.NewService(),
		workerRunner: modelruntime.NewPythonModelWorkerRunner(),
		workflows:    runtimeworkflow.NewService(agents),
		jobCancels:   map[string]context.CancelFunc{},
	}
	executor.runHFModelTool = executor.runHFModelViaService
	executor.runHFWorkerJob = executor.runPythonModelWorkerJob
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

func (e *GoToolExecutor) ExecuteStream(ctx context.Context, req ToolExecutionRequest, emit func(RuntimeStreamEvent)) (ToolExecutionResult, error) {
	return e.toolRunner.ExecuteStream(ctx, req, req.ToolCalls, func(event toolapp.ProgressEvent) {
		if emit == nil {
			return
		}
		runtimeEvent := RuntimeStreamEvent{
			Type:    "tool_progress",
			Status:  event.Status,
			Message: event.Message,
			ToolID:  event.ToolID,
		}
		if event.ToolID != "" {
			runtimeEvent.ToolIDs = []string{event.ToolID}
		}
		if event.Type != "" {
			runtimeEvent.Message = strings.TrimSpace(event.Type + ": " + runtimeEvent.Message)
		}
		emit(runtimeEvent)
	})
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
	e.toolRunner.Register("training.run", func(ctx context.Context, req ToolExecutionRequest, call ToolCall) (ToolExecutionResult, error) {
		return e.submitTrainingDryRun(ctx, call)
	})
	e.toolRunner.Register("evaluation.run", func(ctx context.Context, req ToolExecutionRequest, call ToolCall) (ToolExecutionResult, error) {
		return e.submitEvaluationDryRun(ctx, call)
	})
	e.toolRunner.Register("deployment.run", func(ctx context.Context, req ToolExecutionRequest, call ToolCall) (ToolExecutionResult, error) {
		return e.submitDeploymentDryRun(ctx, call)
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
	if strings.EqualFold(strings.TrimSpace(call.Params["dry_run"]), "true") {
		return e.enqueueHFModelWorkerDryRun(call)
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("AGENT_RUNTIME_HF_DOWNLOAD_SYNC")), "true") {
		return e.runHFModelTool(ctx, call, false)
	}
	if hfDownloadRunnerModeFromEnv() == "service" {
		return e.enqueueHFModelDownload(call)
	}
	return e.enqueueHFModelWorkerDownload(call)
}

func hfDownloadRunnerModeFromEnv() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("AGENT_RUNTIME_HF_DOWNLOAD_RUNNER"))) {
	case "", "python-worker", "worker", "python":
		return "python-worker"
	case "service":
		return "service"
	default:
		return "python-worker"
	}
}

func hfVerifyRunnerModeFromEnv() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("AGENT_RUNTIME_HF_VERIFY_RUNNER"))) {
	case "python-worker", "worker", "python":
		return "python-worker"
	default:
		return "service"
	}
}

func locateAnythingSmokeRunnerModeFromEnv() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("AGENT_RUNTIME_LOCATEANYTHING_SMOKE_RUNNER"))) {
	case "python-worker", "worker", "python":
		return "python-worker"
	default:
		return "service"
	}
}

func (e *GoToolExecutor) enqueueHFModelWorkerDownload(call ToolCall) (ToolExecutionResult, error) {
	req, err := prepareHFModelRequest(call, false)
	if err != nil {
		return ToolExecutionResult{}, err
	}
	return e.enqueueHFModelWorkerJob(req, "", call, hfWorkerJobOptions{
		Kind:          "model.download_hf",
		Action:        "download_hf",
		DryRun:        false,
		Resumable:     true,
		ReplyLabel:    "Python worker 任务已排队",
		QueuedMessage: "queued python worker",
	})
}

func (e *GoToolExecutor) enqueueHFModelWorkerDryRun(call ToolCall) (ToolExecutionResult, error) {
	req, err := prepareHFModelRequest(call, false)
	if err != nil {
		return ToolExecutionResult{}, err
	}
	return e.enqueueHFModelWorkerJob(req, "", call, hfWorkerJobOptions{
		Kind:          "model.download_hf",
		Action:        "download_hf",
		DryRun:        true,
		Resumable:     true,
		ReplyLabel:    "Python worker dry-run 已排队",
		QueuedMessage: "queued python worker dry-run",
	})
}

func (e *GoToolExecutor) enqueueHFVerifyWorkerJob(call ToolCall) (ToolExecutionResult, error) {
	req, err := prepareHFModelRequest(call, true)
	if err != nil {
		return ToolExecutionResult{}, err
	}
	return e.enqueueHFModelWorkerJob(req, "", call, hfWorkerJobOptions{
		Kind:          "model.verify_hf",
		Action:        "verify_hf",
		DryRun:        false,
		Resumable:     false,
		ReplyLabel:    "Python worker 校验任务已排队",
		QueuedMessage: "queued python worker verify",
	})
}

func (e *GoToolExecutor) enqueueHFModelWorkerJob(req hfModelRequest, parentID string, call ToolCall, opts hfWorkerJobOptions) (ToolExecutionResult, error) {
	created := e.now()
	if strings.TrimSpace(opts.Kind) == "" {
		opts.Kind = "model.download_hf"
	}
	if strings.TrimSpace(opts.Action) == "" {
		opts.Action = "download_hf"
	}
	if strings.TrimSpace(opts.ReplyLabel) == "" {
		opts.ReplyLabel = "Python worker 任务已排队"
	}
	if strings.TrimSpace(opts.QueuedMessage) == "" {
		opts.QueuedMessage = "queued python worker"
	}
	job := e.modelJobs.Create(ModelJob{
		ID:              fmt.Sprintf("model-job-%d", created.UnixNano()),
		ParentID:        parentID,
		Kind:            opts.Kind,
		RepoID:          req.RepoID,
		LocalDir:        req.LocalDir,
		Manifest:        req.Manifest,
		VerifyOnly:      req.VerifyOnly,
		Status:          "queued",
		Message:         opts.QueuedMessage,
		ProgressPercent: 0,
		Resumable:       opts.Resumable,
		Retryable:       false,
		Logs:            []ModelJobLog{{At: created, Level: "info", Message: opts.QueuedMessage}},
		Metadata: map[string]string{
			"execution_path": "python-worker",
			"dry_run":        strconv.FormatBool(opts.DryRun),
			"verify_only":    strconv.FormatBool(req.VerifyOnly),
		},
	})
	go e.runHFModelWorkerJob(job.ID, modelruntime.WorkerJobRequest{
		TaskID:     job.ID,
		WorkflowID: "runtime-model-worker",
		AgentID:    "model-agent",
		ToolID:     opts.Kind,
		Action:     opts.Action,
		DatasetID:  strings.TrimSpace(call.Params["dataset_id"]),
		DryRun:     opts.DryRun,
		Params: map[string]string{
			"repo_id":     req.RepoID,
			"local_dir":   req.LocalDir,
			"manifest":    req.Manifest,
			"verify_only": strconv.FormatBool(req.VerifyOnly),
		},
	})
	replyText := fmt.Sprintf("%s：job=%s repo=%s。可通过 /api/runtime/model-jobs 或 `labelctl runtime model-jobs` 查看状态。", opts.ReplyLabel, job.ID, req.RepoID)
	return ToolExecutionResult{
		ReplyText: replyText,
		Status:    "queued",
		Metadata: map[string]string{
			"job_id":         job.ID,
			"repo_id":        req.RepoID,
			"local_dir":      req.LocalDir,
			"manifest":       req.Manifest,
			"execution_path": "python-worker",
			"dry_run":        strconv.FormatBool(opts.DryRun),
			"verify_only":    strconv.FormatBool(req.VerifyOnly),
		},
	}, nil
}

func (e *GoToolExecutor) verifyHFModel(ctx context.Context, call ToolCall) (ToolExecutionResult, error) {
	if strings.EqualFold(strings.TrimSpace(call.Params["job"]), "true") || hfVerifyRunnerModeFromEnv() == "python-worker" {
		return e.enqueueHFVerifyWorkerJob(call)
	}
	return e.runHFModelTool(ctx, call, true)
}

type locateAnythingSmokeRequest struct {
	ModelDir string
	DataRoot string
	Output   string
}

type genericWorkerJobOptions struct {
	Kind          string
	ReplyLabel    string
	QueuedMessage string
	Resumable     bool
	Metadata      map[string]string
}

func prepareLocateAnythingSmokeRequest(call ToolCall) (locateAnythingSmokeRequest, error) {
	req, err := modelruntime.PrepareLocateAnythingSmokeRequest(call.Params)
	if err != nil {
		return locateAnythingSmokeRequest{}, err
	}
	return locateAnythingSmokeRequest{ModelDir: req.ModelDir, DataRoot: req.DataRoot, Output: req.Output}, nil
}

type trainingRunRequest struct {
	DatasetID   string
	TargetTask  string
	ModelFamily string
	DryRun      bool
	Recipe      string
	Params      map[string]string
}

type evaluationRunRequest struct {
	DatasetID string
	ModelID   string
	Split     string
	DryRun    bool
	Recipe    string
	Params    map[string]string
}

type deploymentRunRequest struct {
	ModelID  string
	Target   string
	Runtime  string
	Replicas string
	DryRun   bool
	Recipe   string
	Params   map[string]string
}

func prepareTrainingRunRequest(call ToolCall) (trainingRunRequest, error) {
	datasetID := strings.TrimSpace(call.Params["dataset_id"])
	if datasetID == "" {
		return trainingRunRequest{}, fmt.Errorf("dataset_id is required")
	}
	targetTask := strings.TrimSpace(call.Params["target_task"])
	if targetTask == "" {
		targetTask = "detection"
	}
	modelFamily := strings.TrimSpace(call.Params["model_family"])
	if modelFamily == "" {
		modelFamily = "yolo11n"
	}
	dryRun := !strings.EqualFold(strings.TrimSpace(call.Params["dry_run"]), "false")
	recipe := strings.TrimSpace(call.Params["execution_recipe"])
	if recipe == "" && !dryRun {
		recipe = "default"
	}
	params := map[string]string{
		"dataset_id":   datasetID,
		"target_task":  targetTask,
		"model_family": modelFamily,
		"dry_run":      strconv.FormatBool(dryRun),
	}
	if recipe != "" {
		params["execution_recipe"] = recipe
	}
	for _, key := range []string{"annotation_version", "split_config", "output_registry"} {
		if value := strings.TrimSpace(call.Params[key]); value != "" {
			params[key] = value
		}
	}
	requestBody := map[string]any{
		"dataset_id":   datasetID,
		"target_task":  targetTask,
		"model_family": modelFamily,
		"dry_run":      dryRun,
	}
	if recipe != "" {
		requestBody["execution_recipe"] = recipe
	}
	for _, key := range []string{"annotation_version", "split_config", "output_registry"} {
		if value := strings.TrimSpace(call.Params[key]); value != "" {
			requestBody[key] = value
		}
	}
	raw, err := json.Marshal(requestBody)
	if err != nil {
		return trainingRunRequest{}, fmt.Errorf("marshal training request: %w", err)
	}
	params["request_json"] = string(raw)
	return trainingRunRequest{DatasetID: datasetID, TargetTask: targetTask, ModelFamily: modelFamily, DryRun: dryRun, Recipe: recipe, Params: params}, nil
}

func prepareEvaluationRunRequest(call ToolCall) (evaluationRunRequest, error) {
	datasetID := strings.TrimSpace(call.Params["dataset_id"])
	if datasetID == "" {
		return evaluationRunRequest{}, fmt.Errorf("dataset_id is required")
	}
	modelID := strings.TrimSpace(call.Params["model_id"])
	if modelID == "" {
		return evaluationRunRequest{}, fmt.Errorf("model_id is required")
	}
	split := strings.TrimSpace(call.Params["split"])
	if split == "" {
		split = "validation"
	}
	dryRun := !strings.EqualFold(strings.TrimSpace(call.Params["dry_run"]), "false")
	recipe := strings.TrimSpace(call.Params["execution_recipe"])
	if recipe == "" && !dryRun {
		recipe = "default"
	}
	params := map[string]string{
		"dataset_id": datasetID,
		"model_id":   modelID,
		"split":      split,
		"dry_run":    strconv.FormatBool(dryRun),
	}
	if recipe != "" {
		params["execution_recipe"] = recipe
	}
	for _, key := range []string{"metrics", "save_visuals", "failure_mining"} {
		if value := strings.TrimSpace(call.Params[key]); value != "" {
			params[key] = value
		}
	}
	requestBody := map[string]any{
		"dataset_id": datasetID,
		"model_id":   modelID,
		"split":      split,
		"dry_run":    dryRun,
	}
	if recipe != "" {
		requestBody["execution_recipe"] = recipe
	}
	if rawMetrics := strings.TrimSpace(call.Params["metrics"]); rawMetrics != "" {
		requestBody["metrics"] = compactCSV(rawMetrics)
	}
	if value := strings.TrimSpace(call.Params["save_visuals"]); value != "" {
		requestBody["save_visuals"] = strings.EqualFold(value, "true")
	}
	if value := strings.TrimSpace(call.Params["failure_mining"]); value != "" {
		requestBody["failure_mining"] = strings.EqualFold(value, "true")
	}
	raw, err := json.Marshal(requestBody)
	if err != nil {
		return evaluationRunRequest{}, fmt.Errorf("marshal evaluation request: %w", err)
	}
	params["request_json"] = string(raw)
	return evaluationRunRequest{DatasetID: datasetID, ModelID: modelID, Split: split, DryRun: dryRun, Recipe: recipe, Params: params}, nil
}

func prepareDeploymentRunRequest(call ToolCall) (deploymentRunRequest, error) {
	modelID := strings.TrimSpace(call.Params["model_id"])
	if modelID == "" {
		return deploymentRunRequest{}, fmt.Errorf("model_id is required")
	}
	target := strings.TrimSpace(call.Params["target"])
	if target == "" {
		return deploymentRunRequest{}, fmt.Errorf("target is required")
	}
	runtime := strings.TrimSpace(call.Params["runtime"])
	if runtime == "" {
		runtime = "python-worker"
	}
	replicas := strings.TrimSpace(call.Params["replicas"])
	if replicas == "" {
		replicas = "1"
	}
	if _, err := strconv.Atoi(replicas); err != nil {
		return deploymentRunRequest{}, fmt.Errorf("replicas must be an integer")
	}
	dryRun := !strings.EqualFold(strings.TrimSpace(call.Params["dry_run"]), "false")
	recipe := strings.TrimSpace(call.Params["execution_recipe"])
	if recipe == "" && !dryRun {
		recipe = "default"
	}
	params := map[string]string{
		"model_id": modelID,
		"target":   target,
		"runtime":  runtime,
		"replicas": replicas,
		"dry_run":  strconv.FormatBool(dryRun),
	}
	if recipe != "" {
		params["execution_recipe"] = recipe
	}
	for _, key := range []string{"model_version", "strategy", "resource_class", "rollback_policy"} {
		if value := strings.TrimSpace(call.Params[key]); value != "" {
			params[key] = value
		}
	}
	requestBody := map[string]any{
		"model_id": modelID,
		"target":   target,
		"runtime":  runtime,
		"dry_run":  dryRun,
	}
	if recipe != "" {
		requestBody["execution_recipe"] = recipe
	}
	if parsed, err := strconv.Atoi(replicas); err == nil {
		requestBody["replicas"] = parsed
	}
	for _, key := range []string{"model_version", "strategy", "resource_class", "rollback_policy"} {
		if value := strings.TrimSpace(call.Params[key]); value != "" {
			requestBody[key] = value
		}
	}
	raw, err := json.Marshal(requestBody)
	if err != nil {
		return deploymentRunRequest{}, fmt.Errorf("marshal deployment request: %w", err)
	}
	params["request_json"] = string(raw)
	return deploymentRunRequest{ModelID: modelID, Target: target, Runtime: runtime, Replicas: replicas, DryRun: dryRun, Recipe: recipe, Params: params}, nil
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
			"job_id":         job.ID,
			"repo_id":        req.RepoID,
			"local_dir":      req.LocalDir,
			"manifest":       req.Manifest,
			"execution_path": "service",
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

func (e *GoToolExecutor) runHFModelWorkerJob(jobID string, req modelruntime.WorkerJobRequest) {
	started := e.now()
	e.modelJobs.Update(jobID, func(job *ModelJob) {
		job.Status = "running"
		job.StartedAt = &started
		job.Message = "running python model worker"
		if req.Action == "verify_hf" {
			job.Message = "running python model verify worker"
		}
		job.ProgressPercent = 15
		job.Metadata = mergeStringMaps(job.Metadata, map[string]string{
			"execution_path": "python-worker",
			"tool_id":        req.ToolID,
			"action":         req.Action,
		})
		job.Logs = appendModelJobLog(job.Logs, started, "info", job.Message)
	})
	ctx, cancel := context.WithTimeout(context.Background(), modelruntime.HFDownloadTimeout())
	e.setModelJobCancel(jobID, cancel)
	defer cancel()
	defer e.clearModelJobCancel(jobID)

	result, err := e.runHFWorkerJob(ctx, req, func(event modelruntime.WorkerRuntimeEvent) {
		e.applyWorkerRuntimeEvent(jobID, event)
	})
	finished := e.now()
	e.modelJobs.Update(jobID, func(job *ModelJob) {
		job.FinishedAt = &finished
		if hb := toModelJobHeartbeat(result.Heartbeat); hb != nil {
			job.WorkerHeartbeat = hb
		}
		if len(result.Artifacts) > 0 {
			job.Artifacts = toModelJobArtifacts(result.Artifacts)
		}
		if strings.TrimSpace(result.Stdout) != "" {
			job.Stdout = result.Stdout
		}
		if strings.TrimSpace(result.Stderr) != "" {
			job.Stderr = result.Stderr
		}
		if result.Attempt > 0 {
			job.Attempt = result.Attempt
		}
		if result.MaxAttempts > 0 {
			job.MaxAttempts = result.MaxAttempts
		}
		job.Retryable = result.Retryable
		appendWorkerLogs(job, result.Logs, finished)
		job.Metadata = mergeStringMaps(job.Metadata, workerMetadata(result))
		if ctx.Err() == context.Canceled || job.CancelRequested {
			job.Status = "canceled"
			job.Message = "python worker canceled"
			job.Resumable = true
			job.Logs = appendModelJobLog(job.Logs, finished, "warn", job.Message)
			return
		}
		if err != nil {
			job.Status = "failed"
			job.Error = err.Error()
			job.Message = firstNonEmpty(result.Message, "python worker execution failed")
			job.ProgressPercent = normalizeProgress(job.ProgressPercent)
			job.Retryable = isPythonWorkerRetryable(err)
			job.Resumable = job.Retryable
			job.Metadata = mergeStringMaps(job.Metadata, pythonWorkerErrorMetadata(err))
			job.Logs = appendModelJobLog(job.Logs, finished, "error", err.Error())
			return
		}
		switch strings.ToLower(strings.TrimSpace(result.Status)) {
		case "completed", "ok", "succeeded":
			job.Status = "succeeded"
			job.Message = firstNonEmpty(result.Message, "python worker completed")
			job.ProgressPercent = 100
			job.Resumable = false
		case "failed":
			job.Status = "failed"
			job.Message = firstNonEmpty(result.Message, "python worker reported failure")
			job.Error = result.Message
			job.ProgressPercent = normalizeProgress(job.ProgressPercent)
			job.Resumable = result.Retryable
		default:
			job.Status = "failed"
			job.Message = firstNonEmpty(result.Message, "python worker returned unknown status")
			job.Error = firstNonEmpty(result.Message, result.Status)
			job.Resumable = result.Retryable
		}
		job.Logs = appendModelJobLog(job.Logs, finished, "info", job.Message)
	})
	e.persistModelJobArtifactManifest(jobID)
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

func (e *GoToolExecutor) LineageModelJob(id string) []ModelJob {
	return e.modelJobs.Lineage(id)
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
	var result ToolExecutionResult
	if hfDownloadRunnerModeFromEnv() == "service" {
		result, err = e.enqueueHFModelDownloadRequest(req, id, call)
	} else {
		result, err = e.enqueueHFModelWorkerJob(req, id, call, hfWorkerJobOptions{
			Kind:          "model.download_hf",
			Action:        "download_hf",
			DryRun:        false,
			Resumable:     true,
			ReplyLabel:    "Python worker 任务已排队",
			QueuedMessage: "queued python worker",
		})
	}
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

func (e *GoToolExecutor) runPythonModelWorkerJob(ctx context.Context, req modelruntime.WorkerJobRequest, emit func(modelruntime.WorkerRuntimeEvent)) (modelruntime.WorkerJobResult, error) {
	return e.workerRunner.Run(ctx, req, emit)
}

func (e *GoToolExecutor) smokeLocateAnythingModel(ctx context.Context, call ToolCall) (ToolExecutionResult, error) {
	if strings.EqualFold(strings.TrimSpace(call.Params["job"]), "true") || locateAnythingSmokeRunnerModeFromEnv() == "python-worker" {
		return e.enqueueLocateAnythingSmokeWorkerJob(call)
	}
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

func (e *GoToolExecutor) enqueueLocateAnythingSmokeWorkerJob(call ToolCall) (ToolExecutionResult, error) {
	req, err := prepareLocateAnythingSmokeRequest(call)
	if err != nil {
		return ToolExecutionResult{}, err
	}
	const modelRepoID = "nvidia/LocateAnything-3B"
	created := e.now()
	job := e.modelJobs.Create(ModelJob{
		ID:              fmt.Sprintf("model-job-%d", created.UnixNano()),
		Kind:            "model.smoke_locateanything",
		RepoID:          modelRepoID,
		Status:          "queued",
		Message:         "queued python worker smoke",
		ProgressPercent: 0,
		Resumable:       false,
		Retryable:       false,
		Logs:            []ModelJobLog{{At: created, Level: "info", Message: "queued python worker smoke"}},
		Metadata: map[string]string{
			"execution_path": "python-worker",
			"model_dir":      req.ModelDir,
			"data_root":      req.DataRoot,
			"output":         req.Output,
		},
	})
	go e.runHFModelWorkerJob(job.ID, modelruntime.WorkerJobRequest{
		TaskID:     job.ID,
		WorkflowID: "runtime-model-worker",
		AgentID:    "model-agent",
		ToolID:     "model.smoke_locateanything",
		Action:     "smoke_locateanything",
		DatasetID:  strings.TrimSpace(call.Params["dataset_id"]),
		Params: map[string]string{
			"model_dir": req.ModelDir,
			"data_root": req.DataRoot,
			"output":    req.Output,
		},
	})
	return ToolExecutionResult{
		ReplyText: fmt.Sprintf("Python worker smoke 任务已排队：job=%s model=%s。可通过 /api/runtime/model-jobs 或 `labelctl runtime model-jobs` 查看状态。", job.ID, modelRepoID),
		Status:    "queued",
		Metadata: map[string]string{
			"job_id":         job.ID,
			"model_dir":      req.ModelDir,
			"data_root":      req.DataRoot,
			"output":         req.Output,
			"execution_path": "python-worker",
		},
	}, nil
}

func (e *GoToolExecutor) enqueueGenericWorkerJob(req modelruntime.WorkerJobRequest, opts genericWorkerJobOptions) (ToolExecutionResult, error) {
	created := e.now()
	if strings.TrimSpace(opts.Kind) == "" {
		opts.Kind = req.ToolID
	}
	if strings.TrimSpace(opts.ReplyLabel) == "" {
		opts.ReplyLabel = "Python worker 任务已排队"
	}
	if strings.TrimSpace(opts.QueuedMessage) == "" {
		opts.QueuedMessage = "queued python worker"
	}
	job := e.modelJobs.Create(ModelJob{
		ID:              fmt.Sprintf("model-job-%d", created.UnixNano()),
		Kind:            opts.Kind,
		Status:          "queued",
		Message:         opts.QueuedMessage,
		ProgressPercent: 0,
		Resumable:       opts.Resumable,
		Retryable:       false,
		Logs:            []ModelJobLog{{At: created, Level: "info", Message: opts.QueuedMessage}},
		Metadata: mergeStringMaps(map[string]string{
			"execution_path": "python-worker",
			"dry_run":        strconv.FormatBool(req.DryRun),
		}, opts.Metadata),
	})
	req.TaskID = job.ID
	go e.runHFModelWorkerJob(job.ID, req)
	return ToolExecutionResult{
		ReplyText: fmt.Sprintf("%s：job=%s。可通过 /api/runtime/model-jobs 或 `labelctl runtime model-jobs` 查看状态。", opts.ReplyLabel, job.ID),
		Status:    "queued",
		Metadata: mergeStringMaps(map[string]string{
			"job_id":         job.ID,
			"execution_path": "python-worker",
			"dry_run":        strconv.FormatBool(req.DryRun),
		}, opts.Metadata),
	}, nil
}

func (e *GoToolExecutor) submitTrainingDryRun(_ context.Context, call ToolCall) (ToolExecutionResult, error) {
	req, err := prepareTrainingRunRequest(call)
	if err != nil {
		return ToolExecutionResult{}, err
	}
	replyLabel := fmt.Sprintf("Python worker 训练 dry-run 已排队：dataset=%s target=%s model=%s", req.DatasetID, req.TargetTask, req.ModelFamily)
	queuedMessage := "queued python training dry-run"
	if !req.DryRun {
		replyLabel = fmt.Sprintf("Python worker 训练执行已排队：dataset=%s target=%s model=%s recipe=%s", req.DatasetID, req.TargetTask, req.ModelFamily, valueOrString(req.Recipe, "default"))
		queuedMessage = "queued python training execution"
	}
	return e.enqueueGenericWorkerJob(modelruntime.WorkerJobRequest{
		WorkflowID: "data-to-deployment-lifecycle",
		AgentID:    "training-agent",
		ToolID:     "training.run",
		Action:     "training.run",
		DatasetID:  req.DatasetID,
		DryRun:     req.DryRun,
		Params:     req.Params,
	}, genericWorkerJobOptions{
		Kind:          "training.run",
		ReplyLabel:    replyLabel,
		QueuedMessage: queuedMessage,
		Resumable:     false,
		Metadata: map[string]string{
			"dataset_id":       req.DatasetID,
			"target_task":      req.TargetTask,
			"model_family":     req.ModelFamily,
			"execution_recipe": req.Recipe,
		},
	})
}

func (e *GoToolExecutor) submitEvaluationDryRun(_ context.Context, call ToolCall) (ToolExecutionResult, error) {
	req, err := prepareEvaluationRunRequest(call)
	if err != nil {
		return ToolExecutionResult{}, err
	}
	replyLabel := fmt.Sprintf("Python worker 评估 dry-run 已排队：dataset=%s model=%s split=%s", req.DatasetID, req.ModelID, req.Split)
	queuedMessage := "queued python evaluation dry-run"
	if !req.DryRun {
		replyLabel = fmt.Sprintf("Python worker 评估执行已排队：dataset=%s model=%s split=%s recipe=%s", req.DatasetID, req.ModelID, req.Split, valueOrString(req.Recipe, "default"))
		queuedMessage = "queued python evaluation execution"
	}
	return e.enqueueGenericWorkerJob(modelruntime.WorkerJobRequest{
		WorkflowID: "data-to-deployment-lifecycle",
		AgentID:    "training-agent",
		ToolID:     "evaluation.run",
		Action:     "evaluation.run",
		DatasetID:  req.DatasetID,
		DryRun:     req.DryRun,
		Params:     req.Params,
	}, genericWorkerJobOptions{
		Kind:          "evaluation.run",
		ReplyLabel:    replyLabel,
		QueuedMessage: queuedMessage,
		Resumable:     false,
		Metadata: map[string]string{
			"dataset_id":       req.DatasetID,
			"model_id":         req.ModelID,
			"split":            req.Split,
			"execution_recipe": req.Recipe,
		},
	})
}

func (e *GoToolExecutor) submitDeploymentDryRun(_ context.Context, call ToolCall) (ToolExecutionResult, error) {
	req, err := prepareDeploymentRunRequest(call)
	if err != nil {
		return ToolExecutionResult{}, err
	}
	replyLabel := fmt.Sprintf("Python worker 部署 dry-run 已排队：model=%s target=%s runtime=%s replicas=%s", req.ModelID, req.Target, req.Runtime, req.Replicas)
	queuedMessage := "queued python deployment dry-run"
	if !req.DryRun {
		replyLabel = fmt.Sprintf("Python worker 部署执行已排队：model=%s target=%s runtime=%s replicas=%s recipe=%s", req.ModelID, req.Target, req.Runtime, req.Replicas, valueOrString(req.Recipe, "default"))
		queuedMessage = "queued python deployment execution"
	}
	return e.enqueueGenericWorkerJob(modelruntime.WorkerJobRequest{
		WorkflowID: "data-to-deployment-lifecycle",
		AgentID:    "training-agent",
		ToolID:     "deployment.run",
		Action:     "deployment.run",
		DryRun:     req.DryRun,
		Params:     req.Params,
	}, genericWorkerJobOptions{
		Kind:          "deployment.run",
		ReplyLabel:    replyLabel,
		QueuedMessage: queuedMessage,
		Resumable:     false,
		Metadata: map[string]string{
			"model_id":         req.ModelID,
			"target":           req.Target,
			"runtime":          req.Runtime,
			"replicas":         req.Replicas,
			"execution_recipe": req.Recipe,
		},
	})
}

func compactCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func appendWorkerLogs(job *ModelJob, logs []modelruntime.WorkerLog, fallback time.Time) {
	for _, log := range logs {
		at := fallback
		if parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(log.At)); err == nil {
			at = parsed
		}
		job.Logs = appendModelJobLog(job.Logs, at, log.Level, log.Message)
	}
}

func workerMetadata(result modelruntime.WorkerJobResult) map[string]string {
	metadata := map[string]string{}
	if strings.TrimSpace(result.StartedAt) != "" {
		metadata["worker_started_at"] = result.StartedAt
	}
	if strings.TrimSpace(result.FinishedAt) != "" {
		metadata["worker_finished_at"] = result.FinishedAt
	}
	if result.Attempt > 0 {
		metadata["worker_attempt"] = strconv.Itoa(result.Attempt)
	}
	if result.MaxAttempts > 0 {
		metadata["worker_max_attempts"] = strconv.Itoa(result.MaxAttempts)
	}
	if result.Heartbeat != nil && strings.TrimSpace(result.Heartbeat.Status) != "" {
		metadata["worker_heartbeat_status"] = result.Heartbeat.Status
	}
	if len(result.Artifacts) > 0 {
		metadata["artifact_count"] = strconv.Itoa(len(result.Artifacts))
	}
	if len(metadata) == 0 {
		return nil
	}
	return metadata
}

func toModelJobHeartbeat(value *modelruntime.WorkerHeartbeat) *ModelJobHeartbeat {
	if value == nil {
		return nil
	}
	return &ModelJobHeartbeat{At: value.At, Status: value.Status, Message: value.Message}
}

func toModelJobArtifacts(items []modelruntime.WorkerArtifact) []ModelJobArtifact {
	if len(items) == 0 {
		return nil
	}
	out := make([]ModelJobArtifact, 0, len(items))
	for _, item := range items {
		out = append(out, ModelJobArtifact{
			Name:     item.Name,
			URI:      item.URI,
			Kind:     item.Kind,
			Metadata: item.Metadata,
		})
	}
	return out
}

func mergeStringMaps(base map[string]string, overlay map[string]string) map[string]string {
	if len(base) == 0 && len(overlay) == 0 {
		return nil
	}
	out := map[string]string{}
	for key, value := range base {
		out[key] = value
	}
	for key, value := range overlay {
		if strings.TrimSpace(value) != "" {
			out[key] = value
		}
	}
	return out
}

func (e *GoToolExecutor) applyWorkerRuntimeEvent(jobID string, event modelruntime.WorkerRuntimeEvent) {
	now := e.now()
	e.modelJobs.Update(jobID, func(job *ModelJob) {
		switch strings.ToLower(strings.TrimSpace(event.Type)) {
		case "heartbeat":
			job.WorkerHeartbeat = &ModelJobHeartbeat{
				At:      firstNonEmpty(event.At, now.Format(time.RFC3339Nano)),
				Status:  firstNonEmpty(event.Status, "running"),
				Message: strings.TrimSpace(event.Message),
			}
			job.Metadata = mergeStringMaps(job.Metadata, map[string]string{
				"worker_heartbeat_status": firstNonEmpty(event.Status, "running"),
			})
			job.Logs = appendModelJobLog(job.Logs, parseWorkerEventTime(event.At, now), "info", "worker heartbeat: "+firstNonEmpty(event.Status, "running")+" "+strings.TrimSpace(event.Message))
		case "log":
			level := firstNonEmpty(strings.TrimSpace(event.Level), "info")
			job.Logs = appendModelJobLog(job.Logs, parseWorkerEventTime(event.At, now), level, strings.TrimSpace(event.Message))
		case "stream":
			stream := strings.ToLower(strings.TrimSpace(event.Stream))
			text := strings.TrimSpace(event.Text)
			if text == "" {
				return
			}
			switch stream {
			case "stdout":
				job.Stdout = appendModelJobOutput(job.Stdout, text)
				job.Logs = appendModelJobLog(job.Logs, parseWorkerEventTime(event.At, now), "info", "stdout> "+text)
			case "stderr":
				job.Stderr = appendModelJobOutput(job.Stderr, text)
				job.Logs = appendModelJobLog(job.Logs, parseWorkerEventTime(event.At, now), "warn", "stderr> "+text)
			default:
				job.Logs = appendModelJobLog(job.Logs, parseWorkerEventTime(event.At, now), "info", text)
			}
		}
	})
}

func parseWorkerEventTime(value string, fallback time.Time) time.Time {
	if parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value)); err == nil {
		return parsed
	}
	return fallback
}

func appendModelJobOutput(current string, line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return current
	}
	if strings.TrimSpace(current) == "" {
		return truncateModelJobOutput(line)
	}
	return truncateModelJobOutput(current + "\n" + line)
}

func truncateModelJobOutput(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\r\n", "\n"))
	const limit = 64 * 1024
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "\n...[truncated]"
}

func isPythonWorkerRetryable(err error) bool {
	if err == nil {
		return false
	}
	type retryableWorkerError interface {
		WorkerRetryable() bool
	}
	var typed retryableWorkerError
	if errors.As(err, &typed) {
		return typed.WorkerRetryable()
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "timed out") || strings.Contains(lower, "context deadline exceeded")
}

func pythonWorkerErrorMetadata(err error) map[string]string {
	if err == nil {
		return nil
	}
	type typedWorkerError interface {
		WorkerKind() string
		WorkerRetryable() bool
	}
	var typed typedWorkerError
	if !errors.As(err, &typed) {
		return nil
	}
	return map[string]string{
		"worker_error_kind":      typed.WorkerKind(),
		"worker_error_retryable": strconv.FormatBool(typed.WorkerRetryable()),
	}
}

func (e *GoToolExecutor) persistModelJobArtifactManifest(jobID string) {
	writer, ok := e.modelJobs.(ModelJobArtifactManifestWriter)
	if !ok {
		return
	}
	job, ok := e.modelJobs.Get(jobID)
	if !ok {
		return
	}
	path, err := writer.WriteArtifactManifest(job)
	if err != nil || strings.TrimSpace(path) == "" {
		if err != nil {
			now := e.now()
			e.modelJobs.Update(jobID, func(job *ModelJob) {
				job.Logs = appendModelJobLog(job.Logs, now, "warn", "artifact manifest persist failed: "+err.Error())
			})
		}
		return
	}
	e.modelJobs.Update(jobID, func(job *ModelJob) {
		job.Metadata = mergeStringMaps(job.Metadata, map[string]string{
			"artifact_manifest": path,
		})
	})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
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
