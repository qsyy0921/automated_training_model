package intakeapp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

type Service struct {
	repo    Repository
	scanner Scanner
	planner Planner
}

func NewService(repo Repository, scanner Scanner, planner Planner) *Service {
	if scanner == nil {
		scanner = NewStaticScanner()
	}
	return &Service{repo: repo, scanner: scanner, planner: planner}
}

func (s *Service) PlanFromMessage(ctx context.Context, msg channel.InboundMessage) (channel.DataIntakePlan, error) {
	return s.PlanFromMessageWithOptions(ctx, msg, PlanOptions{})
}

func (s *Service) PlanFromMessageWithOptions(ctx context.Context, msg channel.InboundMessage, opts PlanOptions) (channel.DataIntakePlan, error) {
	if strings.TrimSpace(msg.ID) == "" {
		return channel.DataIntakePlan{}, fmt.Errorf("message id is required")
	}
	var plan channel.DataIntakePlan
	var err error
	if planner, ok := s.planner.(interface {
		PlanWithOptions(context.Context, channel.InboundMessage, PlanOptions) (channel.DataIntakePlan, error)
	}); ok {
		plan, err = planner.PlanWithOptions(ctx, msg, opts)
	} else {
		plan, err = s.planner.Plan(ctx, msg)
	}
	if err != nil {
		return channel.DataIntakePlan{}, err
	}
	if plan.SourceMessageID == "" {
		plan.SourceMessageID = msg.ID
	}
	if plan.Channel == "" {
		plan.Channel = msg.Channel
	}
	if plan.AccountID == "" {
		plan.AccountID = msg.AccountID
	}
	if plan.SenderID == "" {
		plan.SenderID = msg.SenderID
	}
	if plan.CreatedAt.IsZero() {
		plan.CreatedAt = time.Now()
	}
	return s.repo.SavePlan(ctx, plan)
}

func (s *Service) QuarantineAttachment(ctx context.Context, attachment channel.Attachment) (channel.Attachment, error) {
	if strings.TrimSpace(attachment.ID) == "" {
		return channel.Attachment{}, fmt.Errorf("attachment id is required")
	}
	attachment.Status = channel.AttachmentQuarantined
	if attachment.CreatedAt.IsZero() {
		attachment.CreatedAt = time.Now()
	}
	return s.repo.SaveAttachment(ctx, attachment)
}

func (s *Service) ScanAttachment(ctx context.Context, attachment channel.Attachment) (ScanReport, error) {
	return s.scanner.Scan(ctx, attachment)
}
