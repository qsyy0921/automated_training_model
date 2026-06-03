package runtimerepo

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/app/agentruntime"
	"github.com/qsyy0921/automated_training_model/internal/infrastructure/artifactmanifest"
)

type JSONModelJobStore struct {
	mu   sync.RWMutex
	path string
	now  func() time.Time
	jobs map[string]agentruntime.ModelJob
}

type modelJobArtifactManifestFile struct {
	SchemaVersion   string                          `json:"schema_version"`
	JobID           string                          `json:"job_id"`
	ParentID        string                          `json:"parent_id,omitempty"`
	Kind            string                          `json:"kind"`
	RepoID          string                          `json:"repo_id,omitempty"`
	Status          string                          `json:"status"`
	Message         string                          `json:"message,omitempty"`
	Retryable       bool                            `json:"retryable,omitempty"`
	Attempt         int                             `json:"attempt,omitempty"`
	MaxAttempts     int                             `json:"max_attempts,omitempty"`
	WorkerHeartbeat *agentruntime.ModelJobHeartbeat `json:"worker_heartbeat,omitempty"`
	ArtifactSummary artifactmanifest.Summary        `json:"artifact_summary"`
	Artifacts       []agentruntime.ModelJobArtifact `json:"artifacts,omitempty"`
	Metadata        map[string]string               `json:"metadata,omitempty"`
	CreatedAt       time.Time                       `json:"created_at"`
	UpdatedAt       time.Time                       `json:"updated_at"`
	FinishedAt      *time.Time                      `json:"finished_at,omitempty"`
	ArchivedAt      time.Time                       `json:"archived_at"`
}

func NewJSONModelJobStore(path string, now func() time.Time) (*JSONModelJobStore, error) {
	if now == nil {
		now = time.Now
	}
	store := &JSONModelJobStore{
		path: path,
		now:  now,
		jobs: map[string]agentruntime.ModelJob{},
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *JSONModelJobStore) Create(job agentruntime.ModelJob) agentruntime.ModelJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if job.ID == "" {
		job.ID = "model-job-" + now.Format("20060102150405.000000000")
	}
	if job.Kind == "" {
		job.Kind = "model.download_hf"
	}
	if job.Status == "" {
		job.Status = "queued"
	}
	if job.ProgressPercent < 0 {
		job.ProgressPercent = 0
	}
	if job.ProgressPercent > 100 {
		job.ProgressPercent = 100
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	job.UpdatedAt = now
	s.jobs[job.ID] = job
	_ = s.persistLocked()
	return job
}

func (s *JSONModelJobStore) Update(id string, mutate func(*agentruntime.ModelJob)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return
	}
	mutate(&job)
	if job.ProgressPercent < 0 {
		job.ProgressPercent = 0
	}
	if job.ProgressPercent > 100 {
		job.ProgressPercent = 100
	}
	job.UpdatedAt = s.now()
	s.jobs[id] = job
	_ = s.persistLocked()
}

func (s *JSONModelJobStore) Get(id string) (agentruntime.ModelJob, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	return job, ok
}

func (s *JSONModelJobStore) List(limit int) []agentruntime.ModelJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := sortedModelJobs(s.jobs)
	limit = normalizeModelJobLimit(limit)
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (s *JSONModelJobStore) Lineage(id string) []agentruntime.ModelJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]agentruntime.ModelJob, 0, len(s.jobs))
	for _, job := range s.jobs {
		items = append(items, job)
	}
	return modelJobLineage(items, id)
}

func (s *JSONModelJobStore) load() error {
	var rows []agentruntime.ModelJob
	if err := readJSONFile(s.path, &rows); err != nil {
		return err
	}
	now := s.now()
	for _, row := range rows {
		if row.ID == "" {
			continue
		}
		if row.Status == "queued" || row.Status == "running" {
			finished := now
			row.Status = "interrupted"
			row.Message = "server restarted before model job completed; submit a new download job to resume via HuggingFace cache"
			row.Resumable = true
			if row.ProgressPercent < 100 {
				row.ProgressPercent = 0
			}
			row.FinishedAt = &finished
			row.UpdatedAt = now
		}
		s.jobs[row.ID] = row
	}
	if len(rows) > 0 {
		return s.persistLocked()
	}
	return nil
}

