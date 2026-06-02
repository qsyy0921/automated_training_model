package intakerepo

import (
	"context"
	"testing"
	"time"

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
