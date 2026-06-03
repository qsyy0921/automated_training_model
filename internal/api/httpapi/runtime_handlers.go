package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/app/agentruntime"
	"github.com/qsyy0921/automated_training_model/internal/infrastructure/middleware"
)

func (s *Server) runtimeStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"runtime":  agentruntime.Status(),
		"snapshot": s.runtime.Snapshot(runtimeTraceLimit(r)),
		"gateway": map[string]any{
			"auth": middleware.GatewayAuthStatusFromEnv(),
		},
	})
}

func (s *Server) runtimeSessions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"sessions": s.runtime.ListSessions()})
}

func (s *Server) runtimeTraces(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"traces": s.runtime.ListTraces(runtimeTraceLimit(r))})
}

func (s *Server) runtimeModelJobs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"jobs": s.runtime.ListModelJobs(runtimeTraceLimit(r))})
}

func (s *Server) runtimeIntakeWorkflows(w http.ResponseWriter, r *http.Request) {
	workflows, err := s.runtime.ListIntakeWorkflows(r.Context(), runtimeTraceLimit(r))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workflows": workflows})
}

func (s *Server) runtimeIntakeWorkflowDetail(w http.ResponseWriter, r *http.Request) {
	id, action := runtimeIntakeWorkflowPath(r)
	if id == "" || action != "" {
		writeError(w, http.StatusNotFound, errors.New("intake workflow not found"))
		return
	}
	workflow, ok, err := s.runtime.GetIntakeWorkflow(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("intake workflow not found"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workflow": workflow})
}

func (s *Server) runtimeIntakeWorkflowAction(w http.ResponseWriter, r *http.Request) {
	id, action := runtimeIntakeWorkflowPath(r)
	if id == "" || action == "" {
		writeError(w, http.StatusNotFound, errors.New("intake workflow action not found"))
		return
	}
	var payload struct {
		By   string `json:"by"`
		Note string `json:"note"`
	}
	_ = json.NewDecoder(r.Body).Decode(&payload)
	if strings.TrimSpace(payload.By) == "" {
		payload.By = "local-operator"
	}
	var (
		workflow any
		err      error
	)
	switch action {
	case "approve":
		workflow, err = s.runtime.ApproveIntakeWorkflow(r.Context(), id, payload.By, payload.Note)
	case "register":
		workflow, err = s.runtime.RegisterIntakeWorkflow(r.Context(), id, payload.By)
	default:
		writeError(w, http.StatusNotFound, errors.New("intake workflow action not found"))
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workflow": workflow})
}

func (s *Server) runtimeModelJobDetail(w http.ResponseWriter, r *http.Request) {
	id, action := runtimeModelJobPath(r)
	if id == "" {
		writeError(w, http.StatusNotFound, errors.New("model job not found"))
		return
	}
	switch action {
	case "":
	case "logs":
		s.runtimeModelJobLogs(w, r, id)
		return
	case "logs/stream":
		s.runtimeModelJobLogStream(w, r, id)
		return
	default:
		writeError(w, http.StatusNotFound, errors.New("model job not found"))
		return
	}
	job, ok := s.runtime.GetModelJob(id)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("model job not found"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"job": job})
}

func (s *Server) runtimeModelJobLogs(w http.ResponseWriter, r *http.Request, id string) {
	job, ok := s.runtime.GetModelJob(id)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("model job not found"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"job_id":           job.ID,
		"status":           job.Status,
		"progress_percent": job.ProgressPercent,
		"retryable":        job.Retryable,
		"attempt":          job.Attempt,
		"max_attempts":     job.MaxAttempts,
		"worker_heartbeat": job.WorkerHeartbeat,
		"artifacts":        job.Artifacts,
		"stdout":           job.Stdout,
		"stderr":           job.Stderr,
		"metadata":         job.Metadata,
		"logs":             agentruntime.RecentModelJobLogs(job, runtimeTraceLimit(r)),
	})
}

func (s *Server) runtimeModelJobLogStream(w http.ResponseWriter, r *http.Request, id string) {
	job, ok := s.runtime.GetModelJob(id)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("model job not found"))
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
	for _, log := range agentruntime.RecentModelJobLogs(job, runtimeTraceLimit(r)) {
		emit(map[string]any{"type": "log", "job_id": id, "log": log})
	}
	if agentruntime.IsTerminalModelJobStatus(job.Status) {
		emit(runtimeModelJobFinalEvent(job))
		return
	}
	timeout := time.NewTimer(runtimeStreamTimeout(r))
	defer timeout.Stop()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	lastCount := len(job.Logs)
	for {
		select {
		case <-r.Context().Done():
			return
		case <-timeout.C:
			latest, ok := s.runtime.GetModelJob(id)
			if ok {
				final := runtimeModelJobFinalEvent(latest)
				final["message"] = "stream timeout"
				emit(final)
			}
			return
		case <-ticker.C:
			latest, ok := s.runtime.GetModelJob(id)
			if !ok {
				emit(map[string]any{"type": "error", "job_id": id, "message": "model job not found"})
				return
			}
			if len(latest.Logs) > lastCount {
				for _, log := range latest.Logs[lastCount:] {
					emit(map[string]any{"type": "log", "job_id": id, "log": log})
				}
				lastCount = len(latest.Logs)
			}
			if agentruntime.IsTerminalModelJobStatus(latest.Status) {
				emit(runtimeModelJobFinalEvent(latest))
				return
			}
		}
	}
}