func (s *JSONModelJobStore) persistLocked() error {
	return writeJSONFile(s.path, sortedModelJobs(s.jobs))
}

func (s *JSONModelJobStore) WriteArtifactManifest(job agentruntime.ModelJob) (string, error) {
	if strings.TrimSpace(job.ID) == "" {
		return "", nil
	}
	if len(job.Artifacts) == 0 && strings.TrimSpace(job.Metadata["artifact_count"]) == "" {
		return "", nil
	}
	path := s.artifactManifestPath(job.ID)
	summary := artifactmanifest.BuildSummary(modelJobManifestEntries(job.Artifacts))
	if summary.ArtifactCount == 0 {
		if count, err := strconv.Atoi(strings.TrimSpace(job.Metadata["artifact_count"])); err == nil && count > 0 {
			summary.ArtifactCount = count
		}
	}
	payload := modelJobArtifactManifestFile{
		SchemaVersion:   artifactmanifest.SchemaVersionV1,
		JobID:           job.ID,
		ParentID:        job.ParentID,
		Kind:            job.Kind,
		RepoID:          job.RepoID,
		Status:          job.Status,
		Message:         job.Message,
		Retryable:       job.Retryable,
		Attempt:         job.Attempt,
		MaxAttempts:     job.MaxAttempts,
		WorkerHeartbeat: job.WorkerHeartbeat,
		ArtifactSummary: summary,
		Artifacts:       job.Artifacts,
		Metadata:        job.Metadata,
		CreatedAt:       job.CreatedAt,
		UpdatedAt:       job.UpdatedAt,
		FinishedAt:      job.FinishedAt,
		ArchivedAt:      s.now(),
	}
	if err := writeJSONFile(path, payload); err != nil {
		return "", err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path), nil
	}
	return abs, nil
}

func (s *JSONModelJobStore) artifactManifestPath(jobID string) string {
	base := filepath.Dir(s.path)
	return filepath.Join(base, "artifacts", jobID+".artifact_manifest.json")
}

func sortedModelJobs(items map[string]agentruntime.ModelJob) []agentruntime.ModelJob {
	out := make([]agentruntime.ModelJob, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out
}

func normalizeModelJobLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	if limit > 500 {
		return 500
	}
	return limit
}

func modelJobManifestEntries(items []agentruntime.ModelJobArtifact) []artifactmanifest.Entry {
	if len(items) == 0 {
		return nil
	}
	out := make([]artifactmanifest.Entry, 0, len(items))
	for _, item := range items {
		out = append(out, artifactmanifest.Entry{
			Name:     item.Name,
			URI:      item.URI,
			Kind:     item.Kind,
			Metadata: item.Metadata,
		})
	}
	return out
}

func modelJobLineage(items []agentruntime.ModelJob, id string) []agentruntime.ModelJob {
	index := map[string]agentruntime.ModelJob{}
	for _, item := range items {
		if item.ID != "" {
			index[item.ID] = item
		}
	}
	current, ok := index[id]
	if !ok {
		return nil
	}
	rootID := current.ID
	for {
		parentID := strings.TrimSpace(index[rootID].ParentID)
		if parentID == "" {
			break
		}
		parent, ok := index[parentID]
		if !ok {
			break
		}
		rootID = parent.ID
	}
	out := make([]agentruntime.ModelJob, 0, len(items))
	for _, item := range items {
		if modelJobRootID(index, item.ID) == rootID {
			out = append(out, item)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			if out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
				return out[i].ID < out[j].ID
			}
			return out[i].UpdatedAt.Before(out[j].UpdatedAt)
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

func modelJobRootID(index map[string]agentruntime.ModelJob, id string) string {
	currentID := strings.TrimSpace(id)
	seen := map[string]bool{}
	for currentID != "" && !seen[currentID] {
		seen[currentID] = true
		current, ok := index[currentID]
		if !ok || strings.TrimSpace(current.ParentID) == "" {
			break
		}
		currentID = strings.TrimSpace(current.ParentID)
	}
	return currentID
}
