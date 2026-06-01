package jsonannotation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/domain/annotation"
	"github.com/qsyy0921/automated_training_model/internal/domain/taxonomy"
)

type Repository struct {
	root           string
	rejectStatuses map[string]bool
	mu             sync.Mutex
}

func NewRepository(root string) *Repository {
	return NewRepositoryWithRejectStatuses(root, taxonomy.Default().TrackingRejectStatuses)
}

func NewRepositoryWithRejectStatuses(root string, statuses []string) *Repository {
	rejectStatuses := map[string]bool{}
	for _, status := range statuses {
		normalized := annotation.NormalizeStatus(status)
		if normalized != "" {
			rejectStatuses[normalized] = true
		}
	}
	return &Repository{root: root, rejectStatuses: rejectStatuses}
}

func (r *Repository) List(ctx context.Context, scene string) ([]annotation.Annotation, error) {
	return r.read(scene), nil
}

func (r *Repository) Save(ctx context.Context, scene string, ann annotation.Annotation) (annotation.Annotation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().Format("2006-01-02T15:04:05")
	if ann.ID == "" {
		ann.ID = scene + "-" + strings.ReplaceAll(ann.TrackKey, ":", "-") + "-" + newID()
		ann.CreatedAt = now
	}
	ann.Scene = scene
	ann.UpdatedAt = now

	anns := r.read(scene)
	replaced := false
	for i := range anns {
		if anns[i].ID == ann.ID {
			if ann.CreatedAt == "" {
				ann.CreatedAt = anns[i].CreatedAt
			}
			anns[i] = ann
			replaced = true
			break
		}
	}
	if !replaced {
		anns = append(anns, ann)
	}
	if err := r.write(scene, anns); err != nil {
		return annotation.Annotation{}, err
	}
	return ann, nil
}

func (r *Repository) Delete(ctx context.Context, scene string, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	anns := r.read(scene)
	kept := make([]annotation.Annotation, 0, len(anns))
	for _, ann := range anns {
		if ann.ID != id {
			kept = append(kept, ann)
		}
	}
	return r.write(scene, kept)
}

func (r *Repository) RejectedTrackKeys(ctx context.Context, scene string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, ann := range r.read(scene) {
		if ann.TrackKey != "" && r.rejectStatuses[annotation.NormalizeStatus(ann.TrackingStatus)] {
			out[ann.TrackKey] = true
		}
	}
	return out, nil
}

func (r *Repository) read(scene string) []annotation.Annotation {
	raw, err := os.ReadFile(r.path(scene))
	if err != nil {
		return []annotation.Annotation{}
	}
	var anns []annotation.Annotation
	if err := json.Unmarshal(raw, &anns); err != nil {
		return []annotation.Annotation{}
	}
	return anns
}

func (r *Repository) write(scene string, anns []annotation.Annotation) error {
	if err := os.MkdirAll(r.root, 0755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(anns, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(r.path(scene), raw, 0644)
}

func (r *Repository) path(scene string) string {
	return filepath.Join(r.root, scene+".json")
}

func newID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return hex.EncodeToString(buf)
}
