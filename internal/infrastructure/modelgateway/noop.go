package modelgateway

import (
	"context"

	"github.com/qsyy0921/_video_label_tool/labelserver/internal/app"
	"github.com/qsyy0921/_video_label_tool/labelserver/internal/domain/workflow"
)

type NoopGateway struct {
	queue app.TaskQueue
}

func NewNoopGateway(queue app.TaskQueue) *NoopGateway {
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
