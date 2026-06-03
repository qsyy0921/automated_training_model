package modelgateway

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/app/modelruntime"
	"github.com/qsyy0921/automated_training_model/internal/domain/workflow"
	"github.com/qsyy0921/automated_training_model/internal/infrastructure/queue"
)

type fakeWorkerRunner struct {
	run func(context.Context, modelruntime.WorkerJobRequest, func(modelruntime.WorkerRuntimeEvent)) (modelruntime.WorkerJobResult, error)
}

func (f fakeWorkerRunner) Run(ctx context.Context, req modelruntime.WorkerJobRequest, emit func(modelruntime.WorkerRuntimeEvent)) (modelruntime.WorkerJobResult, error) {
	return f.run(ctx, req, emit)
}

func TestWorkerGatewayRunsTrainingTaskThroughPythonWorkerDryRun(t *testing.T) {
	q, err := queue.NewJSONQueue(filepath.Join(t.TempDir(), "tasks.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 6, 3, 15, 0, 0, 0, time.UTC)
	gateway := NewWorkerGatewayWithRunner(q, fakeWorkerRunner{
		run: func(ctx context.Context, req modelruntime.WorkerJobRequest, emit func(modelruntime.WorkerRuntimeEvent)) (modelruntime.WorkerJobResult, error) {
			if req.ToolID != "training.run" || !req.DryRun || req.DatasetID != "shanghaitech-original" {
				t.Fatalf("unexpected worker request: %+v", req)
			}
			emit(modelruntime.WorkerRuntimeEvent{Type: "heartbeat", At: "2026-06-03T15:00:01Z", Status: "running", Message: "worker booted"})
			emit(modelruntime.WorkerRuntimeEvent{Type: "stream", At: "2026-06-03T15:00:02Z", Stream: "stdout", Text: "planning recipe"})
			return modelruntime.WorkerJobResult{
				TaskID:      req.TaskID,
				Status:      "completed",
				Message:     "dry-run completed for training.run",
				Attempt:     1,
				MaxAttempts: 1,
				Retryable:   false,
				Heartbeat:   &modelruntime.WorkerHeartbeat{At: "2026-06-03T15:00:03Z", Status: "completed", Message: "done"},
				Logs:        []modelruntime.WorkerLog{{At: "2026-06-03T15:00:03Z", Level: "info", Message: "worker finished"}},
				Artifacts:   []modelruntime.WorkerArtifact{{Name: "plan", URI: "artifact://training/task_000001", Kind: "dry-run-plan"}},
				Stdout:      "planning recipe",
				StartedAt:   "2026-06-03T15:00:01Z",
				FinishedAt:  "2026-06-03T15:00:03Z",
			}, nil
		},
	}, func() time.Time {
		now = now.Add(time.Second)
		return now
	}, func() time.Duration { return time.Second })

	taskID, err := gateway.Submit(context.Background(), "training.run", map[string]string{
		"dataset_id":   "shanghaitech-original",
		"target_task":  "detection",
		"model_family": "yolo11n",
	})
	if err != nil {
		t.Fatal(err)
	}
	task := waitForTask(t, gateway, taskID, func(task *workflow.Task) bool {
		return task.Status == workflow.TaskCompleted
	})
	if task.ProgressPercent != 100 || task.Message == "" {
		t.Fatalf("unexpected completed task: %+v", task)
	}
	if task.WorkerHeartbeat == nil || task.WorkerHeartbeat.Status != "completed" {
		t.Fatalf("expected completed heartbeat, got %+v", task.WorkerHeartbeat)
	}
	if len(task.Artifacts) != 1 || task.Artifacts[0].URI != "artifact://training/task_000001" {
		t.Fatalf("unexpected artifacts: %+v", task.Artifacts)
	}
	if task.Stdout == "" || len(task.Logs) < 3 {
		t.Fatalf("expected stdout and logs, got %+v", task)
	}
	if task.Metadata["execution_path"] != "python-worker" || task.Metadata["worker_finished_at"] == "" {
		t.Fatalf("unexpected metadata: %+v", task.Metadata)
	}
	if task.Metadata["artifact_manifest"] == "" {
		t.Fatalf("expected artifact manifest metadata, got %+v", task.Metadata)
	}
}

func TestWorkerGatewayRunsDeploymentTaskThroughPythonWorkerExecution(t *testing.T) {
	q, err := queue.NewJSONQueue(filepath.Join(t.TempDir(), "tasks.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	gateway := NewWorkerGatewayWithRunner(q, fakeWorkerRunner{
		run: func(ctx context.Context, req modelruntime.WorkerJobRequest, emit func(modelruntime.WorkerRuntimeEvent)) (modelruntime.WorkerJobResult, error) {
			if req.ToolID != "deployment.run" || req.DryRun {
				t.Fatalf("unexpected worker request: %+v", req)
			}
			if req.Params["artifact_root"] == "" {
				t.Fatalf("expected default artifact_root injection, got %+v", req.Params)
			}
			if req.Params["dry_run"] != "false" {
				t.Fatalf("expected false dry_run param, got %+v", req.Params)
			}
			return modelruntime.WorkerJobResult{
				TaskID:      req.TaskID,
				Status:      "completed",
				Message:     "deployment execution bundle materialized: model=model-1 target=local runtime=python-worker",
				Attempt:     1,
				MaxAttempts: 1,
				Retryable:   false,
				Heartbeat:   &modelruntime.WorkerHeartbeat{At: "2026-06-03T15:00:03Z", Status: "completed", Message: "done"},
				Artifacts:   []modelruntime.WorkerArtifact{{Name: "result", URI: "F:/automated_training_model/data_lake/runtime/lifecycle/deployment.run/task_000001/result.json", Kind: "deployment.run.result"}},
				StartedAt:   "2026-06-03T15:00:01Z",
				FinishedAt:  "2026-06-03T15:00:03Z",
			}, nil
		},
	}, time.Now, func() time.Duration { return time.Second })

	taskID, err := gateway.Submit(context.Background(), "deployment.run", map[string]string{
		"model_id": "model-1",
		"target":   "local",
		"runtime":  "python-worker",
		"dry_run":  "false",
	})
	if err != nil {
		t.Fatal(err)
	}
	task := waitForTask(t, gateway, taskID, func(task *workflow.Task) bool {
		return task.Status == workflow.TaskCompleted
	})
	if task.Message == "" || task.Metadata["dry_run"] != "false" {
		t.Fatalf("unexpected task state: %+v", task)
	}
	if task.Metadata["artifact_manifest"] == "" {
		t.Fatalf("expected artifact manifest metadata, got %+v", task.Metadata)
	}
	if len(task.Artifacts) != 1 || task.Artifacts[0].Kind != "deployment.run.result" {
		t.Fatalf("unexpected artifacts: %+v", task.Artifacts)
	}
}

func TestWorkerGatewayCancelRunningTask(t *testing.T) {
	q := queue.NewMemoryQueue()
	started := make(chan struct{}, 1)
	gateway := NewWorkerGatewayWithRunner(q, fakeWorkerRunner{
		run: func(ctx context.Context, req modelruntime.WorkerJobRequest, emit func(modelruntime.WorkerRuntimeEvent)) (modelruntime.WorkerJobResult, error) {
			emit(modelruntime.WorkerRuntimeEvent{Type: "heartbeat", At: "2026-06-03T15:00:01Z", Status: "running", Message: "worker booted"})
			started <- struct{}{}
			<-ctx.Done()
			return modelruntime.WorkerJobResult{
				TaskID:    req.TaskID,
				Status:    "failed",
				Message:   "context canceled",
				Retryable: true,
				Stderr:    "canceled",
			}, ctx.Err()
		},
	}, time.Now, func() time.Duration { return time.Second })

	taskID, err := gateway.Submit(context.Background(), "evaluation.run", map[string]string{"dataset_id": "shanghaitech-original", "model_id": "model-1"})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("worker did not start")
	}
	if err := gateway.Cancel(context.Background(), taskID); err != nil {
		t.Fatal(err)
	}
	task := waitForTask(t, gateway, taskID, func(task *workflow.Task) bool {
		return task.Status == workflow.TaskCanceled && task.FinishedAt != nil
	})
	if task.Message != "python worker canceled" {
		t.Fatalf("unexpected canceled task: %+v", task)
	}
}

func TestWorkerGatewayResumeInterruptedTask(t *testing.T) {
	q, err := queue.NewJSONQueue(filepath.Join(t.TempDir(), "tasks.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	id, err := q.Enqueue(context.Background(), workflow.TaskSpec{Type: "training.run", Payload: map[string]string{"dataset_id": "shanghaitech-original", "dry_run": "true"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.Update(context.Background(), id, func(task *workflow.Task) {
		task.Status = workflow.TaskInterrupted
		task.Resumable = true
		task.Message = "server restarted before lifecycle task completed"
	}); err != nil {
		t.Fatal(err)
	}
	gateway := NewWorkerGatewayWithRunner(q, fakeWorkerRunner{
		run: func(ctx context.Context, req modelruntime.WorkerJobRequest, emit func(modelruntime.WorkerRuntimeEvent)) (modelruntime.WorkerJobResult, error) {
			return modelruntime.WorkerJobResult{
				TaskID:      req.TaskID,
				Status:      "completed",
				Message:     "dry-run completed for training.run",
				Attempt:     1,
				MaxAttempts: 1,
				Retryable:   false,
			}, nil
		},
	}, time.Now, func() time.Duration { return time.Second })
	newID, err := gateway.Resume(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if newID == id {
		t.Fatalf("expected new task id, got %s", newID)
	}
	prev, err := gateway.Status(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if prev.Metadata["resumed_by_task_id"] != newID || prev.Resumable {
		t.Fatalf("unexpected previous task state: %+v", prev)
	}
	next := waitForTask(t, gateway, newID, func(task *workflow.Task) bool {
		return task.Status == workflow.TaskCompleted
	})
	if next.ParentID != id || next.Metadata["resumed_from_task_id"] != id {
		t.Fatalf("unexpected resumed task linkage: %+v", next)
	}
}

func waitForTask(t *testing.T, gateway *WorkerGateway, id string, done func(*workflow.Task) bool) *workflow.Task {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		task, err := gateway.Status(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if done(task) {
			return task
		}
		time.Sleep(10 * time.Millisecond)
	}
	task, err := gateway.Status(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	t.Fatalf("task %s did not reach target state, got %+v", id, task)
	return nil
}
