package intakeapp

import (
	"context"
	"sort"
	"sync"

	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

type MemoryRepository struct {
	mu          sync.Mutex
	plans       []channel.DataIntakePlan
	attachments []channel.Attachment
	workflows   map[string]IntakeWorkflow
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{workflows: map[string]IntakeWorkflow{}}
}

func (r *MemoryRepository) SavePlan(ctx context.Context, plan channel.DataIntakePlan) (channel.DataIntakePlan, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plans = append([]channel.DataIntakePlan{plan}, r.plans...)
	return plan, nil
}

func (r *MemoryRepository) SaveAttachment(ctx context.Context, attachment channel.Attachment) (channel.Attachment, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	r.attachments = append([]channel.Attachment{attachment}, r.attachments...)
	return attachment, nil
}

func (r *MemoryRepository) ListPlans() []channel.DataIntakePlan {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]channel.DataIntakePlan, len(r.plans))
	copy(out, r.plans)
	return out
}

func (r *MemoryRepository) SaveWorkflow(ctx context.Context, workflow IntakeWorkflow) (IntakeWorkflow, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.workflows == nil {
		r.workflows = map[string]IntakeWorkflow{}
	}
	r.workflows[workflow.ID] = workflow
	return workflow, nil
}

func (r *MemoryRepository) GetWorkflow(ctx context.Context, id string) (IntakeWorkflow, bool, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	workflow, ok := r.workflows[id]
	return workflow, ok, nil
}

func (r *MemoryRepository) ListWorkflows(ctx context.Context, limit int) ([]IntakeWorkflow, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]IntakeWorkflow, 0, len(r.workflows))
	for _, workflow := range r.workflows {
		out = append(out, workflow)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	if limit <= 0 {
		limit = 100
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
