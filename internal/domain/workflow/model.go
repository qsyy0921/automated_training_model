package workflow

import "time"

type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
	TaskCanceled  TaskStatus = "canceled"
)

type TaskSpec struct {
	Type      string            `json:"type"`
	Payload   map[string]string `json:"payload"`
	CreatedAt time.Time         `json:"created_at"`
}

type Task struct {
	ID              string            `json:"id"`
	Type            string            `json:"type"`
	Status          TaskStatus        `json:"status"`
	Payload         map[string]string `json:"payload"`
	Message         string            `json:"message,omitempty"`
	Error           string            `json:"error,omitempty"`
	ProgressPercent int               `json:"progress_percent,omitempty"`
	Retryable       bool              `json:"retryable,omitempty"`
	Attempt         int               `json:"attempt,omitempty"`
	MaxAttempts     int               `json:"max_attempts,omitempty"`
	WorkerHeartbeat *TaskHeartbeat    `json:"worker_heartbeat,omitempty"`
	Artifacts       []TaskArtifact    `json:"artifacts,omitempty"`
	Stdout          string            `json:"stdout,omitempty"`
	Stderr          string            `json:"stderr,omitempty"`
	Logs            []TaskLog         `json:"logs,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	StartedAt       *time.Time        `json:"started_at,omitempty"`
	FinishedAt      *time.Time        `json:"finished_at,omitempty"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

type TaskHeartbeat struct {
	At      string `json:"at"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type TaskArtifact struct {
	Name     string            `json:"name"`
	URI      string            `json:"uri"`
	Kind     string            `json:"kind,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type TaskLog struct {
	At      time.Time `json:"at"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}
