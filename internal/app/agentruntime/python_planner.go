package agentruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

type PythonPlannerConfig struct {
	Enabled    bool
	Python     string
	WorkDir    string
	PythonPath string
	Timeout    time.Duration
}

type PythonPlanner struct {
	cfg PythonPlannerConfig
}

func PlannerFromEnv() PlannerPort {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("AGENT_RUNTIME_PLANNER")), "python") {
		return NewRulePlanner()
	}
	return NewPythonPlanner(PythonPlannerConfigFromEnv())
}

func PythonPlannerConfigFromEnv() PythonPlannerConfig {
	timeout := 15 * time.Second
	if raw := strings.TrimSpace(os.Getenv("AGENT_RUNTIME_PLANNER_TIMEOUT_SECONDS")); raw != "" {
		if parsed, err := time.ParseDuration(raw + "s"); err == nil {
			timeout = parsed
		}
	}
	python := strings.TrimSpace(os.Getenv("AGENT_RUNTIME_PYTHON"))
	if python == "" {
		python = "python"
	}
	pythonPath := strings.TrimSpace(os.Getenv("AGENT_RUNTIME_PYTHONPATH"))
	if pythonPath == "" {
		pythonPath = filepath.Join("workers", "python")
	}
	return PythonPlannerConfig{
		Enabled:    true,
		Python:     python,
		WorkDir:    strings.TrimSpace(os.Getenv("AGENT_RUNTIME_WORKDIR")),
		PythonPath: pythonPath,
		Timeout:    timeout,
	}
}

func NewPythonPlanner(cfg PythonPlannerConfig) *PythonPlanner {
	if cfg.Python == "" {
		cfg.Python = "python"
	}
	if cfg.PythonPath == "" {
		cfg.PythonPath = filepath.Join("workers", "python")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Second
	}
	return &PythonPlanner{cfg: cfg}
}

func (p *PythonPlanner) Plan(ctx context.Context, req PlanRequest) (PlanResult, error) {
	payload, err := json.Marshal(pythonRuntimeRequestFromPlan(req))
	if err != nil {
		return PlanResult{}, err
	}
	requestFile, err := os.CreateTemp("", "agent-runtime-request-*.json")
	if err != nil {
		return PlanResult{}, err
	}
	requestPath := requestFile.Name()
	defer os.Remove(requestPath)
	if _, err := requestFile.Write(payload); err != nil {
		requestFile.Close()
		return PlanResult{}, err
	}
	if err := requestFile.Close(); err != nil {
		return PlanResult{}, err
	}

	runCtx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, p.cfg.Python, "-m", "agent_runtime.main", "--request-file", requestPath)
	if p.cfg.WorkDir != "" {
		cmd.Dir = p.cfg.WorkDir
	}
	cmd.Env = append(os.Environ(), "PYTHONPATH="+p.cfg.PythonPath)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return PlanResult{}, fmt.Errorf("python planner failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var result pythonRuntimeResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return PlanResult{}, fmt.Errorf("decode python planner output: %w: %s", err, strings.TrimSpace(stdout.String()))
	}
	return result.toPlanResult(req), nil
}

type pythonRuntimeRequest struct {
	MessageID   string                `json:"message_id"`
	Channel     channel.Kind          `json:"channel"`
	AccountID   string                `json:"account_id"`
	PeerKind    channel.PeerKind      `json:"peer_kind"`
	PeerID      string                `json:"peer_id"`
	SenderID    string                `json:"sender_id"`
	Text        string                `json:"text,omitempty"`
	Mentioned   bool                  `json:"mentioned,omitempty"`
	Attachments []pythonRuntimeAttach `json:"attachments,omitempty"`
	SessionKey  string                `json:"session_key"`
	Metadata    map[string]string     `json:"metadata,omitempty"`
}

type pythonRuntimeAttach struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	MediaType string `json:"media_type,omitempty"`
	SourceURI string `json:"source_uri,omitempty"`
}

type pythonRuntimeResult struct {
	Status      string                  `json:"status"`
	Intent      Intent                  `json:"intent"`
	ReplyText   string                  `json:"reply_text"`
	Plan        []pythonRuntimePlanItem `json:"plan"`
	Delegations []DelegationDecision    `json:"delegations"`
}

type pythonRuntimePlanItem struct {
	Kind   string            `json:"kind"`
	Params map[string]string `json:"params"`
}

func pythonRuntimeRequestFromPlan(req PlanRequest) pythonRuntimeRequest {
	attachments := make([]pythonRuntimeAttach, 0, len(req.Message.Attachments))
	for _, attachment := range req.Message.Attachments {
		attachments = append(attachments, pythonRuntimeAttach{
			ID:        attachment.ID,
			Name:      attachment.Name,
			MediaType: attachment.MediaType,
			SourceURI: attachment.SourceURI,
		})
	}
	return pythonRuntimeRequest{
		MessageID:   req.Message.ID,
		Channel:     req.Message.Channel,
		AccountID:   req.Message.AccountID,
		PeerKind:    req.Message.Peer.Kind,
		PeerID:      req.Message.Peer.ID,
		SenderID:    req.Message.SenderID,
		Text:        req.Message.Text,
		Mentioned:   req.Message.Mentioned,
		Attachments: attachments,
		SessionKey:  req.Session.Key,
		Metadata: map[string]string{
			"go_intent": string(req.Intent.Kind),
			"agent_id":  req.Session.AgentID,
		},
	}
}

func (r pythonRuntimeResult) toPlanResult(req PlanRequest) PlanResult {
	intent := r.Intent
	if intent.Kind == "" {
		intent = req.Intent
	}
	delegation := req.Delegation
	if len(r.Delegations) > 0 {
		delegation = r.Delegations[0]
	}
	return PlanResult{
		Intent:     intent,
		Delegation: delegation,
		ReplyText:  r.ReplyText,
		ToolCalls:  pythonPlanItemsToToolCalls(r.Plan),
		Status:     r.Status,
	}
}

func pythonPlanItemsToToolCalls(items []pythonRuntimePlanItem) []ToolCall {
	if len(items) == 0 {
		return nil
	}
	calls := make([]ToolCall, 0, len(items))
	for i, item := range items {
		if item.Kind == "" {
			continue
		}
		calls = append(calls, ToolCall{
			ID:     fmt.Sprintf("python-call-%d", i+1),
			ToolID: item.Kind,
			Params: item.Params,
		})
	}
	return calls
}
