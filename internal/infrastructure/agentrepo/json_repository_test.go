package agentrepo

import (
	"context"
	"testing"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/domain/agent"
)

func TestBootstrapDefaultsAddsMissingWorkflowWithoutOverwritingExisting(t *testing.T) {
	ctx := context.Background()
	repo := NewJSONRepository(t.TempDir())
	now := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	existing := agent.WorkflowSpec{
		ID:        "human-loop-autolabel",
		Name:      "Custom Human Loop",
		Version:   "custom",
		Trigger:   "manual",
		Status:    agent.WorkflowStatusAvailable,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if _, err := repo.SaveWorkflow(ctx, existing); err != nil {
		t.Fatal(err)
	}
	if err := repo.BootstrapDefaults(ctx); err != nil {
		t.Fatal(err)
	}
	kept, err := repo.GetWorkflow(ctx, "human-loop-autolabel")
	if err != nil {
		t.Fatal(err)
	}
	if kept.Name != existing.Name || kept.Version != existing.Version {
		t.Fatalf("existing workflow was overwritten: %+v", kept)
	}
	added, err := repo.GetWorkflow(ctx, "data-to-deployment-lifecycle")
	if err != nil {
		t.Fatal(err)
	}
	if added.Status != agent.WorkflowStatusAvailable {
		t.Fatalf("expected available default workflow, got %+v", added)
	}
}
