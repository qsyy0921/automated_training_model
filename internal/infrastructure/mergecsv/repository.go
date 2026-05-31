package mergecsv

import (
	"context"
	"encoding/binary"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/app/mediaapp"
	"github.com/qsyy0921/automated_training_model/internal/domain/annotation"
	"github.com/qsyy0921/automated_training_model/internal/domain/media"
	"github.com/qsyy0921/automated_training_model/internal/domain/tracking"
)

type Repository struct {
	mergeRoot string
	frameRoot string
	maskRoot  string
	videos    map[string]*media.Video
}

func NewRepository(mergeRoot string, frameRoot string, maskRoot string) (*Repository, error) {
	r := &Repository{
		mergeRoot: mergeRoot,
		frameRoot: frameRoot,
		maskRoot:  maskRoot,
		videos:    map[string]*media.Video{},
	}
	if err := r.reload(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Repository) ListVideos(ctx context.Context) ([]media.VideoSummary, error) {
	rows := make([]media.VideoSummary, 0, len(r.videos))
	for _, v := range r.videos {
		rows = append(rows, media.VideoSummary{
			Scene:      v.Scene,
			FrameCount: v.FrameCount,
			Rows:       v.Rows,
			TrackCount: len(v.Tracks),
			Classes:    classCounts(v.ClassCounts),
			HasPreview: fileExists(filepath.Join(r.mergeRoot, "browser_videos", v.Scene+".mp4")) ||
				fileExists(filepath.Join(r.mergeRoot, "vis_videos", v.Scene+".mp4")),
			AnomalySegments:   v.AnomalySegments,
			AnomalyFrameCount: v.AnomalyFrameCount,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Scene < rows[j].Scene })
	return rows, nil
}

func (r *Repository) GetVideo(ctx context.Context, scene string) (*media.Video, error) {
	v := r.videos[scene]
	if v == nil {
		return nil, fmt.Errorf("video not found: %s", scene)
	}
	return v, nil
}

func (r *Repository) GetBoxes(ctx context.Context, scene string, frame int) ([]tracking.Box, error) {
	v, err := r.GetVideo(ctx, scene)
	if err != nil {
		return nil, err
	}
	boxes := append([]tracking.Box(nil), v.Boxes[frame]...)
	sort.Slice(boxes, func(i, j int) bool {
		if boxes[i].ClassID != boxes[j].ClassID {
			return boxes[i].ClassID < boxes[j].ClassID
		}
		return boxes[i].TrackID < boxes[j].TrackID
	})
	return boxes, nil
}

func (r *Repository) OpenFrame(ctx context.Context, scene string, frame int) (mediaapp.ReadSeekCloser, string, error) {
	v, err := r.GetVideo(ctx, scene)
	if err != nil {
		return nil, "", err
	}
	name := v.FrameNames[frame]
	candidates := []string{}
	if name != "" {
		candidates = append(candidates, filepath.Join(r.frameRoot, scene, name))
	}
	candidates = append(candidates,
		filepath.Join(r.frameRoot, scene, fmt.Sprintf("%03d.jpg", frame-1)),
		filepath.Join(r.frameRoot, scene, fmt.Sprintf("%06d.jpg", frame-1)),
		filepath.Join(r.frameRoot, scene, fmt.Sprintf("%06d.jpg", frame)),
	)
	for _, p := range candidates {
		if fileExists(p) {
			f, err := os.Open(p)
			return f, "image/jpeg", err
		}
	}
	return nil, "", fmt.Errorf("frame not found: scene=%s frame=%d", scene, frame)
}

func (r *Repository) PreviewPath(ctx context.Context, scene string) (string, error) {
	candidates := []string{
		filepath.Join(r.mergeRoot, "browser_videos", scene+".mp4"),
		filepath.Join(r.mergeRoot, "vis_videos", scene+".mp4"),
	}
	for _, p := range candidates {
		if fileExists(p) {
			return p, nil
		}
	}
	return "", fmt.Errorf("preview not found: %s", scene)
}

func (r *Repository) PurgeTracks(ctx context.Context, scene string, trackKeys []string) (int, error) {
	keySet := map[string]bool{}
	for _, key := range trackKeys {
		key = strings.TrimSpace(key)
		if key != "" {
			keySet[key] = true
		}
	}
	if len(keySet) == 0 {
		return 0, nil
	}
	path := filepath.Join(r.mergeRoot, "csv", scene+".csv")
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	backupDir := filepath.Join(r.mergeRoot, "csv_backups", time.Now().Format("20060102_150405"))
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return 0, err
	}
	if err := os.WriteFile(filepath.Join(backupDir, scene+".csv"), raw, 0644); err != nil {
		return 0, err
	}

	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, fmt.Errorf("empty csv: %s", path)
	}
	idx := csvIndex(rows[0])
	kept := [][]string{rows[0]}
	removed := 0
	for _, row := range rows[1:] {
		classID := atoi(csvValue(row, idx, "class_id"))
		trackID := atoi(csvValue(row, idx, "track_id"))
		if keySet[tracking.Key(classID, trackID)] {
			removed++
			continue
		}
		kept = append(kept, row)
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return 0, err
	}
	writer := csv.NewWriter(f)
	if err := writer.WriteAll(kept); err != nil {
		_ = f.Close()
		return 0, err
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		_ = f.Close()
		return 0, err
	}
	if err := f.Close(); err != nil {
		return 0, err
	}
	if err := os.Rename(tmp, path); err != nil {
		return 0, err
	}
	v, err := loadVideo(path, r.maskRoot)
	if err != nil {
		return removed, err
	}
	r.videos[scene] = v
	return removed, nil
}

