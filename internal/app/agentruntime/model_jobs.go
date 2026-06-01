package agentruntime

import (
	"sort"
	"sync"
	"time"
)

const defaultModelJobLimit = 100

type ModelJob struct {
	ID         string            `json:"id"`
	Kind       string            `json:"kind"`
	RepoID     string            `json:"repo_id"`
	LocalDir   string            `json:"local_dir"`
	Manifest   string            `json:"manifest"`
	VerifyOnly bool              `json:"verify_only"`
	Status     string            `json:"status"`
	Message    string            `json:"message,omitempty"`
	Error      string            `json:"error,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	StartedAt  *time.Time        `json:"started_at,omitempty"`
	FinishedAt *time.Time        `json:"finished_at,omitempty"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

type ModelJobStore struct {
	mu   sync.RWMutex
	now  func() time.Time
	jobs map[string]ModelJob
}

func NewModelJobStore(now func() time.Time) *ModelJobStore {
	if now == nil {
		now = time.Now
	}
	return &ModelJobStore{now: now, jobs: map[string]ModelJob{}}
}

func (s *ModelJobStore) Create(job ModelJob) ModelJob {
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
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	job.UpdatedAt = now
	s.jobs[job.ID] = job
	return job
}

func (s *ModelJobStore) Update(id string, mutate func(*ModelJob)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return
	}
	mutate(&job)
	job.UpdatedAt = s.now()
	s.jobs[id] = job
}

func (s *ModelJobStore) List(limit int) []ModelJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ModelJob, 0, len(s.jobs))
	for _, job := range s.jobs {
		out = append(out, job)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	limit = normalizeModelJobLimit(limit)
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func normalizeModelJobLimit(limit int) int {
	if limit <= 0 {
		return defaultModelJobLimit
	}
	if limit > 500 {
		return 500
	}
	return limit
}
