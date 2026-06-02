package agentruntime

import (
	"sort"
	"strings"
	"sync"
	"time"
)

const defaultModelJobLimit = 100

type ModelJob struct {
	ID              string            `json:"id"`
	ParentID        string            `json:"parent_id,omitempty"`
	Kind            string            `json:"kind"`
	RepoID          string            `json:"repo_id"`
	LocalDir        string            `json:"local_dir"`
	Manifest        string            `json:"manifest"`
	VerifyOnly      bool              `json:"verify_only"`
	Status          string            `json:"status"`
	Message         string            `json:"message,omitempty"`
	Error           string            `json:"error,omitempty"`
	ProgressPercent int               `json:"progress_percent,omitempty"`
	CancelRequested bool              `json:"cancel_requested,omitempty"`
	Resumable       bool              `json:"resumable,omitempty"`
	Logs            []ModelJobLog     `json:"logs,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	StartedAt       *time.Time        `json:"started_at,omitempty"`
	FinishedAt      *time.Time        `json:"finished_at,omitempty"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

type ModelJobLog struct {
	At      time.Time `json:"at"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

type ModelJobStore interface {
	Create(job ModelJob) ModelJob
	Update(id string, mutate func(*ModelJob))
	Get(id string) (ModelJob, bool)
	List(limit int) []ModelJob
}

type InMemoryModelJobStore struct {
	mu   sync.RWMutex
	now  func() time.Time
	jobs map[string]ModelJob
}

func NewModelJobStore(now func() time.Time) *InMemoryModelJobStore {
	return NewInMemoryModelJobStore(now)
}

func NewInMemoryModelJobStore(now func() time.Time) *InMemoryModelJobStore {
	if now == nil {
		now = time.Now
	}
	return &InMemoryModelJobStore{now: now, jobs: map[string]ModelJob{}}
}

func (s *InMemoryModelJobStore) Create(job ModelJob) ModelJob {
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
	return job
}

func (s *InMemoryModelJobStore) Update(id string, mutate func(*ModelJob)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return
	}
	mutate(&job)
	job.ProgressPercent = normalizeProgress(job.ProgressPercent)
	job.UpdatedAt = s.now()
	s.jobs[id] = job
}

func (s *InMemoryModelJobStore) Get(id string) (ModelJob, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	return job, ok
}

func (s *InMemoryModelJobStore) List(limit int) []ModelJob {
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

func normalizeProgress(value int) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func appendModelJobLog(logs []ModelJobLog, at time.Time, level string, message string) []ModelJobLog {
	message = strings.TrimSpace(message)
	if message == "" {
		return logs
	}
	level = strings.TrimSpace(level)
	if level == "" {
		level = "info"
	}
	logs = append(logs, ModelJobLog{At: at, Level: level, Message: message})
	if len(logs) > 200 {
		return logs[len(logs)-200:]
	}
	return logs
}

func RecentModelJobLogs(job ModelJob, limit int) []ModelJobLog {
	if len(job.Logs) == 0 {
		return nil
	}
	limit = normalizeModelJobLimit(limit)
	start := len(job.Logs) - limit
	if start < 0 {
		start = 0
	}
	return append([]ModelJobLog(nil), job.Logs[start:]...)
}

func IsTerminalModelJobStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "succeeded", "failed", "canceled", "interrupted":
		return true
	default:
		return false
	}
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
