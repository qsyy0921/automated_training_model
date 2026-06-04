package labelctl

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeRuntimeInputDropsPowerShellBOM(t *testing.T) {
	got := normalizeRuntimeInput("\ufeff\ufeff/help\r\n")
	if got != "/help" {
		t.Fatalf("normalizeRuntimeInput() = %q, want /help", got)
	}
}

func TestCompactMetadataHandlesNonStringValues(t *testing.T) {
	got := compactMetadata(map[string]any{
		"complete": true,
		"count":    float64(3),
	})
	if got == "" {
		t.Fatal("compactMetadata() returned empty string")
	}
}

func TestToolProgressLinePrefersSingleToolID(t *testing.T) {
	got := toolProgressLine(runtimeStreamEvent{
		ToolID:  "runtime.health",
		ToolIDs: []string{"runtime.status"},
		Status:  "ok",
		Message: "tool_done: tool handler completed",
	})
	want := "  • tool=runtime.health status=ok tool_done: tool handler completed"
	if got != want {
		t.Fatalf("toolProgressLine() = %q, want %q", got, want)
	}
}

func TestRuntimeStreamErrorMessagePrefersEnvelope(t *testing.T) {
	got := runtimeStreamErrorMessage(runtimeStreamEvent{
		Message: "plain error",
		ErrorEnvelope: &runtimeErrorEnvelope{
			Code:    "runtime.planning_failed",
			Message: "planner unavailable",
			Source:  "planner-agent",
		},
	})
	if got != "planner unavailable" {
		t.Fatalf("runtimeStreamErrorMessage() = %q", got)
	}
}

func TestModelJobLogLineFormatsLifecycleLog(t *testing.T) {
	got := modelJobLogLine(runtimeModelJobLog{
		At:      "2026-06-03T12:34:56Z",
		Level:   "info",
		Message: "queued by Agent Runtime",
	})
	want := "2026-06-03 12:34:56  info   queued by Agent Runtime"
	if got != want {
		t.Fatalf("modelJobLogLine() = %q, want %q", got, want)
	}
}

