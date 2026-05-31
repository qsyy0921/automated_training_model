package tracking

import "fmt"

type Box struct {
	Frame      int     `json:"frame"`
	FrameName  string  `json:"frame_name"`
	TrackID    int     `json:"track_id"`
	ClassID    int     `json:"class_id"`
	ClassName  string  `json:"class_name"`
	TrackKey   string  `json:"track_key"`
	Confidence float64 `json:"confidence"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	W          float64 `json:"w"`
	H          float64 `json:"h"`
	X2         float64 `json:"x2"`
	Y2         float64 `json:"y2"`
	Color      string  `json:"color"`
	Source     string  `json:"source"`
}

type Track struct {
	TrackKey     string  `json:"track_key"`
	TrackID      int     `json:"track_id"`
	ClassID      int     `json:"class_id"`
	ClassName    string  `json:"class_name"`
	Color        string  `json:"color"`
	FirstFrame   int     `json:"first_frame"`
	LastFrame    int     `json:"last_frame"`
	Frames       int     `json:"frames"`
	MeanConf     float64 `json:"mean_conf"`
	AvgConf      float64 `json:"avg_conf"`
	MeanArea     float64 `json:"mean_area"`
	AvgArea      float64 `json:"avg_area"`
	MaxArea      float64 `json:"max_area"`
	ReviewStatus string  `json:"review_status,omitempty"`
}

func Key(classID int, trackID int) string {
	return fmt.Sprintf("%d:%d", classID, trackID)
}

func ClassName(classID int) string {
	switch classID {
	case 0:
		return "person"
	case 1:
		return "bicycle"
	case 2:
		return "car"
	case 3:
		return "motorcycle"
	case 5:
		return "bus"
	case 7:
		return "truck"
	case 36:
		return "skateboard"
	case 80:
		return "stroller"
	default:
		return fmt.Sprintf("class_%d", classID)
	}
}

func ClassColor(classID int) string {
	switch classID {
	case 0:
		return "#06b6d4"
	case 1:
		return "#facc15"
	case 2:
		return "#ef4444"
	case 3:
		return "#a855f7"
	case 5:
		return "#2563eb"
	case 7:
		return "#22c55e"
	case 36:
		return "#d946ef"
	case 80:
		return "#f97316"
	default:
		return "#94a3b8"
	}
}
