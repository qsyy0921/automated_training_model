package httpapi

import (
	"net/http"

	"github.com/qsyy0921/automated_training_model/internal/app/agentruntime"
)

func (s *Server) runtimeStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"runtime": agentruntime.Status()})
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
		},
	})
}
