package modelruntime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestPythonModelWorkerRunnerSuccess(t *testing.T) {
	runner := helperPythonModelWorkerRunner("success")
	result, err := runner.Run(context.Background(), WorkerJobRequest{TaskID: "job1", DryRun: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "completed" || result.Heartbeat == nil || result.Heartbeat.Status != "completed" {
		t.Fatalf("unexpected worker result: %+v", result)
	}
	if len(result.Artifacts) != 1 || result.Artifacts[0].URI != "artifact://dry-run/job1" {
		t.Fatalf("unexpected artifacts: %+v", result.Artifacts)
	}
}

func TestPythonModelWorkerRunnerFailedResult(t *testing.T) {
	runner := helperPythonModelWorkerRunner("failed")
	result, err := runner.Run(context.Background(), WorkerJobRequest{TaskID: "job1", DryRun: false}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "failed" || !result.Retryable || result.Message == "" {
		t.Fatalf("unexpected failed worker result: %+v", result)
	}
}

func TestPythonModelWorkerRunnerRejectsInvalidJSON(t *testing.T) {
	runner := helperPythonModelWorkerRunner("badjson")
	result, err := runner.Run(context.Background(), WorkerJobRequest{TaskID: "job1"}, nil)
	if err == nil || !containsAll(err.Error(), "decode python model worker result", "stdout") {
		t.Fatalf("expected decode error, got %v", err)
	}
	if result.Status != "failed" || result.Retryable || !containsAll(result.Stdout, "{") || !containsAll(result.Stderr, "bad json") {
		t.Fatalf("expected failed partial result with stdout/stderr, got %+v", result)
	}
}

func TestPythonModelWorkerRunnerTimeout(t *testing.T) {
	runner := helperPythonModelWorkerRunner("sleep")
	runner.timeout = func() time.Duration { return 50 * time.Millisecond }
	result, err := runner.Run(context.Background(), WorkerJobRequest{TaskID: "job1"}, nil)
	if err == nil || !containsAll(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got %v", err)
	}
	if result.Status != "failed" || !result.Retryable || !containsAll(result.Stderr, "worker still running") {
		t.Fatalf("expected retryable failed partial result, got %+v", result)
	}
}

func TestPythonModelWorkerRunnerEmitsStructuredRuntimeEvents(t *testing.T) {
	runner := helperPythonModelWorkerRunner("success")
	var events []WorkerRuntimeEvent
	result, err := runner.Run(context.Background(), WorkerJobRequest{TaskID: "job1", DryRun: true}, func(event WorkerRuntimeEvent) {
		events = append(events, event)
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "completed" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(events) < 2 {
		t.Fatalf("expected structured worker events, got %+v", events)
	}
	if events[0].Type != "heartbeat" || events[1].Type != "stream" {
		t.Fatalf("unexpected event sequence: %+v", events)
	}
}

func TestPythonModelWorkerRunnerHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_MODEL_WORKER_HELPER") != "1" {
		return
	}
	mode := os.Getenv("MODEL_WORKER_HELPER_MODE")
	switch mode {
	case "success":
		fmt.Fprintln(os.Stderr, `ATM_EVENT {"type":"heartbeat","at":"2026-06-03T00:00:00Z","status":"running","message":"worker started"}`)
		fmt.Fprintln(os.Stderr, `ATM_EVENT {"type":"stream","at":"2026-06-03T00:00:00Z","stream":"stdout","text":"download started"}`)
		fmt.Fprint(os.Stdout, `{"task_id":"job1","status":"completed","artifacts":[{"name":"plan","uri":"artifact://dry-run/job1","kind":"dry-run-plan"}],"logs":[{"at":"2026-06-03T00:00:00Z","level":"info","message":"ok"}],"heartbeat":{"at":"2026-06-03T00:00:01Z","status":"completed","message":"done"},"attempt":1,"max_attempts":1,"retryable":false,"message":"dry-run completed","started_at":"2026-06-03T00:00:00Z","finished_at":"2026-06-03T00:00:01Z"}`)
		os.Exit(0)
	case "failed":
		fmt.Fprintln(os.Stderr, `ATM_EVENT {"type":"heartbeat","at":"2026-06-03T00:00:00Z","status":"running","message":"worker started"}`)
		fmt.Fprint(os.Stdout, `{"task_id":"job1","status":"failed","logs":[{"at":"2026-06-03T00:00:00Z","level":"error","message":"boom"}],"heartbeat":{"at":"2026-06-03T00:00:01Z","status":"failed","message":"boom"},"attempt":2,"max_attempts":3,"retryable":true,"message":"worker failed"}`)
		os.Exit(1)
	case "badjson":
		fmt.Fprint(os.Stdout, `{`)
		fmt.Fprint(os.Stderr, "bad json")
		os.Exit(1)
	case "sleep":
		fmt.Fprint(os.Stderr, "worker still running")
		time.Sleep(500 * time.Millisecond)
		fmt.Fprint(os.Stdout, `{"task_id":"job1","status":"completed"}`)
		os.Exit(0)
	default:
		fmt.Fprint(os.Stderr, "unknown helper mode")
		os.Exit(2)
	}
}

func helperPythonModelWorkerRunner(mode string) *PythonModelWorkerRunner {
	return &PythonModelWorkerRunner{
		python:     func() string { return os.Args[0] },
		pythonPath: func() string { return "" },
		timeout:    func() time.Duration { return time.Second },
		command: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			cmd := exec.CommandContext(ctx, name, "-test.run=TestPythonModelWorkerRunnerHelperProcess")
			cmd.Env = append(os.Environ(),
				"GO_WANT_MODEL_WORKER_HELPER=1",
				"MODEL_WORKER_HELPER_MODE="+mode,
			)
			return cmd
		},
	}
}

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}
