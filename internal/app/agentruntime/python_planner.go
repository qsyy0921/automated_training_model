package agentruntime

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

type PythonPlannerConfig struct {
	Enabled    bool
	Python     string
	WorkDir    string
	PythonPath string
	Timeout    time.Duration
	Worker     bool
}

type PythonPlanner struct {
	cfg    PythonPlannerConfig
	mu     sync.Mutex
	worker *pythonRuntimeWorker
}

func PlannerFromEnv() PlannerPort {
	planner := strings.ToLower(strings.TrimSpace(os.Getenv("AGENT_RUNTIME_PLANNER")))
	if planner == "rule" {
		return NewRulePlanner()
	}
	if planner != "python" && strings.ToLower(strings.TrimSpace(os.Getenv("AGENT_RUNTIME_USE_MIMO"))) != "true" {
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
		Worker:     !isFalseEnv(os.Getenv("AGENT_RUNTIME_PYTHON_WORKER")),
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
	if !isFalseEnv(os.Getenv("AGENT_RUNTIME_PYTHON_WORKER")) {
		cfg.Worker = true
	}
	return &PythonPlanner{cfg: cfg}
}

func (p *PythonPlanner) Plan(ctx context.Context, req PlanRequest) (PlanResult, error) {
	payload, err := json.Marshal(pythonRuntimeRequestFromPlan(req))
	if err != nil {
		return PlanResult{}, err
	}
	if p.cfg.Worker {
		return p.planWithWorker(ctx, req, payload)
	}
	return p.planWithSpawn(ctx, req, payload)
}

func (p *PythonPlanner) PlanStream(ctx context.Context, req PlanRequest, emit func(RuntimeStreamEvent)) (PlanResult, error) {
	payload, err := json.Marshal(pythonRuntimeRequestFromPlan(req))
	if err != nil {
		return PlanResult{}, err
	}
	if !p.cfg.Worker {
		if emit != nil {
			emit(RuntimeStreamEvent{Type: "status", Message: "python worker disabled; fallback to synchronous planner"})
		}
		return p.planWithSpawn(ctx, req, payload)
	}
	return p.planWithWorkerStream(ctx, req, payload, emit)
}

func (p *PythonPlanner) planWithSpawn(ctx context.Context, req PlanRequest, payload []byte) (PlanResult, error) {
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

func (p *PythonPlanner) planWithWorker(ctx context.Context, req PlanRequest, payload []byte) (PlanResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	startCtx, startCancel := context.WithTimeout(ctx, p.cfg.Timeout)
	defer startCancel()
	worker, err := p.ensureWorkerLocked(startCtx)
	if err != nil {
		return PlanResult{}, err
	}
	requestID := fmt.Sprintf("%s-%d", req.Message.ID, time.Now().UnixNano())
	envelope, err := json.Marshal(map[string]json.RawMessage{
		"request_id": json.RawMessage(strconvQuote(requestID)),
		"request":    payload,
	})
	if err != nil {
		return PlanResult{}, err
	}

	runCtx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
	defer cancel()
	responseCh := make(chan pythonWorkerEnvelope, 1)
	errorCh := make(chan error, 1)

	if _, err := worker.stdin.Write(append(envelope, '\n')); err != nil {
		p.stopWorkerLocked()
		return PlanResult{}, fmt.Errorf("write python worker request: %w", err)
	}
	go func() {
		var envelope pythonWorkerEnvelope
		line, err := worker.stdout.ReadString('\n')
		if err != nil {
			errorCh <- err
			return
		}
		if err := json.Unmarshal([]byte(line), &envelope); err != nil {
			errorCh <- fmt.Errorf("decode python worker envelope: %w: %s", err, strings.TrimSpace(line))
			return
		}
		responseCh <- envelope
	}()

	select {
	case <-runCtx.Done():
		p.stopWorkerLocked()
		return PlanResult{}, runCtx.Err()
	case err := <-errorCh:
		p.stopWorkerLocked()
		return PlanResult{}, fmt.Errorf("python worker failed: %w", err)
	case envelope := <-responseCh:
		if envelope.RequestID != requestID {
			p.stopWorkerLocked()
			return PlanResult{}, fmt.Errorf("python worker response id mismatch: got %q want %q", envelope.RequestID, requestID)
		}
		var result pythonRuntimeResult
		if err := json.Unmarshal(envelope.Result, &result); err != nil {
			return PlanResult{}, fmt.Errorf("decode python worker result: %w", err)
		}
		if result.Status == "failed" && strings.TrimSpace(result.ReplyText) != "" {
			return PlanResult{}, fmt.Errorf("python worker failed: %s", result.ReplyText)
		}
		return result.toPlanResult(req), nil
	}
}

func (p *PythonPlanner) planWithWorkerStream(ctx context.Context, req PlanRequest, payload []byte, emit func(RuntimeStreamEvent)) (PlanResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	startCtx, startCancel := context.WithTimeout(ctx, p.cfg.Timeout)
	defer startCancel()
	worker, err := p.ensureWorkerLocked(startCtx)
	if err != nil {
		return PlanResult{}, err
	}
	requestID := fmt.Sprintf("%s-%d", req.Message.ID, time.Now().UnixNano())
	envelope, err := json.Marshal(map[string]any{
		"request_id": requestID,
		"request":    json.RawMessage(payload),
		"stream":     true,
	})
	if err != nil {
		return PlanResult{}, err
	}

	runCtx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
	defer cancel()
	events := make(chan pythonWorkerEnvelope, 16)
	errors := make(chan error, 1)
	if _, err := worker.stdin.Write(append(envelope, '\n')); err != nil {
		p.stopWorkerLocked()
		return PlanResult{}, fmt.Errorf("write python worker stream request: %w", err)
	}
	go func() {
		for {
			line, err := worker.stdout.ReadString('\n')
			if err != nil {
				errors <- err
				return
			}
			var envelope pythonWorkerEnvelope
			if err := json.Unmarshal([]byte(line), &envelope); err != nil {
				errors <- fmt.Errorf("decode python worker stream envelope: %w: %s", err, strings.TrimSpace(line))
				return
			}
			events <- envelope
			if envelope.Type == "result" {
				return
			}
		}
	}()

	for {
		select {
		case <-runCtx.Done():
			p.stopWorkerLocked()
			return PlanResult{}, runCtx.Err()
		case err := <-errors:
			p.stopWorkerLocked()
			return PlanResult{}, fmt.Errorf("python worker stream failed: %w", err)
		case envelope := <-events:
			if envelope.RequestID != requestID {
				p.stopWorkerLocked()
				return PlanResult{}, fmt.Errorf("python worker stream response id mismatch: got %q want %q", envelope.RequestID, requestID)
			}
			switch envelope.Type {
			case "start":
				if emit != nil {
					emit(RuntimeStreamEvent{Type: "start", Intent: string(req.Intent.Kind), AgentID: req.Delegation.AgentID})
				}
			case "status":
				if emit != nil {
					emit(RuntimeStreamEvent{Type: "status", Message: envelope.Message})
				}
			case "delta":
				if emit != nil && envelope.Delta != "" {
					emit(RuntimeStreamEvent{Type: "delta", Delta: envelope.Delta})
				}
			case "result":
				var result pythonRuntimeResult
				if err := json.Unmarshal(envelope.Result, &result); err != nil {
					return PlanResult{}, fmt.Errorf("decode python worker stream result: %w", err)
				}
				if result.Status == "failed" && strings.TrimSpace(result.ReplyText) != "" {
					return PlanResult{}, fmt.Errorf("python worker stream failed: %s", result.ReplyText)
				}
				return result.toPlanResult(req), nil
			}
		}
	}
}

func (p *PythonPlanner) ensureWorkerLocked(ctx context.Context) (*pythonRuntimeWorker, error) {
	if p.worker != nil && p.worker.cmd.Process != nil {
		return p.worker, nil
	}
	cmd := exec.Command(p.cfg.Python, "-m", "agent_runtime.worker")
	if p.cfg.WorkDir != "" {
		cmd.Dir = p.cfg.WorkDir
	}
	cmd.Env = append(os.Environ(), "PYTHONPATH="+p.cfg.PythonPath)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	worker := &pythonRuntimeWorker{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewReader(stdoutPipe),
		stderr: &stderr,
	}
	readyCh := make(chan error, 1)
	go func() {
		line, err := worker.stdout.ReadString('\n')
		if err != nil {
			readyCh <- err
			return
		}
		var envelope pythonWorkerEnvelope
		if err := json.Unmarshal([]byte(line), &envelope); err != nil {
			readyCh <- fmt.Errorf("decode python worker ready: %w: %s", err, strings.TrimSpace(line))
			return
		}
		if envelope.Type != "ready" {
			readyCh <- fmt.Errorf("unexpected python worker ready envelope: %s", strings.TrimSpace(line))
			return
		}
		readyCh <- nil
	}()
	select {
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		return nil, ctx.Err()
	case err := <-readyCh:
		if err != nil {
			_ = cmd.Process.Kill()
			return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}
	}
	p.worker = worker
	return worker, nil
}

func (p *PythonPlanner) stopWorkerLocked() {
	if p.worker == nil {
		return
	}
	_, _ = io.WriteString(p.worker.stdin, "__quit__\n")
	if p.worker.cmd.Process != nil {
		_ = p.worker.cmd.Process.Kill()
	}
	_ = p.worker.cmd.Wait()
	p.worker = nil
}

type pythonRuntimeWorker struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	stderr *bytes.Buffer
}

type pythonWorkerEnvelope struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id"`
	Result    json.RawMessage `json:"result"`
	Error     string          `json:"error,omitempty"`
	Delta     string          `json:"delta,omitempty"`
	Message   string          `json:"message,omitempty"`
}

func strconvQuote(value string) []byte {
	raw, _ := json.Marshal(value)
	return raw
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
