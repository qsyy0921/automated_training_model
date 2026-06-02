package intakerepo

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/qsyy0921/automated_training_model/internal/app/intakeapp"
	"github.com/qsyy0921/automated_training_model/internal/domain/channel"
)

type JSONRepository struct {
	mu          sync.RWMutex
	root        string
	plans       map[string]channel.DataIntakePlan
	attachments map[string]channel.Attachment
	workflows   map[string]intakeapp.IntakeWorkflow
}

func NewJSONRepository(root string) (*JSONRepository, error) {
	root = filepath.Clean(root)
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, err
	}
	repo := &JSONRepository{
		root:        root,
		plans:       map[string]channel.DataIntakePlan{},
		attachments: map[string]channel.Attachment{},
		workflows:   map[string]intakeapp.IntakeWorkflow{},
	}
	if err := repo.load(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (r *JSONRepository) SavePlan(ctx context.Context, plan channel.DataIntakePlan) (channel.DataIntakePlan, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plans[plan.ID] = plan
	return plan, r.persistLocked()
}

func (r *JSONRepository) SaveAttachment(ctx context.Context, attachment channel.Attachment) (channel.Attachment, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	r.attachments[attachment.ID] = attachment
	return attachment, r.persistLocked()
}

func (r *JSONRepository) ListPlans() []channel.DataIntakePlan {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return sortedPlans(r.plans)
}

func (r *JSONRepository) SaveWorkflow(ctx context.Context, workflow intakeapp.IntakeWorkflow) (intakeapp.IntakeWorkflow, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	r.workflows[workflow.ID] = workflow
	return workflow, r.persistLocked()
}

func (r *JSONRepository) GetWorkflow(ctx context.Context, id string) (intakeapp.IntakeWorkflow, bool, error) {
	_ = ctx
	r.mu.RLock()
	defer r.mu.RUnlock()
	workflow, ok := r.workflows[id]
	return workflow, ok, nil
}

func (r *JSONRepository) ListWorkflows(ctx context.Context, limit int) ([]intakeapp.IntakeWorkflow, error) {
	_ = ctx
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := sortedWorkflows(r.workflows)
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *JSONRepository) load() error {
	var plans []channel.DataIntakePlan
	if err := readJSONFile(r.plansPath(), &plans); err != nil {
		return err
	}
	for _, plan := range plans {
		if plan.ID != "" {
			r.plans[plan.ID] = plan
		}
	}
	var attachments []channel.Attachment
	if err := readJSONFile(r.attachmentsPath(), &attachments); err != nil {
		return err
	}
	for _, attachment := range attachments {
		if attachment.ID != "" {
			r.attachments[attachment.ID] = attachment
		}
	}
	var workflows []intakeapp.IntakeWorkflow
	if err := readJSONFile(r.workflowsPath(), &workflows); err != nil {
		return err
	}
	for _, workflow := range workflows {
		if workflow.ID != "" {
			r.workflows[workflow.ID] = workflow
		}
	}
	return nil
}

func (r *JSONRepository) persistLocked() error {
	if err := writeJSONFile(r.plansPath(), sortedPlans(r.plans)); err != nil {
		return err
	}
	if err := writeJSONFile(r.attachmentsPath(), sortedAttachments(r.attachments)); err != nil {
		return err
	}
	return writeJSONFile(r.workflowsPath(), sortedWorkflows(r.workflows))
}

func (r *JSONRepository) plansPath() string {
	return filepath.Join(r.root, "intake_plans.json")
}

func (r *JSONRepository) attachmentsPath() string {
	return filepath.Join(r.root, "intake_attachments.json")
}

func (r *JSONRepository) workflowsPath() string {
	return filepath.Join(r.root, "intake_workflows.json")
}

func sortedPlans(items map[string]channel.DataIntakePlan) []channel.DataIntakePlan {
	out := make([]channel.DataIntakePlan, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

func sortedAttachments(items map[string]channel.Attachment) []channel.Attachment {
	out := make([]channel.Attachment, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

func sortedWorkflows(items map[string]intakeapp.IntakeWorkflow) []intakeapp.IntakeWorkflow {
	out := make([]intakeapp.IntakeWorkflow, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out
}

func readJSONFile(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, value)
}

func writeJSONFile(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	encoder := json.NewEncoder(tmp)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	_ = os.Remove(path)
	return os.Rename(tmpPath, path)
}
