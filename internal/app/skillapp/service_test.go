package skillapp

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDraftApproveRejectLifecycle(t *testing.T) {
	root := t.TempDir()
	svc := NewService(func() time.Time { return time.Unix(10, 0) })
	draft, err := svc.Draft(DraftRequest{
		ID:      "qq-data-intake",
		Title:   "QQ Data Intake",
		Summary: "QQ attachment enters quarantine, scan, and intake plan.",
		Root:    root,
	})
	if err != nil {
		t.Fatal(err)
	}
	if draft.Status != "draft" || draft.Enabled {
		t.Fatalf("unexpected draft state: %+v", draft)
	}
	if _, err := os.Stat(filepath.Join(root, "qq-data-intake", "SKILL.md")); err != nil {
		t.Fatal(err)
	}

	approved, err := svc.Approve(ReviewRequest{ID: "qq-data-intake", Root: root, By: "tester", Note: "reviewed"})
	if err != nil {
		t.Fatal(err)
	}
	if approved.Status != "approved" || approved.Enabled {
		t.Fatalf("approval must not auto-enable skill: %+v", approved)
	}
	listed, err := svc.List(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].Status != "approved" || listed[0].Enabled {
		t.Fatalf("unexpected listed draft: %+v", listed)
	}

	rejected, err := svc.Reject(ReviewRequest{ID: "qq-data-intake", Root: root, By: "tester", Note: "needs tests"})
	if err != nil {
		t.Fatal(err)
	}
	if rejected.Status != "rejected" || rejected.Enabled {
		t.Fatalf("unexpected rejected record: %+v", rejected)
	}
}

func TestDraftRejectsSecretsAndPathEscape(t *testing.T) {
	svc := NewService(nil)
	if _, err := svc.Draft(DraftRequest{ID: "bad/escape", Root: t.TempDir(), Summary: "ok"}); err == nil {
		t.Fatal("expected invalid skill id to fail")
	}
	if _, err := svc.Draft(DraftRequest{ID: "secret-demo", Root: t.TempDir(), Summary: "use sk-test-token"}); err == nil {
		t.Fatal("expected secret-like summary to fail")
	}
}

func TestApproveMissingDraftFails(t *testing.T) {
	svc := NewService(nil)
	if _, err := svc.Approve(ReviewRequest{ID: "missing", Root: t.TempDir(), By: "tester"}); err == nil {
		t.Fatal("expected missing draft approval to fail")
	}
}
