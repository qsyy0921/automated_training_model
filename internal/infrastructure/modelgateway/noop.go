package modelgateway

import (
	"context"

	"github.com/qsyy0921/automated_training_model/internal/app/workflowapp"
	"github.com/qsyy0921/automated_training_model/internal/domain/workflow"
)

type NoopGateway struct {
	queue workflowapp.TaskQueue
}

func NewNoopGateway(queue workflowapp.TaskQueue) *NoopGateway {
	return &NoopGateway{queue: queue}
}

func (g *NoopGateway) Submit(ctx context.Context, taskType string, payload map[string]string) (string, error) {
	return g.queue.Enqueue(ctx, workflow.TaskSpec{Type: taskType, Payload: payload})
}

func (g *NoopGateway) Status(ctx context.Context, id string) (*workflow.Task, error) {
	return g.queue.Status(ctx, id)
}

func (g *NoopGateway) Cancel(ctx context.Context, id string) error {
	return g.queue.Cancel(ctx, id)
}

func (g *NoopGateway) Resume(ctx context.Context, id string) (string, error) {
	task, err := g.queue.Status(ctx, id)
	if err != nil {
		return "", err
	}
	newID, err := g.queue.Enqueue(ctx, workflow.TaskSpec{Type: task.Type, Payload: task.Payload})
	if err != nil {
		return "", err
	}
	_ = g.queue.Update(ctx, newID, func(next *workflow.Task) {
		next.ParentID = task.ID
		next.Metadata = map[string]string{"resumed_from_task_id": task.ID}
	})
	_ = g.queue.Update(ctx, id, func(prev *workflow.Task) {
		prev.Resumable = false
		if prev.Metadata == nil {
			prev.Metadata = map[string]string{}
		}
		prev.Metadata["resumed_by_task_id"] = newID
		prev.Message = "resumed as " + newID
	})
	return newID, nil
}
