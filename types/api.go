package types

import (
	"github.com/qsyy0921/video_label_tool/labelserver/domain/media"
	"github.com/qsyy0921/video_label_tool/labelserver/domain/tracking"
)

type ListVideosResponse struct {
	Videos []media.VideoSummary `json:"videos"`
}

type VideoMetaResponse struct {
	Scene             string             `json:"scene"`
	FrameCount        int                `json:"frame_count"`
	Rows              int                `json:"rows"`
	Tracks            []tracking.Track   `json:"tracks"`
	Classes           []media.ClassCount `json:"classes"`
	AnomalyFrameCount int                `json:"anomaly_frame_count"`
}

type BoxesResponse struct {
	Scene string         `json:"scene"`
	Frame int            `json:"frame"`
	Boxes []tracking.Box `json:"boxes"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
