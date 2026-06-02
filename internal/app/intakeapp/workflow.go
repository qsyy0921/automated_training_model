package intakeapp

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

type StaticScanner struct{}

func NewStaticScanner() *StaticScanner {
	return &StaticScanner{}
}

func (s *StaticScanner) Scan(ctx context.Context, attachment channel.Attachment) (ScanReport, error) {
	_ = ctx
	report := ScanReport{
		AttachmentID: attachment.ID,
		Accepted:     true,
		Reason:       "static metadata scan passed",
		Metadata: map[string]string{
			"name":       attachment.Name,
			"media_type": attachment.MediaType,
			"source_uri": attachment.SourceURI,
			"local_uri":  attachment.LocalURI,
		},
	}
	target := strings.ToLower(attachment.Name + " " + attachment.SourceURI + " " + attachment.LocalURI)
	if strings.Contains(target, "\x00") || strings.Contains(target, "../") || strings.Contains(target, "..\\") {
		report.Accepted = false
		report.Reason = "path traversal marker detected in attachment metadata"
		return report, nil
	}
	if attachment.SizeBytes < 0 {
		report.Accepted = false
		report.Reason = "negative attachment size"
		return report, nil
	}
	if strings.TrimSpace(attachment.LocalURI) != "" {
		cleaned := filepath.Clean(attachment.LocalURI)
		report.Metadata["local_clean"] = cleaned
	}
	return report, nil
}

func (s *Service) PrepareWorkflowFromMessage(ctx context.Context, msg channel.InboundMessage, opts PlanOptions) (IntakeWorkflow, error) {
	if len(msg.Attachments) == 0 {
		return IntakeWorkflow{}, fmt.Errorf("at least one attachment is required")
	}
	now := time.Now()
	attachments := make([]channel.Attachment, 0, len(msg.Attachments))
	reports := make([]ScanReport, 0, len(msg.Attachments))
	rejected := false
	for i, attachment := range msg.Attachments {
		if strings.TrimSpace(attachment.ID) == "" {
			attachment.ID = fmt.Sprintf("%s-att-%d", msg.ID, i+1)
		}
		quarantined, err := s.QuarantineAttachment(ctx, attachment)
		if err != nil {
			return IntakeWorkflow{}, err
		}
		report, err := s.ScanAttachment(ctx, quarantined)
		if err != nil {
			return IntakeWorkflow{}, err
		}
		if report.Accepted {
			quarantined.Status = channel.AttachmentScanned
		} else {
			quarantined.Status = channel.AttachmentRejected
			rejected = true
		}
		quarantined, err = s.repo.SaveAttachment(ctx, quarantined)
		if err != nil {
			return IntakeWorkflow{}, err
		}
		attachments = append(attachments, quarantined)
		reports = append(reports, report)
	}
	plan, err := s.PlanFromMessageWithOptions(ctx, msg, opts)
	if err != nil {
		return IntakeWorkflow{}, err
	}
	status := WorkflowPendingApproval
	if rejected {
		status = WorkflowRejected
	}
	workflow := IntakeWorkflow{
		ID:               "intake-workflow-" + plan.ID,
		Status:           status,
		Plan:             plan,
		Attachments:      attachments,
		ScanReports:      reports,
		ApprovalRequired: channel.RequiresApproval(plan),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	return s.repo.SaveWorkflow(ctx, workflow)
}

func (s *Service) ListWorkflows(ctx context.Context, limit int) ([]IntakeWorkflow, error) {
	return s.repo.ListWorkflows(ctx, limit)
}

func (s *Service) GetWorkflow(ctx context.Context, id string) (IntakeWorkflow, bool, error) {
	return s.repo.GetWorkflow(ctx, id)
}

func (s *Service) ApproveWorkflow(ctx context.Context, id string, by string, note string) (IntakeWorkflow, error) {
	workflow, ok, err := s.repo.GetWorkflow(ctx, id)
	if err != nil {
		return IntakeWorkflow{}, err
	}
	if !ok {
		return IntakeWorkflow{}, fmt.Errorf("intake workflow not found: %s", id)
	}
	if workflow.Status == WorkflowRejected {
		return IntakeWorkflow{}, fmt.Errorf("rejected intake workflow cannot be approved: %s", id)
	}
	now := time.Now()
	workflow.Status = WorkflowApproved
	workflow.Approval = &ApprovalRecord{Decision: "approved", By: strings.TrimSpace(by), Note: strings.TrimSpace(note), At: now}
	workflow.UpdatedAt = now
	return s.repo.SaveWorkflow(ctx, workflow)
}

func (s *Service) RegisterWorkflow(ctx context.Context, id string, by string) (IntakeWorkflow, error) {
	workflow, ok, err := s.repo.GetWorkflow(ctx, id)
	if err != nil {
		return IntakeWorkflow{}, err
	}
	if !ok {
		return IntakeWorkflow{}, fmt.Errorf("intake workflow not found: %s", id)
	}
	if workflow.Status != WorkflowApproved {
		return IntakeWorkflow{}, fmt.Errorf("intake workflow must be approved before register: %s", id)
	}
	now := time.Now()
	datasetID := strings.TrimSpace(workflow.Plan.DatasetName)
	if datasetID == "" {
		datasetID = "channel-upload-draft"
	}
	workflow.Status = WorkflowRegistered
	workflow.RegisteredDatasetID = datasetID
	workflow.RegisteredAt = &now
	workflow.UpdatedAt = now
	for i := range workflow.Attachments {
		if workflow.Attachments[i].Status == channel.AttachmentScanned {
			workflow.Attachments[i].Status = channel.AttachmentAccepted
			if _, err := s.repo.SaveAttachment(ctx, workflow.Attachments[i]); err != nil {
				return IntakeWorkflow{}, err
			}
		}
	}
	if workflow.Approval == nil {
		workflow.Approval = &ApprovalRecord{Decision: "approved", By: strings.TrimSpace(by), At: now}
	}
	return s.repo.SaveWorkflow(ctx, workflow)
}
