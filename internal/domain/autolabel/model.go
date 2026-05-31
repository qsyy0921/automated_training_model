package autolabel

type Request struct {
	DatasetID     string   `json:"dataset_id"`
	VideoIDs      []string `json:"video_ids,omitempty"`
	TaskTypes     []string `json:"task_types"`
	ModelProfile  string   `json:"model_profile,omitempty"`
	Prompt        string   `json:"prompt,omitempty"`
	RequireReview bool     `json:"require_review"`
}

type Job struct {
	TaskID    string `json:"task_id"`
	DatasetID string `json:"dataset_id"`
	Status    string `json:"status"`
}
