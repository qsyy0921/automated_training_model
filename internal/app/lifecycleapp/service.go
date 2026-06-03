package lifecycleapp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/app/workflowapp"
	"github.com/qsyy0921/automated_training_model/internal/domain/autolabel"
	"github.com/qsyy0921/automated_training_model/internal/domain/deployment"
	"github.com/qsyy0921/automated_training_model/internal/domain/evaluation"
	"github.com/qsyy0921/automated_training_model/internal/domain/modelregistry"
	"github.com/qsyy0921/automated_training_model/internal/domain/training"
	"github.com/qsyy0921/automated_training_model/internal/domain/workflow"
)

type ModelRepository interface {
	List(ctx context.Context) ([]modelregistry.ModelVersion, error)
	Get(ctx context.Context, id string) (*modelregistry.ModelVersion, error)
	Save(ctx context.Context, model modelregistry.ModelVersion) (modelregistry.ModelVersion, error)
}

type Service struct {
	gateway workflowapp.ModelGateway
	models  ModelRepository
}

func NewService(gateway workflowapp.ModelGateway) *Service {
	return &Service{gateway: gateway}
}

func NewServiceWithModelRepository(gateway workflowapp.ModelGateway, models ModelRepository) *Service {
	return &Service{gateway: gateway, models: models}
}

func (s *Service) TaskStatus(ctx context.Context, id string) (*workflow.Task, error) {
	return s.gateway.Status(ctx, id)
}

func (s *Service) ListTasks(ctx context.Context, limit int) ([]workflow.Task, error) {
	return s.gateway.List(ctx, limit)
}

func (s *Service) TaskLineage(ctx context.Context, id string) ([]workflow.Task, error) {
	return s.gateway.Lineage(ctx, id)
}

func (s *Service) CancelTask(ctx context.Context, id string) error {
	return s.gateway.Cancel(ctx, id)
}

func (s *Service) ResumeTask(ctx context.Context, id string) (*workflow.Task, error) {
	newID, err := s.gateway.Resume(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.gateway.Status(ctx, newID)
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

func (s *Service) ListModels(ctx context.Context) ([]modelregistry.ModelVersion, error) {
	if s.models == nil {
		return []modelregistry.ModelVersion{}, nil
	}
	return s.models.List(ctx)
}

func (s *Service) GetModel(ctx context.Context, id string) (*modelregistry.ModelVersion, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("model id is required")
	}
	if s.models == nil {
		return nil, fmt.Errorf("model repository is not configured")
	}
	return s.models.Get(ctx, id)
}

func (s *Service) RegisterModel(ctx context.Context, req modelregistry.RegisterRequest) (modelregistry.ModelVersion, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.ArtifactURI = strings.TrimSpace(req.ArtifactURI)
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
	now := time.Now()
	model := modelregistry.ModelVersion{
		ID:          newModelID(req.Name),
		Version:     "v" + now.Format("20060102-150405"),
		TaskID:      taskID,
		Name:        req.Name,
		ModelFamily: strings.TrimSpace(req.ModelFamily),
		Task:        strings.TrimSpace(req.Task),
		ArtifactURI: req.ArtifactURI,
		MetricsURI:  strings.TrimSpace(req.MetricsURI),
		DatasetID:   strings.TrimSpace(req.DatasetID),
		Tags:        compactStrings(req.Tags),
		RuntimeSpec: compactMap(req.RuntimeSpec),
		Description: strings.TrimSpace(req.Description),
		Status:      "registered",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if s.models == nil {
		model.Status = "queued"
		return model, nil
	}
	return s.models.Save(ctx, model)
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
	payload := map[string]string{"request_json": string(raw)}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err == nil {
		for _, key := range []string{"dataset_id", "model_id", "target_task", "model_family", "target", "runtime", "dry_run", "execution_recipe"} {
			if value := strings.TrimSpace(fmt.Sprint(fields[key])); value != "" && value != "<nil>" {
				payload[key] = value
			}
		}
	}
	return s.gateway.Submit(ctx, taskType, payload)
}

func newModelID(name string) string {
	slug := strings.ToLower(strings.TrimSpace(name))
	slug = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return '-'
	}, slug)
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "model"
	}
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return slug + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return slug + "-" + hex.EncodeToString(buf)
}

func compactStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func compactMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := map[string]string{}
	for key, value := range values {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
