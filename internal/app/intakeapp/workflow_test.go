package intakeapp

import (
	"context"
	"testing"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

func TestPrepareWorkflowQuarantinesScansAndPlans(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewService(repo, NewStaticScanner(), NewDryRunPlanner(func() time.Time { return time.Unix(1, 0) }))
	workflow, err := svc.PrepareWorkflowFromMessage(context.Background(), channel.InboundMessage{
		ID:        "msg1",
		Channel:   channel.KindQQ,
		AccountID: "default",
		SenderID:  "10001",
		Text:      "请为 ShanghaiTech original 创建数据接入计划",
		Attachments: []channel.Attachment{{
			ID:        "att1",
			Name:      "shanghaitech-original.manifest",
			MediaType: "application/x-directory",
			SourceURI: "F:\\automated_training_model\\data_lake\\raw\\datasets\\shanghaitech\\original",
			Status:    channel.AttachmentReceived,
		}},
	}, PlanOptions{Mode: PlanModeData})
	if err != nil {
		t.Fatal(err)
	}
	if workflow.Status != WorkflowPendingApproval {
		t.Fatalf("expected pending approval, got %+v", workflow)
	}
	if workflow.Plan.DatasetName != "shanghaitech-original" {
		t.Fatalf("expected ShanghaiTech plan, got %+v", workflow.Plan)
	}
	if len(workflow.Attachments) != 1 || workflow.Attachments[0].Status != channel.AttachmentScanned {
		t.Fatalf("expected scanned attachment, got %+v", workflow.Attachments)
	}
	if len(workflow.ScanReports) != 1 || !workflow.ScanReports[0].Accepted {
		t.Fatalf("expected accepted scan report, got %+v", workflow.ScanReports)
	}
}

func TestPrepareWorkflowRejectsUnsafeAttachmentMetadata(t *testing.T) {
	svc := NewService(NewMemoryRepository(), NewStaticScanner(), NewDryRunPlanner(nil))
	workflow, err := svc.PrepareWorkflowFromMessage(context.Background(), channel.InboundMessage{
		ID:          "msg1",
		Attachments: []channel.Attachment{{ID: "att1", Name: "../escape.zip", MediaType: "application/zip"}},
	}, PlanOptions{Mode: PlanModeData})
	if err != nil {
		t.Fatal(err)
	}
	if workflow.Status != WorkflowRejected {
		t.Fatalf("expected rejected workflow, got %+v", workflow)
	}
	if workflow.Attachments[0].Status != channel.AttachmentRejected {
		t.Fatalf("expected rejected attachment, got %+v", workflow.Attachments[0])
	}
}

func TestApproveAndRegisterWorkflow(t *testing.T) {
	svc := NewService(NewMemoryRepository(), NewStaticScanner(), NewDryRunPlanner(nil))
	workflow, err := svc.PrepareWorkflowFromMessage(context.Background(), channel.InboundMessage{
		ID:          "msg1",
		Attachments: []channel.Attachment{{ID: "att1", Name: "dataset.jsonl", MediaType: "application/jsonl"}},
	}, PlanOptions{DatasetName: "dataset-demo"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RegisterWorkflow(context.Background(), workflow.ID, "tester"); err == nil {
		t.Fatal("expected register before approval to fail")
	}
	approved, err := svc.ApproveWorkflow(context.Background(), workflow.ID, "tester", "ok")
	if err != nil {
		t.Fatal(err)
	}
	if approved.Status != WorkflowApproved || approved.Approval == nil {
		t.Fatalf("expected approved workflow, got %+v", approved)
	}
	registered, err := svc.RegisterWorkflow(context.Background(), workflow.ID, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if registered.Status != WorkflowRegistered || registered.RegisteredDatasetID != "dataset-demo" {
		t.Fatalf("expected registered workflow, got %+v", registered)
	}
	if registered.Attachments[0].Status != channel.AttachmentAccepted {
		t.Fatalf("expected accepted attachment after register, got %+v", registered.Attachments[0])
	}
}
