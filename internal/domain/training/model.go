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
	DryRun            bool              `json:"dry_run"`
}

type Run struct {
	TaskID      string `json:"task_id"`
	DatasetID   string `json:"dataset_id"`
	TargetTask  string `json:"target_task"`
	ModelFamily string `json:"model_family"`
	Status      string `json:"status"`
}
