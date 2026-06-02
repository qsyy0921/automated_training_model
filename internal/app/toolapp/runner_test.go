package toolapp

import (
	"context"
	"errors"
	"testing"
)

type runnerRequest struct {
	SessionKey string
}

func TestRunnerExecutesRegisteredHandlersAndMergesResults(t *testing.T) {
	runner := NewRunner[runnerRequest](DefaultCatalog(), nil)
	runner.Register("runtime.health", func(ctx context.Context, req runnerRequest, call ToolCall) (ExecutionResult, error) {
		return ExecutionResult{
			ReplyText: "pong",
			Status:    "ok",
			Metadata:  map[string]string{"session_key": req.SessionKey},
		}, nil
	})
	runner.Register("runtime.status", func(ctx context.Context, req runnerRequest, call ToolCall) (ExecutionResult, error) {
		return ExecutionResult{
			ReplyText: "ready",
			Status:    "planned",
			Metadata:  map[string]string{"runtime": "ready"},
		}, nil
	})

	result, err := runner.Execute(context.Background(), runnerRequest{SessionKey: "s1"}, []ToolCall{
		{ToolID: "runtime.health"},
		{ToolID: "runtime.status"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ReplyText != "pong\nready" {
		t.Fatalf("unexpected reply merge: %q", result.ReplyText)
	}
	if result.Status != "planned" {
		t.Fatalf("expected last non-empty status, got %s", result.Status)
	}
	if result.Metadata["session_key"] != "s1" || result.Metadata["runtime"] != "ready" {
		t.Fatalf("unexpected metadata merge: %+v", result.Metadata)
	}
}

func TestRunnerRunsPreflightBeforeHandler(t *testing.T) {
	called := false
	runner := NewRunner[runnerRequest](DefaultCatalog(), nil)
	runner.Register("model.verify_hf", func(ctx context.Context, req runnerRequest, call ToolCall) (ExecutionResult, error) {
		called = true
		return ExecutionResult{Status: "ok"}, nil
	})

	result, err := runner.Execute(context.Background(), runnerRequest{}, []ToolCall{{
		ToolID: "model.verify_hf",
		Params: map[string]string{"repo_id": "nvidia/LocateAnything-3B", "shell": "bad"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("handler should not run after preflight rejection")
	}
	if result.Status != "preflight_failed" {
		t.Fatalf("expected preflight_failed, got %+v", result)
	}
}

func TestRunnerRejectsRegisteredToolWithoutHandler(t *testing.T) {
	runner := NewRunner[runnerRequest](DefaultCatalog(), nil)
	result, err := runner.Execute(context.Background(), runnerRequest{}, []ToolCall{{ToolID: "runtime.status"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "unsupported_tool" || result.Metadata["reason"] != "handler_not_registered" {
		t.Fatalf("expected missing handler rejection, got %+v", result)
	}
}

func TestRunnerReturnsHandlerError(t *testing.T) {
	want := errors.New("boom")
	runner := NewRunner[runnerRequest](DefaultCatalog(), nil)
	runner.Register("runtime.health", func(ctx context.Context, req runnerRequest, call ToolCall) (ExecutionResult, error) {
		return ExecutionResult{}, want
	})

	_, err := runner.Execute(context.Background(), runnerRequest{}, []ToolCall{{ToolID: "runtime.health"}})
	if !errors.Is(err, want) {
		t.Fatalf("expected handler error, got %v", err)
	}
}
