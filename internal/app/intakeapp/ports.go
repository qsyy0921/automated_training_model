package intakeapp

import (
	"context"

	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

type Repository interface {
	SavePlan(ctx context.Context, plan channel.DataIntakePlan) (channel.DataIntakePlan, error)
	SaveAttachment(ctx context.Context, attachment channel.Attachment) (channel.Attachment, error)
}

type Scanner interface {
	Scan(ctx context.Context, attachment channel.Attachment) (ScanReport, error)
}

type Planner interface {
	Plan(ctx context.Context, msg channel.InboundMessage) (channel.DataIntakePlan, error)
}

type ScanReport struct {
	AttachmentID string            `json:"attachment_id"`
	Accepted     bool              `json:"accepted"`
	Reason       string            `json:"reason,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}
