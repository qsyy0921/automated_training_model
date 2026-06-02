package intakeapp

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

type PlanMode string

const (
	PlanModeData   PlanMode = "data"
	PlanModeVision PlanMode = "vision"
)

type PlanOptions struct {
	Mode        PlanMode
	DatasetName string
	ModelRoute  string
	Model       string
	Now         func() time.Time
}

type DryRunPlanner struct {
	now func() time.Time
}

func NewDryRunPlanner(now func() time.Time) *DryRunPlanner {
	if now == nil {
		now = time.Now
	}
	return &DryRunPlanner{now: now}
}

func (p *DryRunPlanner) Plan(ctx context.Context, msg channel.InboundMessage) (channel.DataIntakePlan, error) {
	return p.PlanWithOptions(ctx, msg, PlanOptions{Mode: PlanModeData})
}

func (p *DryRunPlanner) PlanWithOptions(ctx context.Context, msg channel.InboundMessage, opts PlanOptions) (channel.DataIntakePlan, error) {
	_ = ctx
	if strings.TrimSpace(msg.ID) == "" {
		return channel.DataIntakePlan{}, fmt.Errorf("message id is required")
	}
	now := p.now
	if opts.Now != nil {
		now = opts.Now
	}
	mode := opts.Mode
	if mode == "" {
		mode = PlanModeData
	}
	planIDPrefix := "intake-plan"
	actions := []channel.PlannedAction{
		{Kind: "quarantine", Params: map[string]string{"attachment_count": strconv.Itoa(len(msg.Attachments))}},
		{Kind: "scan", Params: map[string]string{"scanner": "mvp-static-preflight"}},
		{Kind: "create_data_intake_plan", Params: map[string]string{"mode": "dry_run"}},
	}
	if mode == PlanModeVision {
		planIDPrefix = "vision-plan"
		modelRoute := valueOr(opts.ModelRoute, "vision")
		model := valueOr(opts.Model, "mimo-v2.5")
		actions = []channel.PlannedAction{
			{Kind: "quarantine", Params: map[string]string{"attachment_count": strconv.Itoa(len(msg.Attachments))}},
			{Kind: "vlm_inspect", Params: map[string]string{"model_route": modelRoute, "model": model}},
			{Kind: "create_data_intake_plan", Params: map[string]string{"mode": "dry_run"}},
		}
	}
	return channel.DataIntakePlan{
		ID:                fmt.Sprintf("%s-%d", planIDPrefix, now().UnixNano()),
		SourceMessageID:   msg.ID,
		Channel:           msg.Channel,
		AccountID:         msg.AccountID,
		SenderID:          msg.SenderID,
		Intent:            channel.IntakeIntentInspect,
		DatasetName:       inferDatasetName(msg, opts.DatasetName),
		ProposedActions:   actions,
		RequiredApprovals: []string{"human_review_before_data_lake_write"},
		RiskLevel:         attachmentRisk(msg.Attachments),
		DryRun:            true,
		CreatedAt:         now(),
	}, nil
}

func inferDatasetName(msg channel.InboundMessage, explicit string) string {
	if value := strings.TrimSpace(explicit); value != "" {
		return value
	}
	text := strings.ToLower(msg.Text)
	for _, attachment := range msg.Attachments {
		text += " " + strings.ToLower(attachment.Name) + " " + strings.ToLower(attachment.SourceURI) + " " + strings.ToLower(attachment.LocalURI)
	}
	if strings.Contains(text, "shanghaitech") || strings.Contains(text, "上海") {
		return "shanghaitech-original"
	}
	return "channel-upload-draft"
}

func attachmentRisk(attachments []channel.Attachment) string {
	for _, attachment := range attachments {
		mediaType := strings.ToLower(strings.TrimSpace(attachment.MediaType))
		name := strings.ToLower(strings.TrimSpace(attachment.Name))
		if strings.Contains(mediaType, "zip") || strings.HasSuffix(name, ".zip") || strings.HasSuffix(name, ".7z") || strings.HasSuffix(name, ".rar") {
			return "medium"
		}
	}
	return "low"
}

func valueOr(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
