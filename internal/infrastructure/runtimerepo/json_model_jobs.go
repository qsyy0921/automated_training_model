package runtimerepo

import (
	"sort"
	"sync"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/app/agentruntime"
)

type JSONModelJobStore struct {
	mu   sync.RWMutex
	path string
	now  func() time.Time
	jobs map[string]agentruntime.ModelJob
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
