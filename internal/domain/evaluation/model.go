package evaluation

type Request struct {
	DatasetID     string            `json:"dataset_id"`
	ModelID       string            `json:"model_id"`
	Checkpoints   []string          `json:"checkpoints,omitempty"`
	Split         string            `json:"split,omitempty"`
	Metrics       []string          `json:"metrics,omitempty"`
	EvalConfig    map[string]string `json:"eval_config,omitempty"`
	SaveVisuals   bool              `json:"save_visuals"`
	FailureMining bool              `json:"failure_mining"`
}

type Run struct {
	TaskID    string `json:"task_id"`
	DatasetID string `json:"dataset_id"`
	ModelID   string `json:"model_id"`
	Status    string `json:"status"`
}
