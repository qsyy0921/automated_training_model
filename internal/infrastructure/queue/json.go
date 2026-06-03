package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/domain/workflow"
)

type JSONQueue struct {
	mu    sync.Mutex
	path  string
	now   func() time.Time
	next  int
	tasks map[string]*workflow.Task
}

func NewJSONQueue(path string, now func() time.Time) (*JSONQueue, error) {
	if now == nil {
		now = time.Now
	}
	q := &JSONQueue{
		path:  path,
		now:   now,
		tasks: map[string]*workflow.Task{},
	}
	if err := q.load(); err != nil {
		return nil, err
	}
	return q, nil
}

func (q *JSONQueue) Enqueue(ctx context.Context, spec workflow.TaskSpec) (string, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.next++
	now := q.now()
	id := fmt.Sprintf("task_%06d", q.next)
	task := &workflow.Task{
		ID:        id,
		Type:      spec.Type,
		Status:    workflow.TaskPending,
		Payload:   copyStringMap(spec.Payload),
		CreatedAt: now,
		UpdatedAt: now,
	}
	q.tasks[id] = task
	if err := q.persistLocked(); err != nil {
		return "", err
	}
	return id, nil
}

func (q *JSONQueue) Status(ctx context.Context, id string) (*workflow.Task, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	task := q.tasks[id]
	if task == nil {
		return nil, fmt.Errorf("task not found: %s", id)
	}
	copied := cloneTask(*task)
	return &copied, nil
}

func (q *JSONQueue) Cancel(ctx context.Context, id string) error {
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
	task.UpdatedAt = q.now()
	return q.persistLocked()
}

func (q *JSONQueue) Update(ctx context.Context, id string, mutate func(*workflow.Task)) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	task := q.tasks[id]
	if task == nil {
		return fmt.Errorf("task not found: %s", id)
	}
	mutate(task)
	task.UpdatedAt = q.now()
	return q.persistLocked()
}

func (q *JSONQueue) load() error {
	var rows []workflow.Task
	if err := readJSONFile(q.path, &rows); err != nil {
		return err
	}
	for i := range rows {
		task := rows[i]
		if task.ID == "" {
			continue
		}
		q.tasks[task.ID] = &task
		if n, ok := parseTaskSequence(task.ID); ok && n > q.next {
			q.next = n
		}
	}
	return nil
}

func (q *JSONQueue) persistLocked() error {
	rows := make([]workflow.Task, 0, len(q.tasks))
	for _, task := range q.tasks {
		copied := cloneTask(*task)
		rows = append(rows, copied)
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].UpdatedAt.After(rows[j].UpdatedAt)
	})
	return writeJSONFile(q.path, rows)
}

func parseTaskSequence(id string) (int, bool) {
	var n int
	if _, err := fmt.Sscanf(id, "task_%d", &n); err != nil {
		return 0, false
	}
	return n, true
}

func readJSONFile(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, value)
}

func writeJSONFile(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
