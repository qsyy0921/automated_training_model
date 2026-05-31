package lifecycleapp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/qsyy0921/automated_training_model/internal/app/workflowapp"
	"github.com/qsyy0921/automated_training_model/internal/domain/autolabel"
	"github.com/qsyy0921/automated_training_model/internal/domain/deployment"
	"github.com/qsyy0921/automated_training_model/internal/domain/evaluation"
	"github.com/qsyy0921/automated_training_model/internal/domain/modelregistry"
	"github.com/qsyy0921/automated_training_model/internal/domain/training"
	"github.com/qsyy0921/automated_training_model/internal/domain/workflow"
)

type Service struct {
	gateway workflowapp.ModelGateway
}

func NewService(gateway workflowapp.ModelGateway) *Service {
	return &Service{gateway: gateway}
}

func (s *Service) TaskStatus(ctx context.Context, id string) (*workflow.Task, error) {
	return s.gateway.Status(ctx, id)
}

func (s *Service) CancelTask(ctx context.Context, id string) error {
	return s.gateway.Cancel(ctx, id)
}

func (s *Service) SubmitAutoLabel(ctx context.Context, req autolabel.Request) (autolabel.Job, error) {
	if req.DatasetID == "" {
		return autolabel.Job{}, fmt.Errorf("dataset_id is required")
	}
	taskID, err := s.submit(ctx, "autolabel.run", req)
	if err != nil {
		return autolabel.Job{}, err
	}
	return autolabel.Job{TaskID: taskID, DatasetID: req.DatasetID, Status: "queued"}, nil
}

func (s *Service) SubmitTraining(ctx context.Context, req training.Request) (training.Run, error) {
	if req.DatasetID == "" {
		return training.Run{}, fmt.Errorf("dataset_id is required")
	}
	if req.TargetTask == "" {
		return training.Run{}, fmt.Errorf("target_task is required")
	}
	taskID, err := s.submit(ctx, "training.run", req)
	if err != nil {
		return training.Run{}, err
	}
	return training.Run{TaskID: taskID, DatasetID: req.DatasetID, TargetTask: req.TargetTask, ModelFamily: req.ModelFamily, Status: "queued"}, nil
}

func (s *Service) SubmitEvaluation(ctx context.Context, req evaluation.Request) (evaluation.Run, error) {
	if req.DatasetID == "" {
		return evaluation.Run{}, fmt.Errorf("dataset_id is required")
	}
	if req.ModelID == "" {
		return evaluation.Run{}, fmt.Errorf("model_id is required")
	}
	taskID, err := s.submit(ctx, "evaluation.run", req)
	if err != nil {
		return evaluation.Run{}, err
	}
	return evaluation.Run{TaskID: taskID, DatasetID: req.DatasetID, ModelID: req.ModelID, Status: "queued"}, nil
}

func (s *Service) RegisterModel(ctx context.Context, req modelregistry.RegisterRequest) (modelregistry.ModelVersion, error) {
	if req.Name == "" {
		return modelregistry.ModelVersion{}, fmt.Errorf("name is required")
	}
	if req.ArtifactURI == "" {
		return modelregistry.ModelVersion{}, fmt.Errorf("artifact_uri is required")
	}
	taskID, err := s.submit(ctx, "model.register", req)
	if err != nil {
		return modelregistry.ModelVersion{}, err
	}
	return modelregistry.ModelVersion{TaskID: taskID, Name: req.Name, ModelFamily: req.ModelFamily, Task: req.Task, ArtifactURI: req.ArtifactURI, Status: "queued"}, nil
}

func (s *Service) SubmitDeployment(ctx context.Context, req deployment.Request) (deployment.Deployment, error) {
	if req.ModelID == "" {
		return deployment.Deployment{}, fmt.Errorf("model_id is required")
	}
	if req.Target == "" {
		return deployment.Deployment{}, fmt.Errorf("target is required")
	}
	taskID, err := s.submit(ctx, "deployment.run", req)
	if err != nil {
		return deployment.Deployment{}, err
	}
	return deployment.Deployment{TaskID: taskID, ModelID: req.ModelID, ModelVersion: req.ModelVersion, Target: req.Target, Status: "queued"}, nil
}

func (s *Service) submit(ctx context.Context, taskType string, req any) (string, error) {
	raw, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	return s.gateway.Submit(ctx, taskType, map[string]string{"request_json": string(raw)})
}
