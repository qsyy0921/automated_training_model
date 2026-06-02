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
		case "/api/runtime/model-jobs/job1":
			_, _ = w.Write([]byte(`{"job":{"id":"job1","repo_id":"nvidia/LocateAnything-3B","kind":"model.download_hf","status":"running","progress_percent":25,"message":"running","updated_at":"2026-06-03T12:34:56Z"}}`))
		case "/api/runtime/model-jobs/job1/logs":
			_, _ = w.Write([]byte(`{"job_id":"job1","status":"running","progress_percent":25,"logs":[{"at":"2026-06-03T12:34:56Z","level":"info","message":"running"}]}`))
		case "/api/runtime/model-jobs/job1/logs/stream":
			_, _ = w.Write([]byte(`{"type":"log","job_id":"job1","log":{"at":"2026-06-03T12:34:56Z","level":"info","message":"running"}}` + "\n"))
			_, _ = w.Write([]byte(`{"type":"final","job_id":"job1","status":"succeeded","progress_percent":100,"message":"done"}` + "\n"))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	var out bytes.Buffer
	chat := newRuntimeChat(Config{addr: server.URL}, strings.NewReader(""), &out, &out)
	chat.noColor = true
	for _, input := range []string{"/job job1", "/job-logs job1", "/follow-job job1"} {
		handled, err := chat.handleCommand(input)
		if err != nil {
			t.Fatalf("%s failed: %v", input, err)
		}
		if !handled {
			t.Fatalf("%s was not handled", input)
		}
	}
	for _, path := range []string{"/api/runtime/model-jobs/job1", "/api/runtime/model-jobs/job1/logs", "/api/runtime/model-jobs/job1/logs/stream"} {
		if !seen[path] {
			t.Fatalf("expected request to %s", path)
		}
	}
	if !strings.Contains(out.String(), "Model Job Logs") || !strings.Contains(out.String(), "final status=succeeded") {
		t.Fatalf("unexpected output:\n%s", out.String())
	}
}
