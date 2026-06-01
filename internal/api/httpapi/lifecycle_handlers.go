package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/qsyy0921/automated_training_model/internal/domain/autolabel"
	"github.com/qsyy0921/automated_training_model/internal/domain/deployment"
	"github.com/qsyy0921/automated_training_model/internal/domain/evaluation"
	"github.com/qsyy0921/automated_training_model/internal/domain/modelregistry"
	"github.com/qsyy0921/automated_training_model/internal/domain/training"
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

func (s *Server) taskStatus(w http.ResponseWriter, r *http.Request) {
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/tasks/"), "/")
	if id == "" {
		writeErrorText(w, http.StatusNotFound, "task id missing")
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
