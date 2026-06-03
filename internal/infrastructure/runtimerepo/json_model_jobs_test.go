package runtimerepo

import (
	"encoding/json"
	"os"
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

func TestJSONModelJobStoreWritesArtifactManifest(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "model_jobs.json")
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	store, err := NewJSONModelJobStore(path, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	finished := now.Add(time.Minute)
	job := store.Create(agentruntime.ModelJob{
		ID:              "model-job-artifact",
		Kind:            "model.verify_hf",
		RepoID:          "nvidia/LocateAnything-3B",
		Status:          "succeeded",
		Message:         "verify complete",
		Retryable:       false,
		Attempt:         1,
		MaxAttempts:     1,
		WorkerHeartbeat: &agentruntime.ModelJobHeartbeat{At: "2026-06-03T12:00:10Z", Status: "completed", Message: "done"},
		Artifacts: []agentruntime.ModelJobArtifact{
			{Name: "request", URI: "artifact://verify/model-job-artifact/request", Kind: "model.verify_hf.request", Metadata: map[string]string{"role": "request"}},
			{Name: "result", URI: "artifact://verify/model-job-artifact/result", Kind: "model.verify_hf.result", Metadata: map[string]string{"role": "result", "execution_mode": "worker-verify"}},
		},
		Metadata:   map[string]string{"execution_path": "python-worker", "artifact_count": "1"},
		CreatedAt:  now,
		FinishedAt: &finished,
	})
	manifestPath, err := store.WriteArtifactManifest(job)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(manifestPath) != "model-job-artifact.artifact_manifest.json" {
		t.Fatalf("unexpected manifest path: %s", manifestPath)
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("parse manifest: %v\n%s", err, string(data))
	}
	if payload["schema_version"] != "artifact-manifest/v1" || payload["job_id"] != "model-job-artifact" || payload["kind"] != "model.verify_hf" {
		t.Fatalf("unexpected manifest payload: %s", string(data))
	}
	summary, ok := payload["artifact_summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected artifact_summary, got %s", string(data))
	}
	if summary["artifact_count"] != float64(2) {
		t.Fatalf("expected artifact_count=2, got %v", summary["artifact_count"])
	}
	roleCounts, ok := summary["role_counts"].(map[string]any)
	if !ok || roleCounts["request"] != float64(1) || roleCounts["result"] != float64(1) {
		t.Fatalf("unexpected role_counts: %+v", summary["role_counts"])
	}
	primary, ok := summary["primary_artifact"].(map[string]any)
	if !ok || primary["name"] != "result" || primary["role"] != "result" || primary["execution_mode"] != "worker-verify" {
		t.Fatalf("unexpected primary_artifact: %+v", summary["primary_artifact"])
	}
}

func TestJSONModelJobStoreLineageReturnsResumeFamily(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model_jobs.json")
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	store, err := NewJSONModelJobStore(path, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	store.Create(agentruntime.ModelJob{
		ID:        "job-1",
		Kind:      "model.download_hf",
		RepoID:    "repo/a",
		Status:    "failed",
		CreatedAt: now,
	})
	store.Create(agentruntime.ModelJob{
		ID:        "job-2",
		ParentID:  "job-1",
		Kind:      "model.download_hf",
		RepoID:    "repo/a",
		Status:    "queued",
		CreatedAt: now.Add(time.Minute),
	})
	lineage := store.Lineage("job-2")
	if len(lineage) != 2 || lineage[0].ID != "job-1" || lineage[1].ParentID != "job-1" {
		t.Fatalf("unexpected lineage: %+v", lineage)
	}
}
