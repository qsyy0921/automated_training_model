package intakeapp

import (
	"context"
	"sync"

	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

type MemoryRepository struct {
	mu          sync.Mutex
	plans       []channel.DataIntakePlan
	attachments []channel.Attachment
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{}
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
