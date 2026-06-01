package annotation

import "strings"

type Segment struct {
	Index      int `json:"index"`
	StartFrame int `json:"start_frame"`
	EndFrame   int `json:"end_frame"`
	Length     int `json:"length"`
}

type Annotation struct {
	ID             string `json:"id"`
	Scene          string `json:"scene"`
	TrackKey       string `json:"track_key"`
	TrackID        int    `json:"track_id"`
	ClassID        int    `json:"class_id"`
	ObjectClass    string `json:"object_class"`
	StartFrame     int    `json:"start_frame"`
	EndFrame       int    `json:"end_frame"`
	Label          string `json:"label"`
	AnomalyType    string `json:"anomaly_type"`
	TrackingStatus string `json:"tracking_status"`
	TrackingIssue  string `json:"tracking_issue"`
	BBoxQuality    string `json:"bbox_quality"`
	EventID        string `json:"event_id"`
	EventTitle     string `json:"event_title"`
	EventReason    string `json:"event_reason"`
	Severity       string `json:"severity"`
	UpperColor     string `json:"upper_color,omitempty"`
	LowerColor     string `json:"lower_color,omitempty"`
	UpperClothing  string `json:"upper_clothing,omitempty"`
	LowerClothing  string `json:"lower_clothing,omitempty"`
	Carrying       string `json:"carrying,omitempty"`
	Appearance     string `json:"appearance,omitempty"`
	RelatedIDs     string `json:"related_track_ids"`
	Notes          string `json:"notes"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

func NormalizeStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}
