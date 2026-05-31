package media

import "github.com/qsyy0921/_video_label_tool/labelserver/internal/domain/tracking"

type ClassCount struct {
	ClassID   int    `json:"class_id"`
	ClassName string `json:"class_name"`
	Color     string `json:"color"`
	Count     int    `json:"count"`
}

type VideoSummary struct {
	Scene           string       `json:"scene"`
	FrameCount      int          `json:"frame_count"`
	Rows            int          `json:"rows"`
	TrackCount      int          `json:"track_count"`
	AnnotationCount int          `json:"annotation_count"`
	Classes         []ClassCount `json:"classes"`
	HasPreview      bool         `json:"has_preview"`
}

type Video struct {
	Scene             string
	FrameCount        int
	Rows              int
	FrameNames        map[int]string
	ClassCounts       map[int]int
	Tracks            []tracking.Track
	Boxes             map[int][]tracking.Box
	AnomalyFrameCount int
}
