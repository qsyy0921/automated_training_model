package workflowapp

import (
	"context"

	"github.com/qsyy0921/automated_training_model/internal/domain/workflow"
)

type TaskQueue interface {
	Enqueue(ctx context.Context, spec workflow.TaskSpec) (string, error)
	List(ctx context.Context, limit int) ([]workflow.Task, error)
	Status(ctx context.Context, id string) (*workflow.Task, error)
	Lineage(ctx context.Context, id string) ([]workflow.Task, error)
	Cancel(ctx context.Context, id string) error
	Update(ctx context.Context, id string, mutate func(*workflow.Task)) error
}

type ModelGateway interface {
	Submit(ctx context.Context, taskType string, payload map[string]string) (string, error)
	List(ctx context.Context, limit int) ([]workflow.Task, error)
	Status(ctx context.Context, id string) (*workflow.Task, error)
	Lineage(ctx context.Context, id string) ([]workflow.Task, error)
	Cancel(ctx context.Context, id string) error
	Resume(ctx context.Context, id string) (string, error)
}
