package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

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

func (s *Server) runtimeModelJobDetail(w http.ResponseWriter, r *http.Request) {
	id, action := runtimeModelJobPath(r)
	if id == "" || action != "" {
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
			"status":       "scaffolded",
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
	return parts[0], parts[1]
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
