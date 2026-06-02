package agentruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/domain/agent"
	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

type GoToolExecutor struct {
	agents         AgentControlPlane
	now            func() time.Time
	modelJobs      ModelJobStore
	runHFModelTool func(context.Context, ToolCall, bool) (ToolExecutionResult, error)
}

func NewGoToolExecutor(agents AgentControlPlane, now func() time.Time) *GoToolExecutor {
	if now == nil {
		now = time.Now
	}
	executor := &GoToolExecutor{
		agents:    agents,
		now:       now,
		modelJobs: NewModelJobStore(now),
	}
	executor.runHFModelTool = executor.runHFModelScript
	return executor
}

func NewGoToolExecutorWithModelJobs(agents AgentControlPlane, now func() time.Time, modelJobs ModelJobStore) *GoToolExecutor {
	executor := NewGoToolExecutor(agents, now)
	if modelJobs != nil {
		executor.modelJobs = modelJobs
	}
	return executor
}

func (e *GoToolExecutor) Execute(ctx context.Context, req ToolExecutionRequest) (ToolExecutionResult, error) {
	if len(req.ToolCalls) == 0 {
		return ToolExecutionResult{}, nil
	}
	results := make([]string, 0, len(req.ToolCalls))
	status := "ok"
	metadata := map[string]string{}
	for _, call := range req.ToolCalls {
		result, err := e.executeOne(ctx, req, call)
		if err != nil {
			return ToolExecutionResult{}, err
		}
		if result.Status != "" {
			status = result.Status
		}
		if result.ReplyText != "" {
			results = append(results, result.ReplyText)
		}
		for key, value := range result.Metadata {
			metadata[key] = value
		}
	}
	if len(metadata) == 0 {
		metadata = nil
	}
	return ToolExecutionResult{ReplyText: strings.Join(results, "\n"), Status: status, Metadata: metadata}, nil
}

func (e *GoToolExecutor) executeOne(ctx context.Context, req ToolExecutionRequest, call ToolCall) (ToolExecutionResult, error) {
	switch call.ToolID {
	case "runtime.health":
		return ToolExecutionResult{ReplyText: "pong", Status: "ok"}, nil
	case "runtime.identify_actor":
		return ToolExecutionResult{
			ReplyText: fmt.Sprintf("channel=%s account=%s peer=%s:%s sender=%s", req.Message.Channel, req.Message.AccountID, req.Message.Peer.Kind, req.Message.Peer.ID, req.Message.SenderID),
			Status:    "ok",
		}, nil
	case "runtime.status":
		return ToolExecutionResult{
			ReplyText: fmt.Sprintf("Agent Gateway online. channel=%s account=%s runtime=ready agent_loop=python-agent-runtime text_model=mimo-v2.5-pro vision_model=mimo-v2.5 time=%s", req.Message.Channel, req.Message.AccountID, e.now().Format(time.RFC3339)),
			Status:    "ok",
		}, nil
	case "workflow.list_runs":
		return e.listRuns(ctx)
	case "workflow.submit_run":
		return e.submitWorkflowRun(ctx, req, call)
	case "intake.quarantine":
		return ToolExecutionResult{
			ReplyText: fmt.Sprintf("已生成数据接入隔离计划：attachments=%d session=%s", len(req.Message.Attachments), req.Session.Key),
			Status:    "planned",
		}, nil
	case "intake.plan":
		return e.planDataIntake(req, call)
	case "vlm.inspect":
		return e.planVisionInspection(req, call)
	case "llm.plan":
		return ToolExecutionResult{
			ReplyText: fmt.Sprintf("已进入 planner-agent 会话：%s；可通过 AGENT_RUNTIME_PLANNER=python 启用 Python/Mimo planner。", req.Session.Key),
			Status:    "planned",
		}, nil
	case "model.download_hf":
		return e.downloadHFModel(ctx, call)
	case "model.verify_hf":
		return e.verifyHFModel(ctx, call)
	case "model.smoke_locateanything":
		return e.smokeLocateAnythingModel(ctx, call)
	default:
		return ToolExecutionResult{
			ReplyText: fmt.Sprintf("工具 %s 尚未接入。", call.ToolID),
			Status:    "unsupported_tool",
		}, nil
	}
}

