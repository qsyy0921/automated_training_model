package agentapp

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
	"github.com/qsyy0921/automated_training_model/internal/domain/agent"
)

type Repository interface {
	ListAgents(ctx context.Context) ([]agent.AgentSpec, error)
	GetAgent(ctx context.Context, id string) (*agent.AgentSpec, error)
	SaveAgent(ctx context.Context, spec agent.AgentSpec) (agent.AgentSpec, error)
	ListTools(ctx context.Context) ([]agent.ToolSpec, error)
	GetTool(ctx context.Context, id string) (*agent.ToolSpec, error)
	SaveTool(ctx context.Context, spec agent.ToolSpec) (agent.ToolSpec, error)
	ListWorkflows(ctx context.Context) ([]agent.WorkflowSpec, error)
	GetWorkflow(ctx context.Context, id string) (*agent.WorkflowSpec, error)
	SaveWorkflow(ctx context.Context, spec agent.WorkflowSpec) (agent.WorkflowSpec, error)
	ListRuns(ctx context.Context) ([]agent.WorkflowRun, error)
	SaveRun(ctx context.Context, run agent.WorkflowRun) (agent.WorkflowRun, error)
	ListAuditEvents(ctx context.Context, limit int) ([]agent.AuditEvent, error)
	AppendAuditEvent(ctx context.Context, event agent.AuditEvent) error
	BootstrapDefaults(ctx context.Context) error
}

type Service struct {
	repo    Repository
	gateway workflowapp.ModelGateway
}

func NewService(repo Repository, gateway workflowapp.ModelGateway) *Service {
	return &Service{repo: repo, gateway: gateway}
}

func (s *Service) BootstrapDefaults(ctx context.Context) error {
	return s.repo.BootstrapDefaults(ctx)
}

func (s *Service) ListAgents(ctx context.Context) ([]agent.AgentSpec, error) {
	return s.repo.ListAgents(ctx)
}

func (s *Service) GetAgent(ctx context.Context, id string) (*agent.AgentSpec, error) {
	return s.repo.GetAgent(ctx, strings.TrimSpace(id))
}

func (s *Service) SaveAgent(ctx context.Context, spec agent.AgentSpec) (agent.AgentSpec, error) {
	now := time.Now()
	spec.ID = normalizeID(spec.ID, spec.Name, "agent")
	spec.Name = strings.TrimSpace(spec.Name)
	if spec.Name == "" {
		return agent.AgentSpec{}, fmt.Errorf("agent name is required")
	}
	if spec.Kind == "" {
		spec.Kind = "python-worker"
	}
	if spec.Version == "" {
		spec.Version = "v0.1"
	}
	if spec.Status == "" {
		spec.Status = agent.AgentStatusAvailable
	}
	if spec.CreatedAt.IsZero() {
		spec.CreatedAt = now
	}
	spec.UpdatedAt = now
	saved, err := s.repo.SaveAgent(ctx, spec)
	if err != nil {
		return agent.AgentSpec{}, err
	}
	_ = s.audit(ctx, "system", "agent.save", "agent", saved.ID, nil)
	return saved, nil
}

func (s *Service) ListTools(ctx context.Context) ([]agent.ToolSpec, error) {
	return s.repo.ListTools(ctx)
}

func (s *Service) SaveTool(ctx context.Context, spec agent.ToolSpec) (agent.ToolSpec, error) {
	now := time.Now()
	spec.ID = normalizeID(spec.ID, spec.Name, "tool")
	spec.Name = strings.TrimSpace(spec.Name)
	if spec.Name == "" {
		return agent.ToolSpec{}, fmt.Errorf("tool name is required")
	}
	if spec.Kind == "" {
		spec.Kind = "python-tool"
	}
	if spec.Version == "" {
		spec.Version = "v0.1"
	}
	if spec.Status == "" {
		spec.Status = agent.ToolStatusAvailable
	}
	if spec.CreatedAt.IsZero() {
		spec.CreatedAt = now
	}
	spec.UpdatedAt = now
	saved, err := s.repo.SaveTool(ctx, spec)
	if err != nil {
		return agent.ToolSpec{}, err
	}
	_ = s.audit(ctx, "system", "tool.save", "tool", saved.ID, nil)
	return saved, nil
}

func (s *Service) ListWorkflows(ctx context.Context) ([]agent.WorkflowSpec, error) {
	return s.repo.ListWorkflows(ctx)
}

func (s *Service) GetWorkflow(ctx context.Context, id string) (*agent.WorkflowSpec, error) {
	return s.repo.GetWorkflow(ctx, strings.TrimSpace(id))
}

