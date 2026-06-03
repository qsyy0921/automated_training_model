package lifecycleapp

import (
	"context"
	"strings"
	"testing"

	"github.com/qsyy0921/automated_training_model/internal/domain/deployment"
	"github.com/qsyy0921/automated_training_model/internal/domain/evaluation"
	"github.com/qsyy0921/automated_training_model/internal/domain/training"
	"github.com/qsyy0921/automated_training_model/internal/domain/workflow"
)

type fakeGateway struct {
	taskType string
	payload  map[string]string
	status   *workflow.Task
	canceled string
}

func (f *fakeGateway) Submit(ctx context.Context, taskType string, payload map[string]string) (string, error) {
	f.taskType = taskType
	f.payload = payload
	return "task_000001", nil
}

func (f *fakeGateway) Status(ctx context.Context, id string) (*workflow.Task, error) {
	if f.status != nil {
		return f.status, nil
	}
	return &workflow.Task{ID: id, Type: "training.run", Status: workflow.TaskPending}, nil
}

func (f *fakeGateway) Cancel(ctx context.Context, id string) error {
	f.canceled = id
	return nil
}

func TestSubmitTrainingUsesGateway(t *testing.T) {
	gateway := &fakeGateway{}
	svc := NewService(gateway)
	run, err := svc.SubmitTraining(context.Background(), training.Request{
		DatasetID:   "shanghaitech-original",
		TargetTask:  "detection",
		ModelFamily: "yolo11n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if run.TaskID != "task_000001" || gateway.taskType != "training.run" {
		t.Fatalf("unexpected training run/gateway state: run=%+v gateway=%+v", run, gateway)
	}
	if gateway.payload["request_json"] == "" {
		t.Fatalf("expected request_json payload, got %+v", gateway.payload)
	}
	if gateway.payload["dataset_id"] != "shanghaitech-original" || gateway.payload["target_task"] != "detection" || gateway.payload["model_family"] != "yolo11n" {
		t.Fatalf("expected normalized training payload fields, got %+v", gateway.payload)
	}
	if gateway.payload["dry_run"] != "false" {
		t.Fatalf("expected default false dry_run payload, got %+v", gateway.payload)
	}
}

func TestSubmitEvaluationUsesGateway(t *testing.T) {
	gateway := &fakeGateway{}
	svc := NewService(gateway)
	run, err := svc.SubmitEvaluation(context.Background(), evaluation.Request{
		DatasetID: "shanghaitech-original",
		ModelID:   "model-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if run.TaskID != "task_000001" || gateway.taskType != "evaluation.run" {
		t.Fatalf("unexpected evaluation run/gateway state: run=%+v gateway=%+v", run, gateway)
	}
	if gateway.payload["dataset_id"] != "shanghaitech-original" || gateway.payload["model_id"] != "model-1" {
		t.Fatalf("expected normalized evaluation payload fields, got %+v", gateway.payload)
	}
}

func TestSubmitDeploymentUsesGateway(t *testing.T) {
	gateway := &fakeGateway{}
	svc := NewService(gateway)
	dep, err := svc.SubmitDeployment(context.Background(), deployment.Request{
		ModelID: "model-1",
		Target:  "local-dry-run",
	})
	if err != nil {
		t.Fatal(err)
	}
	if dep.TaskID != "task_000001" || gateway.taskType != "deployment.run" {
		t.Fatalf("unexpected deployment/gateway state: dep=%+v gateway=%+v", dep, gateway)
	}
	if gateway.payload["model_id"] != "model-1" || gateway.payload["target"] != "local-dry-run" {
		t.Fatalf("expected normalized deployment payload fields, got %+v", gateway.payload)
	}
}

func TestSubmitDeploymentPreservesExplicitDryRun(t *testing.T) {
	gateway := &fakeGateway{}
	svc := NewService(gateway)
	_, err := svc.SubmitDeployment(context.Background(), deployment.Request{
		ModelID: "model-1",
		Target:  "local-dry-run",
		DryRun:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if gateway.payload["dry_run"] != "true" {
		t.Fatalf("expected explicit dry_run payload, got %+v", gateway.payload)
	}
	if !strings.Contains(gateway.payload["request_json"], `"dry_run":true`) {
		t.Fatalf("expected request_json to include dry_run=true, got %s", gateway.payload["request_json"])
	}
}

func TestTaskStatusAndCancelProxyToGateway(t *testing.T) {
	gateway := &fakeGateway{
		status: &workflow.Task{ID: "task_000001", Type: "training.run", Status: workflow.TaskPending},
	}
	svc := NewService(gateway)
	task, err := svc.TaskStatus(context.Background(), "task_000001")
	if err != nil {
		t.Fatal(err)
	}
	if task.ID != "task_000001" {
		t.Fatalf("unexpected task: %+v", task)
	}
	if err := svc.CancelTask(context.Background(), "task_000001"); err != nil {
		t.Fatal(err)
	}
	if gateway.canceled != "task_000001" {
		t.Fatalf("expected cancel to reach gateway, got %q", gateway.canceled)
	}
}
