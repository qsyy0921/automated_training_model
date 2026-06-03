package queue

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/domain/workflow"
)

func TestJSONQueuePersistsTasksAcrossReload(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "tasks.json")
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	q, err := NewJSONQueue(path, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	id, err := q.Enqueue(context.Background(), workflow.TaskSpec{
		Type:    "training.run",
		Payload: map[string]string{"dataset_id": "shanghaitech-original"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.Cancel(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	reloaded, err := NewJSONQueue(path, func() time.Time { return now.Add(time.Hour) })
	if err != nil {
		t.Fatal(err)
	}
	task, err := reloaded.Status(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if task.Type != "training.run" || task.Status != workflow.TaskCanceled {
		t.Fatalf("unexpected task after reload: %+v", task)
	}
	if task.Payload["dataset_id"] != "shanghaitech-original" {
		t.Fatalf("unexpected payload after reload: %+v", task.Payload)
	}
}

func TestJSONQueueWritesArtifactManifest(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "tasks.json")
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	q, err := NewJSONQueue(path, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	id, err := q.Enqueue(context.Background(), workflow.TaskSpec{Type: "training.run"})
	if err != nil {
		t.Fatal(err)
	}
	started := now.Add(time.Minute)
	finished := started.Add(time.Minute)
	if err := q.Update(context.Background(), id, func(task *workflow.Task) {
		task.Status = workflow.TaskCompleted
		task.Message = "dry-run complete"
		task.Retryable = false
		task.Attempt = 1
		task.MaxAttempts = 1
		task.WorkerHeartbeat = &workflow.TaskHeartbeat{At: finished.Format(time.RFC3339Nano), Status: "completed", Message: "done"}
		task.Artifacts = []workflow.TaskArtifact{
			{Name: "request", URI: "artifact://training/task_000001/request", Kind: "training.run.request", Metadata: map[string]string{"role": "request"}},
			{Name: "plan", URI: "artifact://training/task_000001/plan", Kind: "training.run.plan", Metadata: map[string]string{"role": "plan"}},
			{Name: "result", URI: "artifact://training/task_000001/result", Kind: "training.run.result", Metadata: map[string]string{"role": "result", "execution_mode": "recipe-executed"}},
		}
		task.Metadata = map[string]string{"worker_finished_at": finished.Format(time.RFC3339Nano)}
		task.StartedAt = &started
		task.FinishedAt = &finished
	}); err != nil {
		t.Fatal(err)
	}
	task, err := q.Status(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath, err := q.WriteArtifactManifest(*task)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(filepath.ToSlash(manifestPath), "/artifacts/task_000001.artifact_manifest.json") {
		t.Fatalf("unexpected manifest path: %s", manifestPath)
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("parse manifest: %v\n%s", err, string(data))
	}
	if payload["schema_version"] != "artifact-manifest/v1" || payload["task_id"] != "task_000001" || payload["status"] != string(workflow.TaskCompleted) {
		t.Fatalf("unexpected manifest payload: %s", string(data))
	}
	summary, ok := payload["artifact_summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected artifact_summary, got %s", string(data))
	}
	if summary["artifact_count"] != float64(3) {
		t.Fatalf("expected artifact_count=3, got %v", summary["artifact_count"])
	}
	roleCounts, ok := summary["role_counts"].(map[string]any)
	if !ok || roleCounts["request"] != float64(1) || roleCounts["plan"] != float64(1) || roleCounts["result"] != float64(1) {
		t.Fatalf("unexpected role_counts: %+v", summary["role_counts"])
	}
	modeCounts, ok := summary["execution_mode_counts"].(map[string]any)
	if !ok || modeCounts["recipe-executed"] != float64(1) {
		t.Fatalf("unexpected execution_mode_counts: %+v", summary["execution_mode_counts"])
	}
}

func TestJSONQueueContinuesTaskSequenceAfterReload(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "tasks.json")
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	q, err := NewJSONQueue(path, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	first, err := q.Enqueue(context.Background(), workflow.TaskSpec{Type: "training.run"})
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := NewJSONQueue(path, func() time.Time { return now.Add(time.Hour) })
	if err != nil {
		t.Fatal(err)
	}
	second, err := reloaded.Enqueue(context.Background(), workflow.TaskSpec{Type: "evaluation.run"})
	if err != nil {
		t.Fatal(err)
	}
	if first == second || second != "task_000002" {
		t.Fatalf("unexpected task sequence: first=%s second=%s", first, second)
	}
}

func TestJSONQueueMarksRunningTasksInterruptedOnReload(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "tasks.json")
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	q, err := NewJSONQueue(path, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	id, err := q.Enqueue(context.Background(), workflow.TaskSpec{Type: "training.run", Payload: map[string]string{"dataset_id": "ds1"}})
	if err != nil {
		t.Fatal(err)
	}
	started := now.Add(time.Minute)
	if err := q.Update(context.Background(), id, func(task *workflow.Task) {
		task.Status = workflow.TaskRunning
		task.ProgressPercent = 40
		task.StartedAt = &started
	}); err != nil {
		t.Fatal(err)
	}
	restartedAt := now.Add(2 * time.Hour)
	reloaded, err := NewJSONQueue(path, func() time.Time { return restartedAt })
	if err != nil {
		t.Fatal(err)
	}
	task, err := reloaded.Status(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != workflow.TaskInterrupted || !task.Resumable || !task.Retryable {
		t.Fatalf("expected interrupted resumable task, got %+v", task)
	}
	if task.FinishedAt == nil || !task.FinishedAt.Equal(restartedAt) {
		t.Fatalf("expected finished_at set to restart time, got %+v", task)
	}
	if task.Metadata["interrupted_reason"] != "server_restart" {
		t.Fatalf("expected interrupted_reason metadata, got %+v", task.Metadata)
	}
}

func TestJSONQueueListReturnsLatestFirst(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "tasks.json")
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	q, err := NewJSONQueue(path, func() time.Time {
		now = now.Add(time.Second)
		return now
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := q.Enqueue(context.Background(), workflow.TaskSpec{Type: "training.run"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := q.Enqueue(context.Background(), workflow.TaskSpec{Type: "evaluation.run"})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.Update(context.Background(), first, func(task *workflow.Task) {
		task.Message = "updated later"
	}); err != nil {
		t.Fatal(err)
	}
	rows, err := q.List(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].ID != first || rows[1].ID != second {
		t.Fatalf("unexpected task order: %+v", rows)
	}
}

func TestJSONQueueLineageReturnsResumeFamily(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "tasks.json")
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	q, err := NewJSONQueue(path, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	first, err := q.Enqueue(context.Background(), workflow.TaskSpec{Type: "training.run"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := q.Enqueue(context.Background(), workflow.TaskSpec{Type: "training.run"})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.Update(context.Background(), second, func(task *workflow.Task) {
		task.ParentID = first
		task.Status = workflow.TaskPending
	}); err != nil {
		t.Fatal(err)
	}
	lineage, err := q.Lineage(context.Background(), second)
	if err != nil {
		t.Fatal(err)
	}
	if len(lineage) != 2 || lineage[0].ID != first || lineage[1].ParentID != first {
		t.Fatalf("unexpected lineage: %+v", lineage)
	}
}
