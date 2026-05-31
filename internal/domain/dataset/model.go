package dataset

import "time"

type SourceType string

const (
	SourceLocalFolder SourceType = "local_folder"
	SourceUpload      SourceType = "upload_archive"
	SourceManifest    SourceType = "manifest"
)

type Dataset struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	SourceType     SourceType `json:"source_type"`
	MergeRoot      string     `json:"merge_root,omitempty"`
	FrameRoot      string     `json:"frame_root,omitempty"`
	MaskRoot       string     `json:"mask_root,omitempty"`
	AnnotationRoot string     `json:"annotation_root,omitempty"`
	ManifestPath   string     `json:"manifest_path,omitempty"`
	UploadPath     string     `json:"upload_path,omitempty"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type RegisterRequest struct {
	Name           string     `json:"name"`
	SourceType     SourceType `json:"source_type"`
	MergeRoot      string     `json:"merge_root,omitempty"`
	FrameRoot      string     `json:"frame_root,omitempty"`
	MaskRoot       string     `json:"mask_root,omitempty"`
	AnnotationRoot string     `json:"annotation_root,omitempty"`
	ManifestPath   string     `json:"manifest_path,omitempty"`
	UploadPath     string     `json:"upload_path,omitempty"`
}
