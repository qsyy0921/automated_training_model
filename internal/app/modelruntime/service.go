package modelruntime

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
)

const defaultRepoID = "nvidia/LocateAnything-3B"

type Service struct {
	python func() string
}

type ToolResult struct {
	ReplyText string
	Status    string
	Metadata  map[string]string
}

type HFModelRequest struct {
	RepoID     string
	LocalDir   string
	Manifest   string
	VerifyOnly bool
}

type LocateAnythingSmokeRequest struct {
	ModelDir string
	DataRoot string
	Output   string
}

func NewService() *Service {
	return &Service{python: pythonFromEnv}
}

func (s *Service) DownloadRequiresApproval(params map[string]string) bool {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("AGENT_RUNTIME_REQUIRE_MODEL_DOWNLOAD_APPROVAL")), "true") {
		return false
	}
	return !strings.EqualFold(strings.TrimSpace(params["approved"]), "true")
}

func (s *Service) RunHFModelTool(ctx context.Context, params map[string]string, verifyOnly bool) (ToolResult, error) {
	req, err := PrepareHFModelRequest(params, verifyOnly)
	if err != nil {
		return ToolResult{}, err
	}
	timeout := HFDownloadTimeout()
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	args := []string{
		filepath.Join("skills", "huggingface-model-downloader", "scripts", "download_hf_snapshot.py"),
		"--repo-id", req.RepoID,
		"--local-dir", req.LocalDir,
		"--manifest", req.Manifest,
	}
	if req.VerifyOnly {
		args = append(args, "--verify-only")
	}
	cmd := exec.CommandContext(runCtx, s.python(), args...)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if runCtx.Err() == context.DeadlineExceeded {
		return ToolResult{}, fmt.Errorf("HuggingFace model tool timed out after %s", timeout)
	}
	if err != nil {
		return ToolResult{}, fmt.Errorf("HuggingFace model tool failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	action := "下载"
	if req.VerifyOnly {
		action = "校验"
	}
	return ToolResult{
		ReplyText: fmt.Sprintf("HuggingFace 模型%s完成：repo=%s local_dir=%s manifest=%s", action, req.RepoID, req.LocalDir, req.Manifest),
		Status:    "ok",
		Metadata: map[string]string{
			"repo_id":   req.RepoID,
			"local_dir": req.LocalDir,
			"manifest":  req.Manifest,
		},
	}, nil
}

func (s *Service) SmokeLocateAnything(ctx context.Context, params map[string]string) (ToolResult, error) {
	req, err := PrepareLocateAnythingSmokeRequest(params)
	if err != nil {
		return ToolResult{}, err
	}
	timeout := HFDownloadTimeout()
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	args := []string{
		filepath.Join("workers", "python", "agent_worker", "locateanything_smoke.py"),
		"--model-dir", req.ModelDir,
		"--data-root", req.DataRoot,
		"--output", req.Output,
	}
	cmd := exec.CommandContext(runCtx, s.python(), args...)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if runCtx.Err() == context.DeadlineExceeded {
		return ToolResult{}, fmt.Errorf("LocateAnything smoke timed out after %s", timeout)
	}
	if err != nil {
		return ToolResult{}, fmt.Errorf("LocateAnything smoke failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	status, modelLoad, realInference := ParseLocateAnythingSmokeOutput(out)
	return ToolResult{
		ReplyText: fmt.Sprintf("LocateAnything-3B 可用性 smoke 完成：status=%s model_load=%s real_inference=%s report=%s", status, modelLoad, realInference, req.Output),
		Status:    status,
		Metadata: map[string]string{
			"model_id":       defaultRepoID,
			"model_dir":      req.ModelDir,
			"data_root":      req.DataRoot,
			"smoke_report":   req.Output,
			"model_load":     modelLoad,
			"real_inference": realInference,
		},
	}, nil
}

func PrepareHFModelRequest(params map[string]string, verifyOnly bool) (HFModelRequest, error) {
	repoID := strings.TrimSpace(params["repo_id"])
	if repoID == "" {
		repoID = defaultRepoID
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`).MatchString(repoID) {
		return HFModelRequest{}, fmt.Errorf("invalid HuggingFace repo_id: %s", repoID)
	}
	localDir := strings.TrimSpace(params["local_dir"])
	if localDir == "" {
		localDir = filepath.Join("data_lake", "models", "artifacts", "huggingface", strings.ReplaceAll(repoID, "/", string(filepath.Separator)))
	}
	manifest := strings.TrimSpace(params["manifest"])
	if manifest == "" {
		manifest = filepath.Join("data_lake", "catalog", "models", strings.ReplaceAll(repoID, "/", "_")+".download.json")
	}
	if strings.EqualFold(params["verify_only"], "true") {
		verifyOnly = true
	}
	safeLocalDir, err := SafeRepoPath(localDir, filepath.Join("data_lake", "models", "artifacts", "huggingface"))
	if err != nil {
		return HFModelRequest{}, err
	}
	safeManifest, err := SafeRepoPath(manifest, filepath.Join("data_lake", "catalog", "models"))
	if err != nil {
		return HFModelRequest{}, err
	}
	return HFModelRequest{RepoID: repoID, LocalDir: safeLocalDir, Manifest: safeManifest, VerifyOnly: verifyOnly}, nil
}

func PrepareLocateAnythingSmokeRequest(params map[string]string) (LocateAnythingSmokeRequest, error) {
	modelDir := strings.TrimSpace(params["model_dir"])
	if modelDir == "" {
		modelDir = filepath.Join("data_lake", "models", "artifacts", "huggingface", "nvidia", "LocateAnything-3B")
	}
	dataRoot := strings.TrimSpace(params["data_root"])
	if dataRoot == "" {
		dataRoot = filepath.Join("data_lake", "raw", "datasets", "shanghaitech", "original")
	}
	output := strings.TrimSpace(params["output"])
	if output == "" {
		output = filepath.Join("data_lake", "catalog", "models", "nvidia_LocateAnything-3B.smoke.json")
	}
	safeModelDir, err := SafeRepoPath(modelDir, filepath.Join("data_lake", "models", "artifacts", "huggingface"))
	if err != nil {
		return LocateAnythingSmokeRequest{}, err
	}
	safeDataRoot, err := SafeRepoPath(dataRoot, filepath.Join("data_lake", "raw", "datasets"))
	if err != nil {
		return LocateAnythingSmokeRequest{}, err
	}
	safeOutput, err := SafeRepoPath(output, filepath.Join("data_lake", "catalog", "models"))
	if err != nil {
		return LocateAnythingSmokeRequest{}, err
	}
	return LocateAnythingSmokeRequest{ModelDir: safeModelDir, DataRoot: safeDataRoot, Output: safeOutput}, nil
}

func ParseLocateAnythingSmokeOutput(out []byte) (string, string, string) {
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

func SafeRepoPath(path string, allowedRoot string) (string, error) {
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

func HFDownloadTimeout() time.Duration {
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

func pythonFromEnv() string {
	python := strings.TrimSpace(os.Getenv("AGENT_RUNTIME_PYTHON"))
	if python == "" {
		return "python"
	}
	return python
}
