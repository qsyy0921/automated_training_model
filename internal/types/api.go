package types

import (
	"github.com/qsyy0921/automated_training_model/internal/domain/annotation"
	"github.com/qsyy0921/automated_training_model/internal/domain/media"
	"github.com/qsyy0921/automated_training_model/internal/domain/tracking"
)

type ListVideosResponse struct {
	Videos []media.VideoSummary `json:"videos"`
}

type VideoMetaResponse struct {
	Scene             string                  `json:"scene"`
	FrameCount        int                     `json:"frame_count"`
	Rows              int                     `json:"rows"`
	Tracks            []tracking.Track        `json:"tracks"`
	Classes           []media.ClassCount      `json:"classes"`
	AnomalyFrameCount int                     `json:"anomaly_frame_count"`
	AnomalySegments   []annotation.Segment    `json:"anomaly_segments"`
	Annotations       []annotation.Annotation `json:"annotations"`
}

type BoxesResponse struct {
	Scene string         `json:"scene"`
	Frame int            `json:"frame"`
	Boxes []tracking.Box `json:"boxes"`
}

type ErrorResponse struct {
	Error         string        `json:"error"`
	ErrorEnvelope ErrorEnvelope `json:"error_envelope,omitempty"`
}

type ErrorEnvelope struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Source    string `json:"source"`
	Retryable bool   `json:"retryable"`
}
