package agentruntime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/domain/agent"
)

type GoToolExecutor struct {
	agents AgentControlPlane
	now    func() time.Time
}

func NewGoToolExecutor(agents AgentControlPlane, now func() time.Time) *GoToolExecutor {
	if now == nil {
		now = time.Now
	}
	return &GoToolExecutor{agents: agents, now: now}
}

func (e *GoToolExecutor) Execute(ctx context.Context, req ToolExecutionRequest) (ToolExecutionResult, error) {
	if len(req.ToolCalls) == 0 {
		return ToolExecutionResult{}, nil
	}
	results := make([]string, 0, len(req.ToolCalls))
	status := "ok"
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
	}
	return ToolExecutionResult{ReplyText: strings.Join(results, "\n"), Status: status}, nil
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
			ReplyText: fmt.Sprintf("已生成隔离区计划：attachments=%d session=%s", len(req.Message.Attachments), req.Session.Key),
			Status:    "planned",
		}, nil
	case "intake.plan":
		return ToolExecutionResult{
			ReplyText: "已生成 Data Intake Plan 草案；正式入湖前需要人工审批。",
			Status:    "planned",
		}, nil
	case "vlm.inspect":
		return ToolExecutionResult{
			ReplyText: "已进入视觉检查队列：使用 mimo-v2.5 路由，当前不会直接写入 Data Lake。",
			Status:    "planned",
		}, nil
	case "llm.plan":
		return ToolExecutionResult{
			ReplyText: fmt.Sprintf("已进入 planner-agent 会话：%s；当前默认规则 runtime 已就绪，可切换 Python/Mimo planner。", req.Session.Key),
			Status:    "planned",
		}, nil
	case "model.download_hf":
		return e.downloadHFModel(ctx, call)
	case "model.verify_hf":
		return e.verifyHFModel(ctx, call)
	default:
		return ToolExecutionResult{
			ReplyText: fmt.Sprintf("工具 %s 尚未接入。", call.ToolID),
			Status:    "unsupported_tool",
		}, nil
	}
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
	return e.runHFModelScript(ctx, call, false)
}

func (e *GoToolExecutor) verifyHFModel(ctx context.Context, call ToolCall) (ToolExecutionResult, error) {
	return e.runHFModelScript(ctx, call, true)
}

func (e *GoToolExecutor) runHFModelScript(ctx context.Context, call ToolCall, verifyOnly bool) (ToolExecutionResult, error) {
	repoID := strings.TrimSpace(call.Params["repo_id"])
	if repoID == "" {
		repoID = "nvidia/LocateAnything-3B"
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`).MatchString(repoID) {
		return ToolExecutionResult{}, fmt.Errorf("invalid HuggingFace repo_id: %s", repoID)
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
	localDir, err := safeRepoPath(localDir, filepath.Join("data_lake", "models", "artifacts", "huggingface"))
	if err != nil {
		return ToolExecutionResult{}, err
	}
	manifest, err = safeRepoPath(manifest, filepath.Join("data_lake", "catalog", "models"))
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
		"--repo-id", repoID,
		"--local-dir", localDir,
		"--manifest", manifest,
	}
	if verifyOnly {
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
	if verifyOnly {
		action = "校验"
	}
	return ToolExecutionResult{
		ReplyText: fmt.Sprintf("HuggingFace 模型%s完成：repo=%s local_dir=%s manifest=%s", action, repoID, localDir, manifest),
		Status:    "ok",
		Metadata: map[string]string{
			"repo_id":   repoID,
			"local_dir": localDir,
			"manifest":  manifest,
		},
	}, nil
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
