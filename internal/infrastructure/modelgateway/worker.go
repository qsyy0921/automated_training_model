package modelgateway

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/app/modelruntime"
	"github.com/qsyy0921/automated_training_model/internal/app/workflowapp"
	"github.com/qsyy0921/automated_training_model/internal/domain/workflow"
)

const defaultWorkerGatewayTimeout = 2 * time.Minute

type workerRunner interface {
	Run(context.Context, modelruntime.WorkerJobRequest, func(modelruntime.WorkerRuntimeEvent)) (modelruntime.WorkerJobResult, error)
}

type taskArtifactManifestWriter interface {
	WriteArtifactManifest(task workflow.Task) (string, error)
}

type WorkerGateway struct {
	queue   workflowapp.TaskQueue
	runner  workerRunner
	now     func() time.Time
	timeout func() time.Duration

	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

func NewWorkerGateway(queue workflowapp.TaskQueue) *WorkerGateway {
	return NewWorkerGatewayWithRunner(queue, modelruntime.NewPythonModelWorkerRunner(), time.Now, func() time.Duration {
		return defaultWorkerGatewayTimeout
	})
}

func NewWorkerGatewayWithRunner(queue workflowapp.TaskQueue, runner workerRunner, now func() time.Time, timeout func() time.Duration) *WorkerGateway {
	if now == nil {
		now = time.Now
	}
	if timeout == nil {
		timeout = func() time.Duration { return defaultWorkerGatewayTimeout }
	}
	return &WorkerGateway{
		queue:   queue,
		runner:  runner,
		now:     now,
		timeout: timeout,
		cancels: map[string]context.CancelFunc{},
	}
}

func (g *WorkerGateway) Submit(ctx context.Context, taskType string, payload map[string]string) (string, error) {
	id, err := g.queue.Enqueue(ctx, workflow.TaskSpec{Type: taskType, Payload: payload})
	if err != nil {
		return "", err
	}
	now := g.now()
	if err := g.queue.Update(ctx, id, func(task *workflow.Task) {
		task.Message = "queued python worker dry-run"
		task.ProgressPercent = 0
		task.Metadata = mergeTaskMetadata(task.Metadata, map[string]string{
			"execution_path": "python-worker",
			"dry_run":        "true",
			"task_type":      taskType,
		})
		task.Logs = appendTaskLog(task.Logs, now, "info", task.Message)
	}); err != nil {
		return "", err
	}
	if supportsWorkerTask(taskType) {
		go g.runWorkerTask(id, taskType, payload)
	}
	return id, nil
}

func (g *WorkerGateway) Status(ctx context.Context, id string) (*workflow.Task, error) {
	return g.queue.Status(ctx, id)
}

func (g *WorkerGateway) Cancel(ctx context.Context, id string) error {
	if err := g.queue.Cancel(ctx, id); err != nil {
		return err
	}
	g.cancelRunningTask(id)
	return nil
}

func (g *WorkerGateway) runWorkerTask(id string, taskType string, payload map[string]string) {
	task, err := g.queue.Status(context.Background(), id)
	if err != nil || task == nil {
		return
	}
	if task.Status == workflow.TaskCanceled {
		return
	}
	started := g.now()
	_ = g.queue.Update(context.Background(), id, func(task *workflow.Task) {
		task.Status = workflow.TaskRunning
		task.StartedAt = &started
		task.Message = "running python worker dry-run"
		task.ProgressPercent = 15
		task.Metadata = mergeTaskMetadata(task.Metadata, map[string]string{
			"execution_path": "python-worker",
			"tool_id":        taskType,
			"action":         taskType,
		})
		task.Logs = appendTaskLog(task.Logs, started, "info", task.Message)
	})

	ctx, cancel := context.WithTimeout(context.Background(), g.timeout())
	g.setTaskCancel(id, cancel)
	defer cancel()
	defer g.clearTaskCancel(id)

	result, runErr := g.runner.Run(ctx, modelruntime.WorkerJobRequest{
		TaskID:     id,
		WorkflowID: "lifecycle-worker",
		AgentID:    lifecycleAgentID(taskType),
		ToolID:     taskType,
		Action:     taskType,
		DatasetID:  strings.TrimSpace(payload["dataset_id"]),
		DryRun:     true,
		Params:     payload,
	}, func(event modelruntime.WorkerRuntimeEvent) {
		g.applyWorkerRuntimeEvent(id, event)
	})
	finished := g.now()
	_ = g.queue.Update(context.Background(), id, func(task *workflow.Task) {
		task.FinishedAt = &finished
		task.WorkerHeartbeat = toTaskHeartbeat(result.Heartbeat)
		task.Artifacts = toTaskArtifacts(result.Artifacts)
		if strings.TrimSpace(result.Stdout) != "" {
			task.Stdout = result.Stdout
		}
		if strings.TrimSpace(result.Stderr) != "" {
			task.Stderr = result.Stderr
		}
		task.Attempt = result.Attempt
		task.MaxAttempts = result.MaxAttempts
		task.Retryable = result.Retryable
		task.Metadata = mergeTaskMetadata(task.Metadata, workerTaskMetadata(result))
		task.Logs = appendWorkerTaskLogs(task.Logs, result.Logs, finished)
		if ctx.Err() == context.Canceled || task.Status == workflow.TaskCanceled {
			task.Status = workflow.TaskCanceled
			task.Message = "python worker canceled"
			task.Logs = appendTaskLog(task.Logs, finished, "warn", task.Message)
			return
		}
		if runErr != nil {
			task.Status = workflow.TaskFailed
			task.Error = runErr.Error()
			task.Message = firstNonEmpty(result.Message, "python worker execution failed")
			task.Retryable = isWorkerRetryable(runErr)
			task.Metadata = mergeTaskMetadata(task.Metadata, pythonWorkerErrorMetadata(runErr))
			task.Logs = appendTaskLog(task.Logs, finished, "error", runErr.Error())
			return
		}
		switch strings.ToLower(strings.TrimSpace(result.Status)) {
		case "completed", "ok", "succeeded":
			task.Status = workflow.TaskCompleted
			task.Message = firstNonEmpty(result.Message, "python worker dry-run completed")
			task.ProgressPercent = 100
		case "failed":
			task.Status = workflow.TaskFailed
			task.Message = firstNonEmpty(result.Message, "python worker reported failure")
			task.Error = result.Message
		default:
			task.Status = workflow.TaskFailed
			task.Message = firstNonEmpty(result.Message, "python worker returned unknown status")
			task.Error = firstNonEmpty(result.Message, result.Status)
		}
		task.Logs = appendTaskLog(task.Logs, finished, "info", task.Message)
	})
	g.archiveTaskArtifacts(id)
}

func (g *WorkerGateway) applyWorkerRuntimeEvent(id string, event modelruntime.WorkerRuntimeEvent) {
	now := g.now()
	_ = g.queue.Update(context.Background(), id, func(task *workflow.Task) {
		switch strings.ToLower(strings.TrimSpace(event.Type)) {
		case "heartbeat":
			task.WorkerHeartbeat = &workflow.TaskHeartbeat{
				At:      firstNonEmpty(event.At, now.Format(time.RFC3339Nano)),
				Status:  firstNonEmpty(event.Status, "running"),
				Message: strings.TrimSpace(event.Message),
			}
			task.Metadata = mergeTaskMetadata(task.Metadata, map[string]string{
				"worker_heartbeat_status": firstNonEmpty(event.Status, "running"),
			})
			task.Logs = appendTaskLog(task.Logs, parseWorkerEventTime(event.At, now), "info", "worker heartbeat: "+firstNonEmpty(event.Status, "running")+" "+strings.TrimSpace(event.Message))
		case "log":
			task.Logs = appendTaskLog(task.Logs, parseWorkerEventTime(event.At, now), firstNonEmpty(strings.TrimSpace(event.Level), "info"), strings.TrimSpace(event.Message))
		case "stream":
			text := strings.TrimSpace(event.Text)
			if text == "" {
				return
			}
			switch strings.ToLower(strings.TrimSpace(event.Stream)) {
			case "stdout":
				task.Stdout = appendTaskOutput(task.Stdout, text)
				task.Logs = appendTaskLog(task.Logs, parseWorkerEventTime(event.At, now), "info", "stdout> "+text)
			case "stderr":
				task.Stderr = appendTaskOutput(task.Stderr, text)
				task.Logs = appendTaskLog(task.Logs, parseWorkerEventTime(event.At, now), "warn", "stderr> "+text)
			default:
				task.Logs = appendTaskLog(task.Logs, parseWorkerEventTime(event.At, now), "info", text)
			}
		}
	})
}

func supportsWorkerTask(taskType string) bool {
	switch strings.TrimSpace(taskType) {
	case "autolabel.run", "training.run", "evaluation.run", "deployment.run":
		return true
	default:
		return false
	}
}

func lifecycleAgentID(taskType string) string {
	switch strings.TrimSpace(taskType) {
	case "autolabel.run":
		return "vlm-label-agent"
	case "training.run":
		return "training-agent"
	case "evaluation.run":
		return "evaluation-agent"
	case "deployment.run":
		return "deployment-agent"
	default:
		return "lifecycle-agent"
	}
}

func appendTaskLog(logs []workflow.TaskLog, at time.Time, level string, message string) []workflow.TaskLog {
	message = strings.TrimSpace(message)
	if message == "" {
		return logs
	}
	level = strings.TrimSpace(level)
	if level == "" {
		level = "info"
	}
	logs = append(logs, workflow.TaskLog{At: at, Level: level, Message: message})
	if len(logs) > 200 {
		return logs[len(logs)-200:]
	}
	return logs
}

func appendWorkerTaskLogs(logs []workflow.TaskLog, workerLogs []modelruntime.WorkerLog, fallback time.Time) []workflow.TaskLog {
	for _, log := range workerLogs {
		at := fallback
		if parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(log.At)); err == nil {
			at = parsed
		}
		logs = appendTaskLog(logs, at, log.Level, log.Message)
	}
	return logs
}