func (r *Repository) reload() error {
	files, err := filepath.Glob(filepath.Join(r.mergeRoot, "csv", "*.csv"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no csv files found under %s", filepath.Join(r.mergeRoot, "csv"))
	}
	for _, path := range files {
		v, err := loadVideo(path, r.maskRoot)
		if err != nil {
			return err
		}
		r.videos[v.Scene] = v
	}
	return nil
}

func loadVideo(csvPath string, maskRoot string) (*media.Video, error) {
	file, err := os.Open(csvPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}
	idx := csvIndex(header)
	scene := strings.TrimSuffix(filepath.Base(csvPath), filepath.Ext(csvPath))
	v := &media.Video{
		Scene:       scene,
		FrameNames:  map[int]string{},
		ClassCounts: map[int]int{},
		Boxes:       map[int][]tracking.Box{},
	}

	stats := map[string]*struct {
		Track   tracking.Track
		confSum float64
		areaSum float64
	}{}

	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}

		frame := atoi(csvValue(row, idx, "frame_index"))
		trackID := atoi(csvValue(row, idx, "track_id"))
		classID := atoi(csvValue(row, idx, "class_id"))
		className := csvValue(row, idx, "class_name")
		if className == "" {
			className = tracking.ClassName(classID)
		}
		x1 := atof(csvValue(row, idx, "x1"))
		y1 := atof(csvValue(row, idx, "y1"))
		x2 := atof(csvValue(row, idx, "x2"))
		y2 := atof(csvValue(row, idx, "y2"))
		conf := atof(csvValue(row, idx, "confidence"))
		key := tracking.Key(classID, trackID)
		box := tracking.Box{
			Frame:      frame,
			FrameName:  csvValue(row, idx, "frame_name"),
			TrackID:    trackID,
			ClassID:    classID,
			ClassName:  className,
			TrackKey:   key,
			Confidence: conf,
			X:          x1,
			Y:          y1,
			W:          x2 - x1,
			H:          y2 - y1,
			X2:         x2,
			Y2:         y2,
			Color:      tracking.ClassColor(classID),
			Source:     csvValue(row, idx, "source"),
		}
		v.Boxes[frame] = append(v.Boxes[frame], box)
		v.FrameNames[frame] = box.FrameName
		v.ClassCounts[classID]++
		v.Rows++
		if frame > v.FrameCount {
			v.FrameCount = frame
		}
		st := stats[key]
		if st == nil {
			st = &struct {
				Track   tracking.Track
				confSum float64
				areaSum float64
			}{Track: tracking.Track{
				TrackKey:   key,
				TrackID:    trackID,
				ClassID:    classID,
				ClassName:  className,
				Color:      tracking.ClassColor(classID),
				FirstFrame: frame,
				LastFrame:  frame,
			}}
			stats[key] = st
		}
		if frame < st.Track.FirstFrame {
			st.Track.FirstFrame = frame
		}
		if frame > st.Track.LastFrame {
			st.Track.LastFrame = frame
		}
		st.Track.Frames++
		st.confSum += conf
		st.areaSum += box.W * box.H
		if area := box.W * box.H; area > st.Track.MaxArea {
			st.Track.MaxArea = area
		}
	}

	for _, st := range stats {
		if st.Track.Frames > 0 {
			st.Track.MeanConf = st.confSum / float64(st.Track.Frames)
			st.Track.AvgConf = st.Track.MeanConf
			st.Track.MeanArea = st.areaSum / float64(st.Track.Frames)
			st.Track.AvgArea = st.Track.MeanArea
		}
		v.Tracks = append(v.Tracks, st.Track)
	}
	sort.Slice(v.Tracks, func(i, j int) bool {
		if v.Tracks[i].ClassID != v.Tracks[j].ClassID {
			return v.Tracks[i].ClassID < v.Tracks[j].ClassID
		}
		return v.Tracks[i].TrackID < v.Tracks[j].TrackID
	})
	v.AnomalySegments, v.AnomalyFrameCount = loadMaskSegments(maskRoot, v.Scene)
	return v, nil
}

