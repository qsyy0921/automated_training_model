package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/app/lifecycleapp"
	"github.com/qsyy0921/automated_training_model/internal/domain/workflow"
)

type fakeTaskGateway struct {
	task    *workflow.Task
	lineage []workflow.Task
}

func (f fakeTaskGateway) Submit(ctx context.Context, taskType string, payload map[string]string) (string, error) {
	return "task_000001", nil
}

func (f fakeTaskGateway) List(ctx context.Context, limit int) ([]workflow.Task, error) {
	if f.task == nil {
		return nil, nil
	}
	return []workflow.Task{*f.task}, nil
}

func (f fakeTaskGateway) Status(ctx context.Context, id string) (*workflow.Task, error) {
	if f.task == nil || f.task.ID != id {
		return nil, errors.New("task not found")
	}
	copied := *f.task
	return &copied, nil
}

func (f fakeTaskGateway) Lineage(ctx context.Context, id string) ([]workflow.Task, error) {
	if len(f.lineage) > 0 {
		return f.lineage, nil
	}
	if f.task == nil || f.task.ID != id {
		return nil, errors.New("task not found")
	}
	return []workflow.Task{*f.task}, nil
}

func (f fakeTaskGateway) Cancel(ctx context.Context, id string) error { return nil }

func (f fakeTaskGateway) Resume(ctx context.Context, id string) (string, error) {
	if f.task == nil {
		return "", errors.New("task not found")
	}
	return "task_000002", nil
}