func runtimeModelJobFinalEvent(job agentruntime.ModelJob) map[string]any {
	return map[string]any{
		"type":             "final",
		"job_id":           job.ID,
		"status":           job.Status,
		"progress_percent": job.ProgressPercent,
		"message":          job.Message,
		"retryable":        job.Retryable,
		"attempt":          job.Attempt,
		"max_attempts":     job.MaxAttempts,
		"worker_heartbeat": job.WorkerHeartbeat,
		"artifacts":        job.Artifacts,
		"stdout":           job.Stdout,
		"stderr":           job.Stderr,
		"metadata":         job.Metadata,
	}
}

func (s *Server) runtimeModelJobAction(w http.ResponseWriter, r *http.Request) {
	id, action := runtimeModelJobPath(r)
	if id == "" || action == "" {
		writeError(w, http.StatusNotFound, errors.New("model job action not found"))
		return
	}
	var (
		job agentruntime.ModelJob
		err error
	)
	switch action {
	case "cancel":
		job, err = s.runtime.CancelModelJob(id)
	case "resume":
		job, err = s.runtime.ResumeModelJob(id)
	default:
		writeError(w, http.StatusNotFound, errors.New("model job action not found"))
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"job": job})
}

func (s *Server) desktopStatus(w http.ResponseWriter, r *http.Request) {
	status := agentruntime.Status()
	writeJSON(w, http.StatusOK, map[string]any{
		"desktop": map[string]any{
			"status":       "ready",
			"profile":      "local-desktop",
			"gateway":      r.Host,
			"runtime":      status.Runtime,
			"entry_points": status.EntryPoints,
			"snapshot":     s.runtime.Snapshot(runtimeTraceLimit(r)),
			"auth":         middleware.GatewayAuthStatusFromEnv(),
		},
	})
}

func runtimeModelJobPath(r *http.Request) (string, string) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/runtime/model-jobs/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], strings.Join(parts[1:], "/")
}

func runtimeIntakeWorkflowPath(r *http.Request) (string, string) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/runtime/intake/workflows/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

func runtimeStreamTimeout(r *http.Request) time.Duration {
	raw := r.URL.Query().Get("timeout_ms")
	if raw == "" {
		return 30 * time.Second
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 30 * time.Second
	}
	if value > 60000 {
		value = 60000
	}
	return time.Duration(value) * time.Millisecond
}

func runtimeTraceLimit(r *http.Request) int {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return 100
	}
	limit, err := strconv.Atoi(raw)
	if err != nil {
		return 100
	}
	return limit
}
