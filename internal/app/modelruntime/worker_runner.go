package modelruntime

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
)

const defaultPythonModelWorkerTimeout = 5 * time.Minute
const maxWorkerOutputBytes = 64 * 1024
const workerEventPrefix = "ATM_EVENT "

type WorkerJobRequest struct {
	TaskID     string            `json:"task_id"`
	WorkflowID string            `json:"workflow_id"`
	AgentID    string            `json:"agent_id"`
	ToolID     string            `json:"tool_id"`
	Action     string            `json:"action"`
	DatasetID  string            `json:"dataset_id,omitempty"`
	Scene      string            `json:"scene,omitempty"`
	DryRun     bool              `json:"dry_run"`
	Params     map[string]string `json:"params,omitempty"`
}

type WorkerHeartbeat struct {
	At      string `json:"at"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type WorkerLog struct {
	At      string `json:"at"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

type WorkerArtifact struct {
	Name     string            `json:"name"`
	URI      string            `json:"uri"`
	Kind     string            `json:"kind,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type WorkerJobResult struct {
	TaskID      string             `json:"task_id"`
	Status      string             `json:"status"`
	Artifacts   []WorkerArtifact   `json:"artifacts,omitempty"`
	Metrics     map[string]float64 `json:"metrics,omitempty"`
	Logs        []WorkerLog        `json:"logs,omitempty"`
	Heartbeat   *WorkerHeartbeat   `json:"heartbeat,omitempty"`
	Attempt     int                `json:"attempt,omitempty"`
	MaxAttempts int                `json:"max_attempts,omitempty"`
	Retryable   bool               `json:"retryable,omitempty"`
	Message     string             `json:"message,omitempty"`
	StartedAt   string             `json:"started_at,omitempty"`
	FinishedAt  string             `json:"finished_at,omitempty"`
	Stdout      string             `json:"stdout,omitempty"`
	Stderr      string             `json:"stderr,omitempty"`
}

type WorkerRuntimeEvent struct {
	Type    string `json:"type"`
	At      string `json:"at,omitempty"`
	Level   string `json:"level,omitempty"`
	Message string `json:"message,omitempty"`
	Status  string `json:"status,omitempty"`
	Stream  string `json:"stream,omitempty"`
	Text    string `json:"text,omitempty"`
}

type WorkerRunError struct {
	Kind          string
	Message       string
	Stdout        string
	Stderr        string
	RetryableFlag bool
}

func (e WorkerRunError) Error() string {
	return e.Message
}

func (e WorkerRunError) WorkerRetryable() bool {
	return e.RetryableFlag
}

func (e WorkerRunError) WorkerKind() string {
	return e.Kind
}

type PythonModelWorkerRunner struct {
	python     func() string
	pythonPath func() string
	timeout    func() time.Duration
	command    func(context.Context, string, ...string) *exec.Cmd
}

func NewPythonModelWorkerRunner() *PythonModelWorkerRunner {
	return &PythonModelWorkerRunner{
		python:     pythonFromEnv,
		pythonPath: pythonPathFromEnv,
		timeout:    pythonModelWorkerTimeoutFromEnv,
		command:    exec.CommandContext,
	}
}

func (r *PythonModelWorkerRunner) Run(ctx context.Context, req WorkerJobRequest, emit func(WorkerRuntimeEvent)) (WorkerJobResult, error) {
	if r == nil {
		return WorkerJobResult{}, fmt.Errorf("python model worker runner is not configured")
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return WorkerJobResult{}, fmt.Errorf("encode python worker job: %w", err)
	}
	runCtx, cancel := context.WithTimeout(ctx, r.timeout())
	defer cancel()

	cmd := r.command(runCtx, r.python(), "-m", "agent_worker.main", "--job-json", string(payload))
	baseEnv := cmd.Env
	if len(baseEnv) == 0 {
		baseEnv = os.Environ()
	}
	cmd.Env = withPythonPath(baseEnv, r.pythonPath())
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return WorkerJobResult{}, fmt.Errorf("pipe python worker stdout: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return WorkerJobResult{}, fmt.Errorf("pipe python worker stderr: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return WorkerJobResult{}, fmt.Errorf("start python worker: %w", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var ioWG sync.WaitGroup
	ioWG.Add(2)
	go func() {
		defer ioWG.Done()
		_, _ = io.Copy(&stdout, stdoutPipe)
	}()
	go func() {
		defer ioWG.Done()
		consumeWorkerStderr(stderrPipe, &stderr, emit)
	}()
	runErr := cmd.Wait()
	ioWG.Wait()
	stdoutText := truncateWorkerOutput(stdout.String())
	stderrText := truncateWorkerOutput(stderr.String())
	if runCtx.Err() == context.DeadlineExceeded {
		message := fmt.Sprintf("python model worker timed out after %s; stderr=%s", r.timeout(), compactWorkerError(stderrText))
		return WorkerJobResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Retryable: true,
			Message:   message,
			Stdout:    stdoutText,
			Stderr:    stderrText,
		}, WorkerRunError{
			Kind:          "timeout",
			Message:       message,
			Stdout:        stdoutText,
			Stderr:        stderrText,
			RetryableFlag: true,
		}
	}

	var result WorkerJobResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdoutText)), &result); err != nil {
		message := fmt.Sprintf("decode python model worker result: %v; stdout=%s stderr=%s", err, compactWorkerError(stdoutText), compactWorkerError(stderrText))
		return WorkerJobResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Retryable: false,
			Message:   message,
			Stdout:    stdoutText,
			Stderr:    stderrText,
		}, WorkerRunError{
			Kind:          "decode_result",
			Message:       message,
			Stdout:        stdoutText,
			Stderr:        stderrText,
			RetryableFlag: false,
		}
	}
	result.Stdout = stdoutText
	result.Stderr = stderrText
	if runErr != nil && strings.TrimSpace(result.Message) == "" {
		result.Message = runErr.Error()
	}
	return result, nil
}

func consumeWorkerStderr(src io.Reader, stderr *bytes.Buffer, emit func(WorkerRuntimeEvent)) {
	scanner := bufio.NewScanner(src)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if event, ok := parseWorkerRuntimeEvent(trimmed); ok {
			if emit != nil {
				emit(event)
			}
			continue
		}
		if stderr.Len() > 0 {
			stderr.WriteByte('\n')
		}
		stderr.WriteString(line)
	}
}

func parseWorkerRuntimeEvent(line string) (WorkerRuntimeEvent, bool) {
	if !strings.HasPrefix(line, workerEventPrefix) {
		return WorkerRuntimeEvent{}, false
	}
	var event WorkerRuntimeEvent
	if err := json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, workerEventPrefix))), &event); err != nil {
		return WorkerRuntimeEvent{}, false
	}
	if strings.TrimSpace(event.Type) == "" {
		return WorkerRuntimeEvent{}, false
	}
	return event, true
}

func pythonModelWorkerTimeoutFromEnv() time.Duration {
	raw := strings.TrimSpace(os.Getenv("AGENT_RUNTIME_MODEL_WORKER_TIMEOUT_MS"))
	if raw == "" {
		return defaultPythonModelWorkerTimeout
	}
	value, err := time.ParseDuration(raw + "ms")
	if err != nil || value <= 0 {
		return defaultPythonModelWorkerTimeout
	}
	return value
}

func pythonPathFromEnv() string {
	path := strings.TrimSpace(os.Getenv("AGENT_RUNTIME_PYTHONPATH"))
	if path == "" {
		path = filepath.Join("workers", "python")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

func withPythonPath(env []string, pythonPath string) []string {
	if strings.TrimSpace(pythonPath) == "" {
		return env
	}
	key := "PYTHONPATH="
	out := make([]string, 0, len(env)+1)
	found := false
	for _, item := range env {
		if !strings.HasPrefix(strings.ToUpper(item), key) {
			out = append(out, item)
			continue
		}
		found = true
		current := strings.TrimPrefix(item, key)
		if strings.TrimSpace(current) == "" {
			out = append(out, key+pythonPath)
			continue
		}
		out = append(out, key+pythonPath+string(os.PathListSeparator)+current)
	}
	if !found {
		out = append(out, key+pythonPath)
	}
	return out
}

func truncateWorkerOutput(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\r\n", "\n"))
	if len(value) <= maxWorkerOutputBytes {
		return value
	}
	return value[:maxWorkerOutputBytes] + "\n...[truncated]"
}

func compactWorkerError(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	if len(value) > 240 {
		return value[:240] + "..."
	}
	return value
}