func TestLifecycleTaskLogsEndpoints(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "task_000001.artifact_manifest.json")
	if err := os.WriteFile(manifestPath, []byte("{\n  \"schema_version\": \"artifact-manifest/v1\",\n  \"artifact_summary\": {\n    \"artifact_count\": 1,\n    \"primary_artifact\": {\"name\": \"plan\", \"uri\": \"artifact://training/task_000001\", \"role\": \"plan\"}\n  }\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	task := &workflow.Task{
		ID:              "task_000001",
		Type:            "training.run",
		Status:          workflow.TaskCompleted,
		Message:         "dry-run complete",
		ProgressPercent: 100,
		Attempt:         1,
		MaxAttempts:     1,
		WorkerHeartbeat: &workflow.TaskHeartbeat{At: "2026-06-03T12:34:56Z", Status: "completed", Message: "done"},
		Artifacts:       []workflow.TaskArtifact{{Name: "plan", URI: "artifact://training/task_000001", Kind: "dry-run-plan"}},
		Stdout:          "{\"status\":\"completed\"}",
		Metadata:        map[string]string{"artifact_manifest": manifestPath},
		Logs: []workflow.TaskLog{
			{At: time.Unix(1, 0), Level: "info", Message: "queued"},
			{At: time.Unix(2, 0), Level: "info", Message: "done"},
		},
	}
	server := &Server{lifecycle: lifecycleapp.NewService(fakeTaskGateway{task: task})}

	req := httptest.NewRequest(http.MethodGet, "/api/tasks/task_000001/logs?limit=1", nil)
	rec := httptest.NewRecorder()
	server.taskDetail(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"task_id":"task_000001"`) || !strings.Contains(body, `"status":"completed"`) || strings.Contains(body, `"queued"`) {
		t.Fatalf("unexpected logs response: %s", body)
	}
	for _, fragment := range []string{`"worker_heartbeat"`, `"artifact://training/task_000001"`, `"stdout":"{\"status\":\"completed\"}"`, `"artifact_manifest"`} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("expected fragment %q in body: %s", fragment, body)
		}
	}

	req = httptest.NewRequest(http.MethodGet, "/api/tasks/task_000001/logs/stream", nil)
	rec = httptest.NewRecorder()
	server.taskDetail(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected stream status: %d body=%s", rec.Code, rec.Body.String())
	}
	stream := rec.Body.String()
	if !strings.Contains(stream, `"type":"log"`) || !strings.Contains(stream, `"type":"final"`) || !strings.Contains(stream, `"status":"completed"`) || !strings.Contains(stream, `"artifact_manifest"`) {
		t.Fatalf("unexpected stream body: %s", stream)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/tasks/task_000001/manifest", nil)
	rec = httptest.NewRecorder()
	server.taskDetail(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected manifest status: %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"schema_version":"artifact-manifest/v1"`) || !strings.Contains(rec.Body.String(), `"primary_artifact"`) || !strings.Contains(rec.Body.String(), `task_000001.artifact_manifest.json`) {
		t.Fatalf("unexpected manifest response: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/tasks/task_000001/lineage", nil)
	rec = httptest.NewRecorder()
	server.taskDetail(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected lineage status: %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"task_id":"task_000001"`) || !strings.Contains(rec.Body.String(), `"root_id":"task_000001"`) || !strings.Contains(rec.Body.String(), `"lineage"`) {
		t.Fatalf("unexpected lineage response: %s", rec.Body.String())
	}
}

func TestLifecycleTaskListEndpoint(t *testing.T) {
	task := &workflow.Task{
		ID:              "task_000001",
		Type:            "training.run",
		Status:          workflow.TaskRunning,
		Message:         "running python worker execution",
		ProgressPercent: 35,
	}
	server := &Server{lifecycle: lifecycleapp.NewService(fakeTaskGateway{task: task})}
	req := httptest.NewRequest(http.MethodGet, "/api/tasks?limit=10", nil)
	rec := httptest.NewRecorder()
	server.listTasks(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, fragment := range []string{`"tasks"`, `"task_000001"`, `"training.run"`, `"running"`} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("expected fragment %q in body: %s", fragment, body)
		}
	}
}

func TestLifecycleTaskNotFoundReturnsErrorEnvelope(t *testing.T) {
	server := &Server{lifecycle: lifecycleapp.NewService(fakeTaskGateway{})}
	req := httptest.NewRequest(http.MethodGet, "/api/tasks/missing", nil)
	rec := httptest.NewRecorder()
	server.taskDetail(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("parse response: %v\n%s", err, rec.Body.String())
	}
	if payload.Error == "" {
		t.Fatalf("missing error payload: %s", rec.Body.String())
	}
}

func TestLifecycleTaskResumeEndpoint(t *testing.T) {
	server := &Server{lifecycle: lifecycleapp.NewService(fakeTaskGateway{task: &workflow.Task{
		ID:        "task_000002",
		ParentID:  "task_000001",
		Type:      "training.run",
		Status:    workflow.TaskPending,
		Resumable: false,
	}})}
	req := httptest.NewRequest(http.MethodPost, "/api/tasks/task_000001/resume", nil)
	rec := httptest.NewRecorder()
	server.taskAction(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, fragment := range []string{`"task":{"id":"task_000002"`, `"parent_id":"task_000001"`, `"status":"pending"`} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("expected fragment %q in body: %s", fragment, body)
		}
	}
}

func TestLifecycleTaskLogStreamEmitsUpdateBeforeFinal(t *testing.T) {
	task := &workflow.Task{
		ID:              "task_stream",
		Type:            "training.run",
		Status:          workflow.TaskRunning,
		Message:         "running",
		ProgressPercent: 15,
		Retryable:       true,
		Attempt:         1,
		MaxAttempts:     2,
		WorkerHeartbeat: &workflow.TaskHeartbeat{At: "2026-06-03T12:34:56Z", Status: "running", Message: "alive"},
		Stdout:          "{\"status\":\"running\"}",
	}
	server := &Server{lifecycle: lifecycleapp.NewService(fakeTaskGateway{task: task})}
	go func() {
		time.Sleep(600 * time.Millisecond)
		task.ProgressPercent = 60
		task.Message = "worker progressing"
		task.Stdout = "{\"status\":\"progressing\"}"
		task.WorkerHeartbeat = &workflow.TaskHeartbeat{At: "2026-06-03T12:34:57Z", Status: "running", Message: "progress"}
		time.Sleep(600 * time.Millisecond)
		task.Status = workflow.TaskCompleted
		task.ProgressPercent = 100
		task.Message = "done"
		task.Stdout = "{\"status\":\"completed\"}"
		task.WorkerHeartbeat = &workflow.TaskHeartbeat{At: "2026-06-03T12:34:58Z", Status: "completed", Message: "done"}
	}()
	req := httptest.NewRequest(http.MethodGet, "/api/tasks/task_stream/logs/stream?timeout_ms=2500", nil)
	rec := httptest.NewRecorder()
	server.taskDetail(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected stream status: %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"type":"update"`) || !strings.Contains(body, `"progress_percent":60`) || !strings.Contains(body, `"type":"final"`) || !strings.Contains(body, `"status":"completed"`) {
		t.Fatalf("unexpected update stream body: %s", body)
	}
}
