package training

type Request struct {
	DatasetID         string            `json:"dataset_id"`
	AnnotationVersion string            `json:"annotation_version,omitempty"`
	TargetTask        string            `json:"target_task"`
	ModelFamily       string            `json:"model_family"`
	BaseModel         string            `json:"base_model,omitempty"`
	SplitConfig       string            `json:"split_config,omitempty"`
	TrainingConfig    map[string]string `json:"training_config,omitempty"`
	OutputRegistry    string            `json:"output_registry,omitempty"`
	ExecutionCommand  []string          `json:"execution_command,omitempty"`
	ExecutionCwd      string            `json:"execution_cwd,omitempty"`
	ExecutionEnv      map[string]string `json:"execution_env,omitempty"`
	ExecutionTimeout  int               `json:"execution_timeout_seconds,omitempty"`
	DryRun            bool              `json:"dry_run"`
}

type Run struct {
	TaskID      string `json:"task_id"`
	DatasetID   string `json:"dataset_id"`
	TargetTask  string `json:"target_task"`
	ModelFamily string `json:"model_family"`
	Status      string `json:"status"`
}
