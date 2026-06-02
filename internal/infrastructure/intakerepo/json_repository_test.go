package intakerepo

import (
	"context"
	"testing"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/app/intakeapp"
	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

func TestJSONRepositoryRestoresPlans(t *testing.T) {
	root := t.TempDir()
	created := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	repo, err := NewJSONRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.SavePlan(context.Background(), channel.DataIntakePlan{
		ID:              "intake-plan-1",
		SourceMessageID: "msg1",
		Channel:         channel.KindQQ,
		AccountID:       "default",
		SenderID:        "10001",
		DatasetName:     "shanghaitech-original",
		DryRun:          true,
		CreatedAt:       created,
	})
	if err != nil {
		t.Fatal(err)
	}

	restored, err := NewJSONRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	plans := restored.ListPlans()
	if len(plans) != 1 {
		t.Fatalf("expected one restored plan, got %+v", plans)
	}
	if plans[0].DatasetName != "shanghaitech-original" || !plans[0].DryRun {
		t.Fatalf("unexpected restored plan: %+v", plans[0])
	}
}

func TestJSONRepositoryRestoresWorkflows(t *testing.T) {
	root := t.TempDir()
	created := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	repo, err := NewJSONRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.SaveWorkflow(context.Background(), intakeapp.IntakeWorkflow{
		ID:     "intake-workflow-1",
		Status: intakeapp.WorkflowApproved,
		Plan: channel.DataIntakePlan{
			ID:          "intake-plan-1",
			DatasetName: "shanghaitech-original",
			DryRun:      true,
			CreatedAt:   created,
		},
		Attachments:      []channel.Attachment{{ID: "att1", Status: channel.AttachmentScanned, CreatedAt: created}},
		ApprovalRequired: true,
		CreatedAt:        created,
		UpdatedAt:        created,
	})
	if err != nil {
		t.Fatal(err)
	}
	restored, err := NewJSONRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	workflows, err := restored.ListWorkflows(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(workflows) != 1 || workflows[0].Status != intakeapp.WorkflowApproved {
		t.Fatalf("unexpected restored workflows: %+v", workflows)
	}
	workflow, ok, err := restored.GetWorkflow(context.Background(), "intake-workflow-1")
	if err != nil || !ok || workflow.Plan.DatasetName != "shanghaitech-original" {
		t.Fatalf("unexpected workflow lookup: ok=%v err=%v workflow=%+v", ok, err, workflow)
	}
}
