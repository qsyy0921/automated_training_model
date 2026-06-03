package autolabel

type Request struct {
	DatasetID        string            `json:"dataset_id"`
	VideoIDs         []string          `json:"video_ids,omitempty"`
	TaskTypes        []string          `json:"task_types"`
	ModelProfile     string            `json:"model_profile,omitempty"`
	Prompt           string            `json:"prompt,omitempty"`
	ExecutionRecipe  string            `json:"execution_recipe,omitempty"`
	ExecutionCommand []string          `json:"execution_command,omitempty"`
	ExecutionCwd     string            `json:"execution_cwd,omitempty"`
	ExecutionEnv     map[string]string `json:"execution_env,omitempty"`
	ExecutionTimeout int               `json:"execution_timeout_seconds,omitempty"`
	RequireReview    bool              `json:"require_review"`
	DryRun           bool              `json:"dry_run"`
}

type Job struct {
	TaskID    string `json:"task_id"`
	DatasetID string `json:"dataset_id"`
	Status    string `json:"status"`
}