func (s *Service) SaveWorkflow(ctx context.Context, spec agent.WorkflowSpec) (agent.WorkflowSpec, error) {
	now := time.Now()
	spec.ID = normalizeID(spec.ID, spec.Name, "workflow")
	spec.Name = strings.TrimSpace(spec.Name)
	if spec.Name == "" {
		return agent.WorkflowSpec{}, fmt.Errorf("workflow name is required")
	}
	if spec.Version == "" {
		spec.Version = "v0.1"
	}
	if spec.Trigger == "" {
		spec.Trigger = "manual"
	}
	if spec.Status == "" {
		spec.Status = agent.WorkflowStatusAvailable
	}
	if spec.CreatedAt.IsZero() {
		spec.CreatedAt = now
	}
	spec.UpdatedAt = now
	saved, err := s.repo.SaveWorkflow(ctx, spec)
	if err != nil {
		return agent.WorkflowSpec{}, err
	}
	_ = s.audit(ctx, "system", "workflow.save", "workflow", saved.ID, nil)
	return saved, nil
}

func (s *Service) SubmitWorkflowRun(ctx context.Context, req agent.RunRequest) (agent.WorkflowRun, error) {
	req.WorkflowID = strings.TrimSpace(req.WorkflowID)
	if req.WorkflowID == "" {
		return agent.WorkflowRun{}, fmt.Errorf("workflow_id is required")
	}
	workflow, err := s.repo.GetWorkflow(ctx, req.WorkflowID)
	if err != nil {
		return agent.WorkflowRun{}, err
	}
	if workflow.Status != agent.WorkflowStatusAvailable {
		return agent.WorkflowRun{}, fmt.Errorf("workflow is not available: %s", workflow.ID)
	}
	raw, err := json.Marshal(req)
	if err != nil {
		return agent.WorkflowRun{}, err
	}
	taskID, err := s.gateway.Submit(ctx, "agent.workflow.run", map[string]string{
		"workflow_id":  workflow.ID,
		"dataset_id":   strings.TrimSpace(req.DatasetID),
		"scene":        strings.TrimSpace(req.Scene),
		"dry_run":      strconv.FormatBool(req.DryRun),
		"request_json": string(raw),
	})
	if err != nil {
		return agent.WorkflowRun{}, err
	}
	now := time.Now()
	run := agent.WorkflowRun{
		ID:         "run_" + randomSuffix(),
		TaskID:     taskID,
		WorkflowID: workflow.ID,
		DatasetID:  strings.TrimSpace(req.DatasetID),
		Scene:      strings.TrimSpace(req.Scene),
		Status:     "queued",
		Params:     compactMap(req.Params),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	saved, err := s.repo.SaveRun(ctx, run)
	if err != nil {
		return agent.WorkflowRun{}, err
	}
	_ = s.audit(ctx, "system", "workflow.run.submit", "workflow_run", saved.ID, map[string]string{
		"workflow_id": workflow.ID,
		"task_id":     taskID,
	})
	return saved, nil
}

func (s *Service) ListRuns(ctx context.Context) ([]agent.WorkflowRun, error) {
	return s.repo.ListRuns(ctx)
}

func (s *Service) ListAuditEvents(ctx context.Context, limit int) ([]agent.AuditEvent, error) {
	return s.repo.ListAuditEvents(ctx, limit)
}

func (s *Service) ListEnforcementPoints(ctx context.Context) ([]agent.EnforcementPoint, error) {
	return agent.DefaultEnforcementPoints(), nil
}

func (s *Service) ListDataGovernancePolicies(ctx context.Context) ([]agent.DataGovernancePolicy, error) {
	return agent.DefaultDataGovernancePolicies(), nil
}

func (s *Service) ListReleasePolicies(ctx context.Context) ([]agent.ReleasePolicy, error) {
	return agent.DefaultReleasePolicies(), nil
}

func (s *Service) ListRuntimePolicies(ctx context.Context) ([]agent.RuntimePolicy, error) {
	return agent.DefaultRuntimePolicies(), nil
}

func (s *Service) GetControlSurface(ctx context.Context) (agent.ControlSurface, error) {
	return agent.DefaultControlSurface(), nil
}

func (s *Service) audit(ctx context.Context, actor string, action string, resourceType string, resourceID string, details map[string]string) error {
	return s.repo.AppendAuditEvent(ctx, agent.AuditEvent{
		ID:           "audit_" + randomSuffix(),
		Actor:        actor,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Details:      compactMap(details),
		CreatedAt:    time.Now(),
	})
}

func normalizeID(id string, name string, fallback string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		id = name
	}
	id = strings.ToLower(strings.TrimSpace(id))
	id = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, id)
	id = strings.Trim(id, "-")
	if id == "" {
		id = fallback + "-" + randomSuffix()
	}
	return id
}

func randomSuffix() string {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return hex.EncodeToString(buf)
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