func loadMaskSegments(maskRoot string, scene string) ([]annotation.Segment, int) {
	if maskRoot == "" {
		return []annotation.Segment{}, 0
	}
	path := filepath.Join(maskRoot, scene+".npy")
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) < 16 || string(raw[:6]) != "\x93NUMPY" {
		return []annotation.Segment{}, 0
	}
	pos := 8
	if pos+2 > len(raw) {
		return []annotation.Segment{}, 0
	}
	headerLen := int(binary.LittleEndian.Uint16(raw[pos : pos+2]))
	pos += 2
	if raw[6] >= 2 {
		if pos+4 > len(raw) {
			return []annotation.Segment{}, 0
		}
		headerLen = int(binary.LittleEndian.Uint32(raw[pos : pos+4]))
		pos += 4
	}
	if pos+headerLen > len(raw) {
		return []annotation.Segment{}, 0
	}
	header := string(raw[pos : pos+headerLen])
	data := raw[pos+headerLen:]
	if !(strings.Contains(header, "'descr': '|u1'") || strings.Contains(header, "\"descr\": \"|u1\"") || strings.Contains(header, "'descr': '|b1'")) {
		return []annotation.Segment{}, 0
	}
	segments := []annotation.Segment{}
	start := 0
	count := 0
	for i, v := range data {
		frame := i + 1
		isAnomaly := v != 0
		if isAnomaly {
			count++
			if start == 0 {
				start = frame
			}
		}
		if (!isAnomaly || i == len(data)-1) && start != 0 {
			end := frame - 1
			if isAnomaly && i == len(data)-1 {
				end = frame
			}
			segments = append(segments, annotation.Segment{
				Index:      len(segments) + 1,
				StartFrame: start,
				EndFrame:   end,
				Length:     end - start + 1,
			})
			start = 0
		}
	}
	return segments, count
}

func classCounts(counts map[int]int) []media.ClassCount {
	out := make([]media.ClassCount, 0, len(counts))
	for classID, count := range counts {
		out = append(out, media.ClassCount{
			ClassID:   classID,
			ClassName: tracking.ClassName(classID),
			Color:     tracking.ClassColor(classID),
			Count:     count,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ClassID < out[j].ClassID })
	return out
}

func csvIndex(header []string) map[string]int {
	idx := map[string]int{}
	for i, h := range header {
		idx[h] = i
	}
	return idx
}

func csvValue(row []string, idx map[string]int, name string) string {
	i, ok := idx[name]
	if !ok || i < 0 || i >= len(row) {
		return ""
	}
	return row[i]
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func atof(s string) float64 {
	n, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return n
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}