func TestRuntimeChatModelJobCommandsUseGateway(t *testing.T) {
	seen := map[string]bool{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen[r.URL.Path] = true
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/tasks":
			_, _ = w.Write([]byte(`{"tasks":[{"id":"task1","type":"training.run","status":"interrupted","progress_percent":0,"message":"server restarted before lifecycle task completed","resumable":true,"attempt":1,"max_attempts":1}]}`))
		case "/api/tasks/task1":
			_, _ = w.Write([]byte(`{"task":{"id":"task1","type":"training.run","status":"interrupted","progress_percent":0,"message":"server restarted before lifecycle task completed","resumable":true,"retryable":true,"attempt":1,"max_attempts":1,"updated_at":"2026-06-03T12:34:56Z"}}`))
		case "/api/tasks/task1/logs":
			_, _ = w.Write([]byte(`{"task_id":"task1","type":"training.run","status":"interrupted","progress_percent":0,"resumable":true,"retryable":true,"attempt":1,"max_attempts":1,"worker_heartbeat":{"at":"2026-06-03T12:34:56Z","status":"interrupted","message":"restart"},"artifacts":[{"name":"result","uri":"artifact://training/task1","kind":"training.run.result"}],"stdout":"{\"status\":\"interrupted\"}","metadata":{"artifact_manifest":"F:\\automated_training_model\\data_lake\\runtime\\artifacts\\task1.artifact_manifest.json"},"logs":[{"at":"2026-06-03T12:34:56Z","level":"warn","message":"interrupted"}]}`))
		case "/api/tasks/task1/manifest":
			_, _ = w.Write([]byte(`{"path":"F:\\automated_training_model\\data_lake\\runtime\\artifacts\\task1.artifact_manifest.json","manifest":{"schema_version":"artifact-manifest/v1","artifact_summary":{"artifact_count":1,"role_counts":{"result":1},"execution_mode_counts":{"recipe-executed":1},"primary_artifact":{"name":"result","uri":"artifact://training/task1","role":"result","execution_mode":"recipe-executed"}},"artifacts":[{"name":"result","uri":"artifact://training/task1","kind":"training.run.result"}]}}`))
		case "/api/tasks/task1/lineage":
			_, _ = w.Write([]byte(`{"task_id":"task1","root_id":"task1","count":2,"lineage":[{"id":"task1","type":"training.run","status":"interrupted","progress_percent":0},{"id":"task2","parent_id":"task1","type":"training.run","status":"pending","progress_percent":0}]}`))
		case "/api/tasks/task1/logs/stream":
			_, _ = w.Write([]byte(`{"type":"log","task_id":"task1","log":{"at":"2026-06-03T12:34:56Z","level":"info","message":"running"}}` + "\n"))
			_, _ = w.Write([]byte(`{"type":"update","task_id":"task1","status":"running","progress_percent":55,"message":"worker progressing","retryable":true,"attempt":1,"max_attempts":1,"worker_heartbeat":{"at":"2026-06-03T12:34:56Z","status":"running","message":"alive"},"stdout":"{\"status\":\"running\"}"}` + "\n"))
			_, _ = w.Write([]byte(`{"type":"final","task_id":"task1","status":"completed","progress_percent":100,"message":"done","retryable":false,"attempt":1,"max_attempts":1,"worker_heartbeat":{"at":"2026-06-03T12:34:57Z","status":"completed","message":"done"},"artifacts":[{"name":"result","uri":"artifact://training/task1","kind":"training.run.result"}],"stdout":"{\"status\":\"completed\"}","metadata":{"artifact_manifest":"F:\\automated_training_model\\data_lake\\runtime\\artifacts\\task1.artifact_manifest.json"}}` + "\n"))
		case "/api/tasks/task1/resume":
			_, _ = w.Write([]byte(`{"task":{"id":"task2","parent_id":"task1","type":"training.run","status":"pending","message":"queued python worker dry-run","resumable":false}}`))
		case "/api/runtime/model-jobs/job1":
			_, _ = w.Write([]byte(`{"job":{"id":"job1","repo_id":"nvidia/LocateAnything-3B","kind":"model.download_hf","status":"running","progress_percent":25,"message":"running","retryable":true,"attempt":1,"max_attempts":3,"updated_at":"2026-06-03T12:34:56Z"}}`))
		case "/api/runtime/model-jobs/job1/logs":
			_, _ = w.Write([]byte(`{"job_id":"job1","status":"running","progress_percent":25,"retryable":true,"attempt":1,"max_attempts":3,"worker_heartbeat":{"at":"2026-06-03T12:34:56Z","status":"running","message":"alive"},"artifacts":[{"name":"plan","uri":"artifact://dry-run/job1","kind":"dry-run-plan"}],"stdout":"{\"status\":\"running\"}","metadata":{"artifact_manifest":"F:\\automated_training_model\\data_lake\\runtime\\artifacts\\job1.artifact_manifest.json"},"logs":[{"at":"2026-06-03T12:34:56Z","level":"info","message":"running"}]}`))
		case "/api/runtime/model-jobs/job1/manifest":
			_, _ = w.Write([]byte(`{"path":"F:\\automated_training_model\\data_lake\\runtime\\artifacts\\job1.artifact_manifest.json","manifest":{"schema_version":"artifact-manifest/v1","artifact_summary":{"artifact_count":1,"role_counts":{"plan":1},"primary_artifact":{"name":"plan","uri":"artifact://dry-run/job1","role":"plan"}},"artifacts":[{"name":"plan","uri":"artifact://dry-run/job1","kind":"dry-run-plan"}]}}`))
		case "/api/runtime/model-jobs/job1/lineage":
			_, _ = w.Write([]byte(`{"job_id":"job1","root_id":"job1","count":2,"lineage":[{"id":"job1","kind":"model.download_hf","repo_id":"nvidia/LocateAnything-3B","status":"running","progress_percent":25},{"id":"job2","parent_id":"job1","kind":"model.download_hf","repo_id":"nvidia/LocateAnything-3B","status":"queued","progress_percent":0}]}`))
		case "/api/runtime/model-jobs/job1/logs/stream":
			_, _ = w.Write([]byte(`{"type":"log","job_id":"job1","log":{"at":"2026-06-03T12:34:56Z","level":"info","message":"running"}}` + "\n"))
			_, _ = w.Write([]byte(`{"type":"update","job_id":"job1","status":"running","progress_percent":60,"message":"worker progressing","retryable":true,"attempt":1,"max_attempts":3,"worker_heartbeat":{"at":"2026-06-03T12:34:56Z","status":"running","message":"alive"},"stdout":"{\"status\":\"running\"}"}` + "\n"))
			_, _ = w.Write([]byte(`{"type":"final","job_id":"job1","status":"succeeded","progress_percent":100,"message":"done","retryable":false,"attempt":1,"max_attempts":3,"worker_heartbeat":{"at":"2026-06-03T12:34:57Z","status":"completed","message":"done"},"artifacts":[{"name":"plan","uri":"artifact://dry-run/job1","kind":"dry-run-plan"}],"stdout":"{\"status\":\"completed\"}","metadata":{"artifact_manifest":"F:\\automated_training_model\\data_lake\\runtime\\artifacts\\job1.artifact_manifest.json"}}` + "\n"))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	var out bytes.Buffer
	chat := newRuntimeChat(Config{addr: server.URL}, strings.NewReader(""), &out, &out)
	chat.noColor = true
	for _, input := range []string{"/tasks", "/task task1", "/task-logs task1", "/task-manifest task1", "/task-lineage task1", "/resume-task task1", "/follow-task task1", "/job job1", "/job-logs job1", "/job-manifest job1", "/job-lineage job1", "/follow-job job1"} {
		handled, err := chat.handleCommand(input)
		if err != nil {
			t.Fatalf("%s failed: %v", input, err)
		}
		if !handled {
			t.Fatalf("%s was not handled", input)
		}
	}
	for _, path := range []string{"/api/tasks", "/api/tasks/task1", "/api/tasks/task1/logs", "/api/tasks/task1/manifest", "/api/tasks/task1/lineage", "/api/tasks/task1/resume", "/api/tasks/task1/logs/stream", "/api/runtime/model-jobs/job1", "/api/runtime/model-jobs/job1/logs", "/api/runtime/model-jobs/job1/manifest", "/api/runtime/model-jobs/job1/lineage", "/api/runtime/model-jobs/job1/logs/stream"} {
		if !seen[path] {
			t.Fatalf("expected request to %s", path)
		}
	}
	if !strings.Contains(out.String(), "Lifecycle Task Logs") || !strings.Contains(out.String(), "Lifecycle Task Artifact Manifest") || !strings.Contains(out.String(), "Lifecycle Task Lineage") || !strings.Contains(out.String(), "Model Job Lineage") || !strings.Contains(out.String(), "Lifecycle Task Resumed") || !strings.Contains(out.String(), "resumable=true") || !strings.Contains(out.String(), "roles     result=1") || !strings.Contains(out.String(), "Following Lifecycle Task") || !strings.Contains(out.String(), "artifact://training/task1") || !strings.Contains(out.String(), "Model Job Logs") || !strings.Contains(out.String(), "Model Job Artifact Manifest") || !strings.Contains(out.String(), "update status=running progress=55%") || !strings.Contains(out.String(), "update status=running progress=60%") || !strings.Contains(out.String(), "final status=succeeded") || !strings.Contains(out.String(), "artifact://dry-run/job1") || !strings.Contains(out.String(), "attempt=1/3") || !strings.Contains(out.String(), "manifest  F:\\") || !strings.Contains(out.String(), "stdout    {\"status\":\"completed\"}") {
		t.Fatalf("unexpected output:\n%s", out.String())
	}
}
