package intakeapp

import (
	"context"
	"testing"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

func TestDryRunPlannerCreatesShanghaiTechPlan(t *testing.T) {
	planner := NewDryRunPlanner(func() time.Time { return time.Unix(1, 0) })
	plan, err := planner.PlanWithOptions(context.Background(), channel.InboundMessage{
		ID:        "msg1",
		Channel:   channel.KindQQ,
		AccountID: "default",
		Peer:      channel.Peer{Channel: channel.KindQQ, AccountID: "default", Kind: channel.PeerKindDirect, ID: "10001"},
		SenderID:  "10001",
		Text:      "请为 ShanghaiTech original 创建数据接入计划",
		Attachments: []channel.Attachment{{
			ID:        "att1",
			Name:      "shanghaitech-original.manifest",
			MediaType: "application/x-directory",
			SourceURI: "F:\\automated_training_model\\data_lake\\raw\\datasets\\shanghaitech\\original",
		}},
	}, PlanOptions{Mode: PlanModeData})
	if err != nil {
		t.Fatal(err)
	}
	if plan.DatasetName != "shanghaitech-original" {
		t.Fatalf("expected ShanghaiTech dataset, got %s", plan.DatasetName)
	}
	if plan.RiskLevel != "low" {
		t.Fatalf("expected low risk directory plan, got %s", plan.RiskLevel)
	}
	if len(plan.ProposedActions) != 3 || plan.ProposedActions[2].Kind != "create_data_intake_plan" {
		t.Fatalf("unexpected actions: %+v", plan.ProposedActions)
	}
	if !plan.DryRun {
		t.Fatal("expected dry-run plan")
	}
}

func TestDryRunPlannerMarksArchiveAsMediumRisk(t *testing.T) {
	planner := NewDryRunPlanner(nil)
	plan, err := planner.Plan(context.Background(), channel.InboundMessage{
		ID:          "msg1",
		Attachments: []channel.Attachment{{ID: "att1", Name: "upload.zip", MediaType: "application/zip"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.RiskLevel != "medium" {
		t.Fatalf("expected medium risk, got %s", plan.RiskLevel)
	}
}

func TestDryRunPlannerCreatesVisionPlan(t *testing.T) {
	planner := NewDryRunPlanner(nil)
	plan, err := planner.PlanWithOptions(context.Background(), channel.InboundMessage{
		ID:          "msg1",
		Attachments: []channel.Attachment{{ID: "att1", Name: "frame.png", MediaType: "image/png"}},
	}, PlanOptions{Mode: PlanModeVision, ModelRoute: "vision", Model: "mimo-v2.5"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.ID == "" || plan.ProposedActions[1].Kind != "vlm_inspect" {
		t.Fatalf("expected vision plan, got %+v", plan)
	}
	if plan.ProposedActions[1].Params["model"] != "mimo-v2.5" {
		t.Fatalf("expected Mimo vision model params, got %+v", plan.ProposedActions[1].Params)
	}
}
