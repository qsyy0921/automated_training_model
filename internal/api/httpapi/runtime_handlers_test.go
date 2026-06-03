package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/app/agentruntime"
	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

type fakeRuntimeRunner struct {
	job agentruntime.ModelJob
}

func (f fakeRuntimeRunner) Run(ctx context.Context, msg channel.InboundMessage) (channel.OutboundMessage, error) {
	return channel.OutboundMessage{Text: "ok"}, nil
}

func (f fakeRuntimeRunner) GetModelJob(id string) (agentruntime.ModelJob, bool) {
	if id != f.job.ID {
		return agentruntime.ModelJob{}, false
	}
	return f.job, true
}

func TestRuntimeModelJobLogsEndpoints(t *testing.T) {
	job := agentruntime.ModelJob{
		ID:              "job1",
		Status:          "succeeded",
		Message:         "done",
		ProgressPercent: 100,
		Retryable:       false,
		Attempt:         1,
		MaxAttempts:     2,
		WorkerHeartbeat: &agentruntime.ModelJobHeartbeat{At: "2026-06-03T12:34:56Z", Status: "completed", Message: "done"},
		Artifacts:       []agentruntime.ModelJobArtifact{{Name: "plan", URI: "artifact://dry-run/job1", Kind: "dry-run-plan"}},
		Stdout:          "{\"status\":\"completed\"}",
		Logs: []agentruntime.ModelJobLog{
			{At: time.Unix(1, 0), Level: "info", Message: "queued"},
			{At: time.Unix(2, 0), Level: "info", Message: "done"},
		},
	}
	server := &Server{runtime: agentruntime.NewServiceWithRunner(fakeRuntimeRunner{job: job})}

	req := httptest.NewRequest(http.MethodGet, "/api/runtime/model-jobs/job1/logs?limit=1", nil)
	rec := httptest.NewRecorder()
	server.runtimeModelJobDetail(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"job_id":"job1"`) || !strings.Contains(rec.Body.String(), `"done"`) || strings.Contains(rec.Body.String(), `"queued"`) {
		t.Fatalf("unexpected logs response: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"worker_heartbeat"`) || !strings.Contains(rec.Body.String(), `"artifact://dry-run/job1"`) || !strings.Contains(rec.Body.String(), `"stdout":"{\"status\":\"completed\"}"`) {
		t.Fatalf("unexpected logs response: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/runtime/model-jobs/job1/logs/stream", nil)
	rec = httptest.NewRecorder()
	server.runtimeModelJobDetail(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected stream status: %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"type":"log"`) || !strings.Contains(body, `"type":"final"`) || !strings.Contains(body, `"status":"succeeded"`) {
		t.Fatalf("unexpected stream body: %s", body)
	}
}

func TestRuntimeModelJobNotFoundReturnsErrorEnvelope(t *testing.T) {
	server := &Server{runtime: agentruntime.NewServiceWithRunner(fakeRuntimeRunner{job: agentruntime.ModelJob{ID: "job1"}})}
	req := httptest.NewRequest(http.MethodGet, "/api/runtime/model-jobs/missing", nil)
	rec := httptest.NewRecorder()
	server.runtimeModelJobDetail(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Error         string                     `json:"error"`
		ErrorEnvelope agentruntime.ErrorEnvelope `json:"error_envelope"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("parse response: %v\n%s", err, rec.Body.String())
	}
	if payload.Error == "" || payload.ErrorEnvelope.Code != "gateway.not_found" || payload.ErrorEnvelope.Source != "gateway" {
		t.Fatalf("unexpected error envelope: %+v body=%s", payload, rec.Body.String())
	}
}
