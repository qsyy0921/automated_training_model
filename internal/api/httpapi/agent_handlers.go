package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/qsyy0921/automated_training_model/internal/domain/agent"
)

func (s *Server) listAgents(w http.ResponseWriter, r *http.Request) {
	rows, err := s.agents.ListAgents(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"agents": rows})
}

func (s *Server) agentDetail(w http.ResponseWriter, r *http.Request) {
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/agents/"), "/")
	row, err := s.agents.GetAgent(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"agent": row})
}

func (s *Server) saveAgent(w http.ResponseWriter, r *http.Request) {
	var req agent.AgentSpec
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	row, err := s.agents.SaveAgent(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"agent": row})
}

func (s *Server) listAgentTools(w http.ResponseWriter, r *http.Request) {
	rows, err := s.agents.ListTools(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tools": rows})
}

func (s *Server) saveAgentTool(w http.ResponseWriter, r *http.Request) {
	var req agent.ToolSpec
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	row, err := s.agents.SaveTool(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"tool": row})
}

func (s *Server) listAgentWorkflows(w http.ResponseWriter, r *http.Request) {
	rows, err := s.agents.ListWorkflows(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workflows": rows})
}

func (s *Server) agentWorkflowDetail(w http.ResponseWriter, r *http.Request) {
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/workflows/"), "/")
	row, err := s.agents.GetWorkflow(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workflow": row})
}

func (s *Server) saveAgentWorkflow(w http.ResponseWriter, r *http.Request) {
	var req agent.WorkflowSpec
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	row, err := s.agents.SaveWorkflow(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"workflow": row})
}

func (s *Server) submitAgentRun(w http.ResponseWriter, r *http.Request) {
	var req agent.RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	row, err := s.agents.SubmitWorkflowRun(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"run": row})
}

func (s *Server) listAgentRuns(w http.ResponseWriter, r *http.Request) {
	rows, err := s.agents.ListRuns(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": rows})
}

func (s *Server) listAuditEvents(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	rows, err := s.agents.ListAuditEvents(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": rows})
}

func (s *Server) listEnforcementPoints(w http.ResponseWriter, r *http.Request) {
	rows, err := s.agents.ListEnforcementPoints(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"enforcement_points": rows})
}

func (s *Server) listDataGovernancePolicies(w http.ResponseWriter, r *http.Request) {
	rows, err := s.agents.ListDataGovernancePolicies(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data_policies": rows})
}

func (s *Server) listReleasePolicies(w http.ResponseWriter, r *http.Request) {
	rows, err := s.agents.ListReleasePolicies(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"release_policies": rows})
}

func (s *Server) listRuntimePolicies(w http.ResponseWriter, r *http.Request) {
	rows, err := s.agents.ListRuntimePolicies(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runtime_policies": rows})
}

func (s *Server) getControlSurface(w http.ResponseWriter, r *http.Request) {
	row, err := s.agents.GetControlSurface(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"control_surface": row})
}
