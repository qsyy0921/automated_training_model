package modelregistry

import "time"

type RegisterRequest struct {
	Name        string            `json:"name"`
	ModelFamily string            `json:"model_family"`
	Task        string            `json:"task"`
	ArtifactURI string            `json:"artifact_uri"`
	MetricsURI  string            `json:"metrics_uri,omitempty"`
	DatasetID   string            `json:"dataset_id,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	RuntimeSpec map[string]string `json:"runtime_spec,omitempty"`
	Description string            `json:"description,omitempty"`
}

type ModelVersion struct {
	ID          string            `json:"id"`
	Version     string            `json:"version"`
	TaskID      string            `json:"task_id"`
	Name        string            `json:"name"`
	ModelFamily string            `json:"model_family"`
	Task        string            `json:"task"`
	ArtifactURI string            `json:"artifact_uri"`
	MetricsURI  string            `json:"metrics_uri,omitempty"`
	DatasetID   string            `json:"dataset_id,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	RuntimeSpec map[string]string `json:"runtime_spec,omitempty"`
	Description string            `json:"description,omitempty"`
	Status      string            `json:"status"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}
