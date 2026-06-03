package evaluation

type Request struct {
	DatasetID        string            `json:"dataset_id"`
	ModelID          string            `json:"model_id"`
	Checkpoints      []string          `json:"checkpoints,omitempty"`
	Split            string            `json:"split,omitempty"`
	Metrics          []string          `json:"metrics,omitempty"`
	EvalConfig       map[string]string `json:"eval_config,omitempty"`
	ExecutionRecipe  string            `json:"execution_recipe,omitempty"`
	ExecutionCommand []string          `json:"execution_command,omitempty"`
	ExecutionCwd     string            `json:"execution_cwd,omitempty"`
	ExecutionEnv     map[string]string `json:"execution_env,omitempty"`
	ExecutionTimeout int               `json:"execution_timeout_seconds,omitempty"`
	SaveVisuals      bool              `json:"save_visuals"`
	FailureMining    bool              `json:"failure_mining"`
	DryRun           bool              `json:"dry_run"`
}

type Run struct {
	TaskID    string `json:"task_id"`
	DatasetID string `json:"dataset_id"`
	ModelID   string `json:"model_id"`
	Status    string `json:"status"`
}
