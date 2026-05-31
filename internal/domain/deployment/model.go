package deployment

type Request struct {
	ModelID        string            `json:"model_id"`
	ModelVersion   string            `json:"model_version,omitempty"`
	Target         string            `json:"target"`
	Runtime        string            `json:"runtime"`
	Replicas       int               `json:"replicas,omitempty"`
	ResourceClass  string            `json:"resource_class,omitempty"`
	Strategy       string            `json:"strategy,omitempty"`
	Config         map[string]string `json:"config,omitempty"`
	CanaryPercent  int               `json:"canary_percent,omitempty"`
	RollbackPolicy string            `json:"rollback_policy,omitempty"`
}

type Deployment struct {
	TaskID       string `json:"task_id"`
	ModelID      string `json:"model_id"`
	ModelVersion string `json:"model_version,omitempty"`
	Target       string `json:"target"`
	Status       string `json:"status"`
}