func toTaskHeartbeat(heartbeat *modelruntime.WorkerHeartbeat) *workflow.TaskHeartbeat {
	if heartbeat == nil {
		return nil
	}
	return &workflow.TaskHeartbeat{At: heartbeat.At, Status: heartbeat.Status, Message: heartbeat.Message}
}

func toTaskArtifacts(artifacts []modelruntime.WorkerArtifact) []workflow.TaskArtifact {
	if len(artifacts) == 0 {
		return nil
	}
	out := make([]workflow.TaskArtifact, 0, len(artifacts))
	for _, artifact := range artifacts {
		out = append(out, workflow.TaskArtifact{
			Name:     artifact.Name,
			URI:      artifact.URI,
			Kind:     artifact.Kind,
			Metadata: mergeTaskMetadata(nil, artifact.Metadata),
		})
	}
	return out
}

func mergeTaskMetadata(base map[string]string, overlay map[string]string) map[string]string {
	if len(base) == 0 && len(overlay) == 0 {
		return nil
	}
	out := map[string]string{}
	for key, value := range base {
		if strings.TrimSpace(value) != "" {
			out[key] = value
		}
	}
	for key, value := range overlay {
		if strings.TrimSpace(value) != "" {
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func workerTaskMetadata(result modelruntime.WorkerJobResult) map[string]string {
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
	return metadata
}

func parseWorkerEventTime(value string, fallback time.Time) time.Time {
	if parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value)); err == nil {
		return parsed
	}
	return fallback
}

func appendTaskOutput(current string, line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return current
	}
	if strings.TrimSpace(current) == "" {
		return truncateTaskOutput(line)
	}
	return truncateTaskOutput(current + "\n" + line)
}

func truncateTaskOutput(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\r\n", "\n"))
	const limit = 64 * 1024
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "\n...[truncated]"
}

func isWorkerRetryable(err error) bool {
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (g *WorkerGateway) archiveTaskArtifacts(id string) {
	writer, ok := g.queue.(taskArtifactManifestWriter)
	if !ok {
		return
	}
	task, err := g.queue.Status(context.Background(), id)
	if err != nil || task == nil {
		return
	}
	path, err := writer.WriteArtifactManifest(*task)
	if err != nil || strings.TrimSpace(path) == "" {
		return
	}
	_ = g.queue.Update(context.Background(), id, func(task *workflow.Task) {
		task.Metadata = mergeTaskMetadata(task.Metadata, map[string]string{
			"artifact_manifest": path,
		})
	})
}

func (g *WorkerGateway) setTaskCancel(id string, cancel context.CancelFunc) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.cancels[id] = cancel
}

func (g *WorkerGateway) clearTaskCancel(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.cancels, id)
}

func (g *WorkerGateway) cancelRunningTask(id string) {
	g.mu.Lock()
	cancel := g.cancels[id]
	g.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func EncodeTaskPayload(value any) (map[string]string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return map[string]string{"request_json": string(raw)}, nil
}
