package intakeapp

import (
	"context"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

type Repository interface {
	SavePlan(ctx context.Context, plan channel.DataIntakePlan) (channel.DataIntakePlan, error)
	SaveAttachment(ctx context.Context, attachment channel.Attachment) (channel.Attachment, error)
	SaveWorkflow(ctx context.Context, workflow IntakeWorkflow) (IntakeWorkflow, error)
	GetWorkflow(ctx context.Context, id string) (IntakeWorkflow, bool, error)
	ListWorkflows(ctx context.Context, limit int) ([]IntakeWorkflow, error)
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

type WorkflowStatus string

const (
	WorkflowPendingApproval WorkflowStatus = "pending_approval"
	WorkflowApproved        WorkflowStatus = "approved"
	WorkflowRegistered      WorkflowStatus = "registered"
	WorkflowRejected        WorkflowStatus = "rejected"
)

type ApprovalRecord struct {
	Decision string    `json:"decision"`
	By       string    `json:"by,omitempty"`
	Note     string    `json:"note,omitempty"`
	At       time.Time `json:"at"`
}

type IntakeWorkflow struct {
	ID                  string                 `json:"id"`
	Status              WorkflowStatus         `json:"status"`
	Plan                channel.DataIntakePlan `json:"plan"`
	Attachments         []channel.Attachment   `json:"attachments,omitempty"`
	ScanReports         []ScanReport           `json:"scan_reports,omitempty"`
	ApprovalRequired    bool                   `json:"approval_required"`
	Approval            *ApprovalRecord        `json:"approval,omitempty"`
	RegisteredDatasetID string                 `json:"registered_dataset_id,omitempty"`
	CreatedAt           time.Time              `json:"created_at"`
	UpdatedAt           time.Time              `json:"updated_at"`
	RegisteredAt        *time.Time             `json:"registered_at,omitempty"`
}
