package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/domain/autolabel"
	"github.com/qsyy0921/automated_training_model/internal/domain/deployment"
	"github.com/qsyy0921/automated_training_model/internal/domain/evaluation"
	"github.com/qsyy0921/automated_training_model/internal/domain/modelregistry"
	"github.com/qsyy0921/automated_training_model/internal/domain/training"
	"github.com/qsyy0921/automated_training_model/internal/domain/workflow"
)

func (s *Server) submitAutoLabel(w http.ResponseWriter, r *http.Request) {
	var req autolabel.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	job, err := s.lifecycle.SubmitAutoLabel(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job": job})
}

func (s *Server) submitTraining(w http.ResponseWriter, r *http.Request) {
	var req training.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	run, err := s.lifecycle.SubmitTraining(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"run": run})
}

func (s *Server) submitEvaluation(w http.ResponseWriter, r *http.Request) {
	var req evaluation.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	run, err := s.lifecycle.SubmitEvaluation(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"run": run})
}

func (s *Server) listModels(w http.ResponseWriter, r *http.Request) {
	models, err := s.lifecycle.ListModels(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": models})
}

func (s *Server) modelDetail(w http.ResponseWriter, r *http.Request) {
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/models/"), "/")
	if id == "" {
		writeErrorText(w, http.StatusNotFound, "model id missing")
		return
	}
	model, err := s.lifecycle.GetModel(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"model": model})
}

func (s *Server) registerModel(w http.ResponseWriter, r *http.Request) {
	var req modelregistry.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	model, err := s.lifecycle.RegisterModel(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"model": model})
}

func (s *Server) submitDeployment(w http.ResponseWriter, r *http.Request) {
	var req deployment.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	dep, err := s.lifecycle.SubmitDeployment(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"deployment": dep})
}

func (s *Server) listTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := s.lifecycle.ListTasks(r.Context(), runtimeTraceLimit(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": tasks})
}

func (s *Server) taskDetail(w http.ResponseWriter, r *http.Request) {
	id, action := lifecycleTaskPath(r)
	if id == "" {
		writeErrorText(w, http.StatusNotFound, "task id missing")
		return
	}
	switch action {
	case "":
	case "logs":
		s.taskLogs(w, r, id)
		return
	case "logs/stream":
		s.taskLogStream(w, r, id)
		return
	default:
		writeError(w, http.StatusNotFound, errors.New("task not found"))
		return
	}
	task, err := s.lifecycle.TaskStatus(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"task": task})
}

func (s *Server) cancelTask(w http.ResponseWriter, r *http.Request) {
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/tasks/"), "/")
	if id == "" {
		writeErrorText(w, http.StatusNotFound, "task id missing")
		return
	}
	if err := s.lifecycle.CancelTask(r.Context(), id); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"canceled": true})
}

func (s *Server) taskLogs(w http.ResponseWriter, r *http.Request, id string) {
	task, err := s.lifecycle.TaskStatus(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, lifecycleTaskLogsPayload(task, lifecycleTaskRecentLogs(task, runtimeTraceLimit(r))))
}

func (s *Server) taskLogStream(w http.ResponseWriter, r *http.Request, id string) {
	task, err := s.lifecycle.TaskStatus(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	flusher, _ := w.(http.Flusher)
	emit := func(event map[string]any) {
		_ = enc.Encode(event)
		if flusher != nil {
			flusher.Flush()
		}
	}
	for _, log := range lifecycleTaskRecentLogs(task, runtimeTraceLimit(r)) {
		emit(map[string]any{"type": "log", "task_id": id, "log": log})
	}
	if lifecycleTaskTerminal(task.Status) {
		emit(lifecycleTaskFinalEvent(task))
		return
	}
	timeout := time.NewTimer(runtimeStreamTimeout(r))
	defer timeout.Stop()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	lastCount := len(task.Logs)
	for {
		select {
		case <-r.Context().Done():
			return
		case <-timeout.C:
			latest, err := s.lifecycle.TaskStatus(r.Context(), id)
			if err == nil && latest != nil {
				final := lifecycleTaskFinalEvent(latest)
				final["message"] = "stream timeout"
				emit(final)
			}
			return
		case <-ticker.C:
			latest, err := s.lifecycle.TaskStatus(r.Context(), id)
			if err != nil || latest == nil {
				emit(map[string]any{"type": "error", "task_id": id, "message": "task not found"})
				return
			}
			if len(latest.Logs) > lastCount {
				for _, log := range latest.Logs[lastCount:] {
					emit(map[string]any{"type": "log", "task_id": id, "log": log})
				}
				lastCount = len(latest.Logs)
			}
			if lifecycleTaskTerminal(latest.Status) {
				emit(lifecycleTaskFinalEvent(latest))
				return
			}
		}
	}
}

func lifecycleTaskLogsPayload(task *workflow.Task, logs []workflow.TaskLog) map[string]any {
	return map[string]any{
		"id":               task.ID,
		"task_id":          task.ID,
		"type":             task.Type,
		"status":           task.Status,
		"progress_percent": task.ProgressPercent,
		"message":          task.Message,
		"retryable":        task.Retryable,
		"attempt":          task.Attempt,
		"max_attempts":     task.MaxAttempts,
		"worker_heartbeat": task.WorkerHeartbeat,
		"artifacts":        task.Artifacts,
		"stdout":           task.Stdout,
		"stderr":           task.Stderr,
		"metadata":         task.Metadata,
		"logs":             logs,
		"created_at":       task.CreatedAt,
		"started_at":       task.StartedAt,
		"finished_at":      task.FinishedAt,
		"updated_at":       task.UpdatedAt,
	}
}

func lifecycleTaskPath(r *http.Request) (string, string) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], strings.Join(parts[1:], "/")
}

func lifecycleTaskRecentLogs(task *workflow.Task, limit int) []workflow.TaskLog {
	if task == nil || len(task.Logs) == 0 {
		return nil
	}
	if limit <= 0 || limit >= len(task.Logs) {
		return task.Logs
	}
	return task.Logs[len(task.Logs)-limit:]
}

func lifecycleTaskFinalEvent(task *workflow.Task) map[string]any {
	event := lifecycleTaskLogsPayload(task, nil)
	event["type"] = "final"
	return event
}

func lifecycleTaskTerminal(status workflow.TaskStatus) bool {
	switch status {
	case workflow.TaskCompleted, workflow.TaskFailed, workflow.TaskCanceled:
		return true
	default:
		return false
	}
}
