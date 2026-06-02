package runtimerepo

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/app/agentruntime"
)

func TestJSONModelJobStoreRestoresFinishedJobs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model_jobs.json")
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	store, err := NewJSONModelJobStore(path, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	finished := now.Add(time.Minute)
	store.Create(agentruntime.ModelJob{
		ID:         "model-job-1",
		Kind:       "model.download_hf",
		RepoID:     "nvidia/LocateAnything-3B",
		LocalDir:   "data_lake/models/artifacts/huggingface/nvidia/LocateAnything-3B",
		Manifest:   "data_lake/catalog/models/nvidia_LocateAnything-3B.download.json",
		Status:     "succeeded",
		Message:    "download complete",
		FinishedAt: &finished,
	})

	restored, err := NewJSONModelJobStore(path, func() time.Time { return now.Add(time.Hour) })
	if err != nil {
		t.Fatal(err)
	}
	jobs := restored.List(10)
	if len(jobs) != 1 {
		t.Fatalf("expected one restored job, got %+v", jobs)
	}
	if jobs[0].Status != "succeeded" || jobs[0].RepoID != "nvidia/LocateAnything-3B" {
		t.Fatalf("unexpected restored job: %+v", jobs[0])
	}
}

func TestJSONModelJobStoreMarksRunningJobsInterruptedOnRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model_jobs.json")
	now := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	store, err := NewJSONModelJobStore(path, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	started := now.Add(time.Second)
	store.Create(agentruntime.ModelJob{
		ID:        "model-job-running",
		RepoID:    "nvidia/LocateAnything-3B",
		Status:    "running",
		StartedAt: &started,
	})

	restartedAt := now.Add(time.Hour)
	restored, err := NewJSONModelJobStore(path, func() time.Time { return restartedAt })
	if err != nil {
		t.Fatal(err)
	}
	jobs := restored.List(10)
	if len(jobs) != 1 {
		t.Fatalf("expected one restored job, got %+v", jobs)
	}
	if jobs[0].Status != "interrupted" {
		t.Fatalf("expected interrupted job after restart, got %+v", jobs[0])
	}
	if !jobs[0].Resumable {
		t.Fatalf("expected interrupted job to be resumable, got %+v", jobs[0])
	}
	if jobs[0].FinishedAt == nil || !jobs[0].FinishedAt.Equal(restartedAt) {
		t.Fatalf("expected interrupted job finished_at to be restart time, got %+v", jobs[0])
	}
	got, ok := restored.Get("model-job-running")
	if !ok || got.Status != "interrupted" {
		t.Fatalf("expected get interrupted job, got ok=%v job=%+v", ok, got)
	}
}