func (e *GoToolExecutor) planDataIntake(req ToolExecutionRequest, call ToolCall) (ToolExecutionResult, error) {
	plan := channel.DataIntakePlan{
		ID:              fmt.Sprintf("intake-plan-%d", e.now().UnixNano()),
		SourceMessageID: req.Message.ID,
		Channel:         req.Message.Channel,
		AccountID:       req.Message.AccountID,
		SenderID:        req.Message.SenderID,
		Intent:          channel.IntakeIntentInspect,
		DatasetName:     inferDatasetName(req, call),
		ProposedActions: []channel.PlannedAction{
			{Kind: "quarantine", Params: map[string]string{"attachment_count": strconv.Itoa(len(req.Message.Attachments))}},
			{Kind: "scan", Params: map[string]string{"scanner": "mvp-static-preflight"}},
			{Kind: "create_data_intake_plan", Params: map[string]string{"mode": "dry_run"}},
		},
		RequiredApprovals: []string{"human_review_before_data_lake_write"},
		RiskLevel:         intakeRisk(req.Message.Attachments),
		DryRun:            true,
		CreatedAt:         e.now(),
	}
	planJSON, err := json.Marshal(plan)
	if err != nil {
		return ToolExecutionResult{}, err
	}
	metadata := map[string]string{
		"plan_id":          plan.ID,
		"dataset_name":     plan.DatasetName,
		"attachment_count": strconv.Itoa(len(req.Message.Attachments)),
		"risk_level":       plan.RiskLevel,
		"dry_run":          strconv.FormatBool(plan.DryRun),
		"approval":         strings.Join(plan.RequiredApprovals, ","),
		"plan_json":        string(planJSON),
	}
	if source := firstAttachmentSource(req.Message.Attachments); source != "" {
		metadata["source_uri"] = source
	}
	return ToolExecutionResult{
		ReplyText: fmt.Sprintf("已生成 Data Intake Plan：plan=%s agent=%s dataset=%s attachments=%d risk=%s dry_run=true；正式入湖前需要人工审批。", plan.ID, req.Delegation.AgentID, plan.DatasetName, len(req.Message.Attachments), plan.RiskLevel),
		Status:    "planned",
		Metadata:  metadata,
	}, nil
}

func (e *GoToolExecutor) planVisionInspection(req ToolExecutionRequest, call ToolCall) (ToolExecutionResult, error) {
	plan := channel.DataIntakePlan{
		ID:              fmt.Sprintf("vision-plan-%d", e.now().UnixNano()),
		SourceMessageID: req.Message.ID,
		Channel:         req.Message.Channel,
		AccountID:       req.Message.AccountID,
		SenderID:        req.Message.SenderID,
		Intent:          channel.IntakeIntentInspect,
		DatasetName:     inferDatasetName(req, call),
		ProposedActions: []channel.PlannedAction{
			{Kind: "quarantine", Params: map[string]string{"attachment_count": strconv.Itoa(len(req.Message.Attachments))}},
			{Kind: "vlm_inspect", Params: map[string]string{"model_route": "vision", "model": "mimo-v2.5"}},
			{Kind: "create_data_intake_plan", Params: map[string]string{"mode": "dry_run"}},
		},
		RequiredApprovals: []string{"human_review_before_data_lake_write"},
		RiskLevel:         intakeRisk(req.Message.Attachments),
		DryRun:            true,
		CreatedAt:         e.now(),
	}
	planJSON, err := json.Marshal(plan)
	if err != nil {
		return ToolExecutionResult{}, err
	}
	metadata := map[string]string{
		"plan_id":          plan.ID,
		"dataset_name":     plan.DatasetName,
		"attachment_count": strconv.Itoa(len(req.Message.Attachments)),
		"risk_level":       plan.RiskLevel,
		"dry_run":          strconv.FormatBool(plan.DryRun),
		"model_route":      "vision",
		"model":            "mimo-v2.5",
		"approval":         strings.Join(plan.RequiredApprovals, ","),
		"plan_json":        string(planJSON),
	}
	if source := firstAttachmentSource(req.Message.Attachments); source != "" {
		metadata["source_uri"] = source
	}
	return ToolExecutionResult{
		ReplyText: fmt.Sprintf("已生成视觉数据检查计划：plan=%s agent=vision-agent route=mimo-v2.5 attachments=%d；当前 MVP 只做计划和审批边界，不自动写入 Data Lake。", plan.ID, len(req.Message.Attachments)),
		Status:    "planned",
		Metadata:  metadata,
	}, nil
}

