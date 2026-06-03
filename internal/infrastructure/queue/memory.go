package queue

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/domain/workflow"
)

type MemoryQueue struct {
	mu    sync.Mutex
	next  int
	tasks map[string]*workflow.Task
}

func NewMemoryQueue() *MemoryQueue {
	return &MemoryQueue{tasks: map[string]*workflow.Task{}}
}

func (q *MemoryQueue) Enqueue(ctx context.Context, spec workflow.TaskSpec) (string, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.next++
	now := time.Now()
	id := fmt.Sprintf("task_%06d", q.next)
	q.tasks[id] = &workflow.Task{
		ID:        id,
		Type:      spec.Type,
		Status:    workflow.TaskPending,
		Payload:   copyStringMap(spec.Payload),
		CreatedAt: now,
		UpdatedAt: now,
	}
	return id, nil
}

func (q *MemoryQueue) Status(ctx context.Context, id string) (*workflow.Task, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	task := q.tasks[id]
	if task == nil {
		return nil, fmt.Errorf("task not found: %s", id)
	}
	copied := cloneTask(*task)
	return &copied, nil
}

func (q *MemoryQueue) Cancel(ctx context.Context, id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	task := q.tasks[id]
	if task == nil {
		return fmt.Errorf("task not found: %s", id)
	}
	if task.Status != workflow.TaskPending && task.Status != workflow.TaskRunning {
		return fmt.Errorf("task %s cannot be canceled from status %s", id, task.Status)
	}
	task.Status = workflow.TaskCanceled
	task.Message = "cancel requested"
	task.UpdatedAt = time.Now()
	return nil
}

func (q *MemoryQueue) Update(ctx context.Context, id string, mutate func(*workflow.Task)) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	task := q.tasks[id]
	if task == nil {
		return fmt.Errorf("task not found: %s", id)
	}
	mutate(task)
	task.UpdatedAt = time.Now()
	return nil
}
