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