func inferDatasetName(req ToolExecutionRequest, call ToolCall) string {
	if value := strings.TrimSpace(call.Params["dataset_id"]); value != "" {
		return value
	}
	if value := strings.TrimSpace(req.Intent.DatasetID); value != "" {
		return value
	}
	text := strings.ToLower(req.Message.Text)
	for _, attachment := range req.Message.Attachments {
		text += " " + strings.ToLower(attachment.Name) + " " + strings.ToLower(attachment.SourceURI) + " " + strings.ToLower(attachment.LocalURI)
	}
	if strings.Contains(text, "shanghaitech") || strings.Contains(text, "上海") {
		return "shanghaitech-original"
	}
	return "channel-upload-draft"
}

func intakeRisk(attachments []channel.Attachment) string {
	for _, attachment := range attachments {
		mediaType := strings.ToLower(strings.TrimSpace(attachment.MediaType))
		name := strings.ToLower(strings.TrimSpace(attachment.Name))
		if strings.Contains(mediaType, "zip") || strings.HasSuffix(name, ".zip") || strings.HasSuffix(name, ".7z") || strings.HasSuffix(name, ".rar") {
			return "medium"
		}
	}
	return "low"
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
	if modelDownloadRequiresApproval(call) {
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

type locateAnythingSmokeRequest struct {
	ModelDir string
	DataRoot string
	Output   string
}

func prepareHFModelRequest(call ToolCall, verifyOnly bool) (hfModelRequest, error) {
	repoID := strings.TrimSpace(call.Params["repo_id"])
	if repoID == "" {
		repoID = "nvidia/LocateAnything-3B"
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`).MatchString(repoID) {
		return hfModelRequest{}, fmt.Errorf("invalid HuggingFace repo_id: %s", repoID)
	}
	localDir := strings.TrimSpace(call.Params["local_dir"])
	if localDir == "" {
		localDir = filepath.Join("data_lake", "models", "artifacts", "huggingface", strings.ReplaceAll(repoID, "/", string(filepath.Separator)))
	}
	manifest := strings.TrimSpace(call.Params["manifest"])
	if manifest == "" {
		manifest = filepath.Join("data_lake", "catalog", "models", strings.ReplaceAll(repoID, "/", "_")+".download.json")
	}
	if strings.EqualFold(call.Params["verify_only"], "true") {
		verifyOnly = true
	}
	safeLocalDir, err := safeRepoPath(localDir, filepath.Join("data_lake", "models", "artifacts", "huggingface"))
	if err != nil {
		return hfModelRequest{}, err
	}
	safeManifest, err := safeRepoPath(manifest, filepath.Join("data_lake", "catalog", "models"))
	if err != nil {
		return hfModelRequest{}, err
	}
	return hfModelRequest{RepoID: repoID, LocalDir: safeLocalDir, Manifest: safeManifest, VerifyOnly: verifyOnly}, nil
}

func (e *GoToolExecutor) enqueueHFModelDownload(call ToolCall) (ToolExecutionResult, error) {
	req, err := prepareHFModelRequest(call, false)
	if err != nil {
		return ToolExecutionResult{}, err
	}
	job := e.modelJobs.Create(ModelJob{
		ID:       fmt.Sprintf("model-job-%d", e.now().UnixNano()),
		Kind:     "model.download_hf",
		RepoID:   req.RepoID,
		LocalDir: req.LocalDir,
		Manifest: req.Manifest,
		Status:   "queued",
		Message:  "queued by Agent Runtime",
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
	})
	ctx, cancel := context.WithTimeout(context.Background(), hfDownloadTimeout())
	defer cancel()
	result, err := e.runHFModelTool(ctx, call, false)
	finished := e.now()
	e.modelJobs.Update(jobID, func(job *ModelJob) {
		job.FinishedAt = &finished
		if err != nil {
			job.Status = "failed"
			job.Error = err.Error()
			job.Message = "download failed"
			return
		}
		job.Status = "succeeded"
		job.Message = result.ReplyText
		job.Metadata = result.Metadata
	})
}

func (e *GoToolExecutor) ListModelJobs(limit int) []ModelJob {
	return e.modelJobs.List(limit)
}

func (e *GoToolExecutor) runHFModelScript(ctx context.Context, call ToolCall, verifyOnly bool) (ToolExecutionResult, error) {
	req, err := prepareHFModelRequest(call, verifyOnly)
	if err != nil {
		return ToolExecutionResult{}, err
	}
	timeout := hfDownloadTimeout()
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	python := strings.TrimSpace(os.Getenv("AGENT_RUNTIME_PYTHON"))
	if python == "" {
		python = "python"
	}
	args := []string{
		filepath.Join("skills", "huggingface-model-downloader", "scripts", "download_hf_snapshot.py"),
		"--repo-id", req.RepoID,
		"--local-dir", req.LocalDir,
		"--manifest", req.Manifest,
	}
	if req.VerifyOnly {
		args = append(args, "--verify-only")
	}
	cmd := exec.CommandContext(runCtx, python, args...)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if runCtx.Err() == context.DeadlineExceeded {
		return ToolExecutionResult{}, fmt.Errorf("HuggingFace model tool timed out after %s", timeout)
	}
	if err != nil {
		return ToolExecutionResult{}, fmt.Errorf("HuggingFace model tool failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	action := "下载"
	if req.VerifyOnly {
		action = "校验"
	}
	return ToolExecutionResult{
		ReplyText: fmt.Sprintf("HuggingFace 模型%s完成：repo=%s local_dir=%s manifest=%s", action, req.RepoID, req.LocalDir, req.Manifest),
		Status:    "ok",
		Metadata: map[string]string{
			"repo_id":   req.RepoID,
			"local_dir": req.LocalDir,
			"manifest":  req.Manifest,
		},
	}, nil
}

func prepareLocateAnythingSmokeRequest(call ToolCall) (locateAnythingSmokeRequest, error) {
	modelDir := strings.TrimSpace(call.Params["model_dir"])
	if modelDir == "" {
		modelDir = filepath.Join("data_lake", "models", "artifacts", "huggingface", "nvidia", "LocateAnything-3B")
	}
	dataRoot := strings.TrimSpace(call.Params["data_root"])
	if dataRoot == "" {
		dataRoot = filepath.Join("data_lake", "raw", "datasets", "shanghaitech", "original")
	}
	output := strings.TrimSpace(call.Params["output"])
	if output == "" {
		output = filepath.Join("data_lake", "catalog", "models", "nvidia_LocateAnything-3B.smoke.json")
	}
	safeModelDir, err := safeRepoPath(modelDir, filepath.Join("data_lake", "models", "artifacts", "huggingface"))
	if err != nil {
		return locateAnythingSmokeRequest{}, err
	}
	safeDataRoot, err := safeRepoPath(dataRoot, filepath.Join("data_lake", "raw", "datasets"))
	if err != nil {
		return locateAnythingSmokeRequest{}, err
	}
	safeOutput, err := safeRepoPath(output, filepath.Join("data_lake", "catalog", "models"))
	if err != nil {
		return locateAnythingSmokeRequest{}, err
	}
	return locateAnythingSmokeRequest{ModelDir: safeModelDir, DataRoot: safeDataRoot, Output: safeOutput}, nil
}

func (e *GoToolExecutor) smokeLocateAnythingModel(ctx context.Context, call ToolCall) (ToolExecutionResult, error) {
	req, err := prepareLocateAnythingSmokeRequest(call)
	if err != nil {
		return ToolExecutionResult{}, err
	}
	timeout := hfDownloadTimeout()
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	python := strings.TrimSpace(os.Getenv("AGENT_RUNTIME_PYTHON"))
	if python == "" {
		python = "python"
	}
	args := []string{
		filepath.Join("workers", "python", "agent_worker", "locateanything_smoke.py"),
		"--model-dir", req.ModelDir,
		"--data-root", req.DataRoot,
		"--output", req.Output,
	}
	cmd := exec.CommandContext(runCtx, python, args...)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if runCtx.Err() == context.DeadlineExceeded {
		return ToolExecutionResult{}, fmt.Errorf("LocateAnything smoke timed out after %s", timeout)
	}
	if err != nil {
		return ToolExecutionResult{}, fmt.Errorf("LocateAnything smoke failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	status, modelLoad, realInference := parseLocateAnythingSmokeOutput(out)
	return ToolExecutionResult{
		ReplyText: fmt.Sprintf("LocateAnything-3B 可用性 smoke 完成：status=%s model_load=%s real_inference=%s report=%s", status, modelLoad, realInference, req.Output),
		Status:    status,
		Metadata: map[string]string{
			"model_id":       "nvidia/LocateAnything-3B",
			"model_dir":      req.ModelDir,
			"data_root":      req.DataRoot,
			"smoke_report":   req.Output,
			"model_load":     modelLoad,
			"real_inference": realInference,
		},
	}, nil
}

func parseLocateAnythingSmokeOutput(out []byte) (string, string, string) {
	var payload struct {
		Status    string `json:"status"`
		Completed struct {
			ModelLoad     bool `json:"model_load"`
			RealInference bool `json:"real_inference"`
		} `json:"completed"`
	}
	status := "ok"
	modelLoad := "unknown"
	realInference := "unknown"
	raw := strings.TrimSpace(string(out))
	if start := strings.Index(raw, "{"); start >= 0 {
		if end := strings.LastIndex(raw, "}"); end >= start {
			raw = raw[start : end+1]
		}
	}
	if err := json.Unmarshal([]byte(raw), &payload); err == nil {
		if strings.TrimSpace(payload.Status) != "" {
			status = payload.Status
		}
		modelLoad = strconv.FormatBool(payload.Completed.ModelLoad)
		realInference = strconv.FormatBool(payload.Completed.RealInference)
	}
	if status == "partial" {
		status = "ok"
	}
	return status, modelLoad, realInference
}

func safeRepoPath(path string, allowedRoot string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	root, err := filepath.Abs(allowedRoot)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", err
	}
	if rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path %s must stay under %s", abs, root)
	}
	return abs, nil
}

func hfDownloadTimeout() time.Duration {
	raw := strings.TrimSpace(os.Getenv("AGENT_RUNTIME_HF_DOWNLOAD_TIMEOUT_MINUTES"))
	if raw == "" {
		return 360 * time.Minute
	}
	minutes, err := strconv.Atoi(raw)
	if err != nil || minutes <= 0 {
		return 360 * time.Minute
	}
	return time.Duration(minutes) * time.Minute
}

func modelDownloadRequiresApproval(call ToolCall) bool {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("AGENT_RUNTIME_REQUIRE_MODEL_DOWNLOAD_APPROVAL")), "true") {
		return false
	}
	return !strings.EqualFold(strings.TrimSpace(call.Params["approved"]), "true")
}

func (e *GoToolExecutor) listRuns(ctx context.Context) (ToolExecutionResult, error) {
	runs, err := e.agents.ListRuns(ctx)
	if err != nil {
		return ToolExecutionResult{}, err
	}
	if len(runs) == 0 {
		return ToolExecutionResult{ReplyText: "暂无 Agent run。", Status: "ok"}, nil
	}
	limit := 5
	if len(runs) < limit {
		limit = len(runs)
	}
	lines := []string{"最近 Agent runs:"}
	for i := 0; i < limit; i++ {
		run := runs[i]
		lines = append(lines, fmt.Sprintf("- %s workflow=%s status=%s task=%s", run.ID, run.WorkflowID, run.Status, run.TaskID))
	}
	return ToolExecutionResult{ReplyText: strings.Join(lines, "\n"), Status: "ok"}, nil
}

func (e *GoToolExecutor) submitWorkflowRun(ctx context.Context, req ToolExecutionRequest, call ToolCall) (ToolExecutionResult, error) {
	if req.Intent.Kind != IntentSubmitDryRun && !strings.EqualFold(call.Params["dry_run"], "true") {
		return ToolExecutionResult{}, fmt.Errorf("workflow.submit_run requires dry_run=true or explicit /bot-run dry intent")
	}
	workflowID := strings.TrimSpace(call.Params["workflow_id"])
	if workflowID == "" {
		workflowID = defaultWorkflowID
	}
	datasetID := "workspace-dataset"
	if req.Intent.DatasetID != "" {
		datasetID = req.Intent.DatasetID
	}
	if value := strings.TrimSpace(call.Params["dataset_id"]); value != "" {
		datasetID = value
	}
	run, err := e.agents.SubmitWorkflowRun(ctx, agent.RunRequest{
		WorkflowID: workflowID,
		DatasetID:  datasetID,
		DryRun:     true,
		Params: map[string]string{
			"source":      string(req.Message.Channel),
			"account_id":  req.Message.AccountID,
			"peer_kind":   string(req.Message.Peer.Kind),
			"peer_id":     req.Message.Peer.ID,
			"sender_id":   req.Message.SenderID,
			"session_key": req.Session.Key,
			"agent_id":    req.Session.AgentID,
		},
	})
	if err != nil {
		return ToolExecutionResult{}, err
	}
	return ToolExecutionResult{
		ReplyText: fmt.Sprintf("已提交 dry-run：run=%s task=%s workflow=%s dataset=%s", run.ID, run.TaskID, run.WorkflowID, run.DatasetID),
		Status:    "ok",
	}, nil
}
