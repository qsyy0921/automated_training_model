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
		Metadata:        map[string]string{"artifact_manifest": "F:\\automated_training_model\\data_lake\\runtime\\artifacts\\job1.artifact_manifest.json"},
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
	if !strings.Contains(rec.Body.String(), `"worker_heartbeat"`) || !strings.Contains(rec.Body.String(), `"artifact://dry-run/job1"`) || !strings.Contains(rec.Body.String(), `"stdout":"{\"status\":\"completed\"}"`) || !strings.Contains(rec.Body.String(), `"artifact_manifest"`) {
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

func TestRuntimeModelJobLogsEndpointReturnsFailedWorkerFields(t *testing.T) {
	job := agentruntime.ModelJob{
		ID:              "job-timeout",
		Status:          "failed",
		Message:         "python model worker timed out after 1s; stderr=worker still running",
		Error:           "python model worker timed out after 1s; stderr=worker still running",
		ProgressPercent: 15,
		Retryable:       true,
		Attempt:         2,
		MaxAttempts:     3,
		WorkerHeartbeat: &agentruntime.ModelJobHeartbeat{At: "2026-06-03T12:34:56Z", Status: "running", Message: "alive"},
		Stdout:          "{\"stage\":\"download\"}",
		Stderr:          "worker still running",
		Metadata:        map[string]string{"artifact_manifest": "F:\\automated_training_model\\data_lake\\runtime\\artifacts\\job-timeout.artifact_manifest.json"},
		Logs: []agentruntime.ModelJobLog{
			{At: time.Unix(1, 0), Level: "info", Message: "queued"},
			{At: time.Unix(2, 0), Level: "error", Message: "python model worker timed out after 1s; stderr=worker still running"},
		},
	}
	server := &Server{runtime: agentruntime.NewServiceWithRunner(fakeRuntimeRunner{job: job})}
	req := httptest.NewRequest(http.MethodGet, "/api/runtime/model-jobs/job-timeout/logs?limit=5", nil)
	rec := httptest.NewRecorder()
	server.runtimeModelJobDetail(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, fragment := range []string{`"job_id":"job-timeout"`, `"status":"failed"`, `"retryable":true`, `"attempt":2`, `"max_attempts":3`, `"worker_heartbeat"`, `"stdout":"{\"stage\":\"download\"}"`, `"stderr":"worker still running"`, `"artifact_manifest"`} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("expected fragment %q in body: %s", fragment, body)
		}
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
