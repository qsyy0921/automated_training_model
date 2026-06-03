package queue

import (
	"context"
	"path/filepath"
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
