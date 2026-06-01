package httpapi

import (
	"net/http"
	"strconv"

	"github.com/qsyy0921/automated_training_model/internal/app/agentruntime"
)

func (s *Server) runtimeStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"runtime":  agentruntime.Status(),
		"snapshot": s.runtime.Snapshot(runtimeTraceLimit(r)),
	})
}

func (s *Server) runtimeSessions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"sessions": s.runtime.ListSessions()})
}

func (s *Server) runtimeTraces(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"traces": s.runtime.ListTraces(runtimeTraceLimit(r))})
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
		},
	})
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
