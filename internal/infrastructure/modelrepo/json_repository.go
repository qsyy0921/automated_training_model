package modelrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/qsyy0921/automated_training_model/internal/domain/modelregistry"
)

type JSONRepository struct {
	path string
	mu   sync.Mutex
}

func NewJSONRepository(root string) *JSONRepository {
	return &JSONRepository{path: filepath.Join(root, "models.json")}
}

func (r *JSONRepository) List(ctx context.Context) ([]modelregistry.ModelVersion, error) {
	rows, err := r.read()
	if err != nil {
		return nil, err
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].CreatedAt.After(rows[j].CreatedAt)
	})
	return rows, nil
}

func (r *JSONRepository) Get(ctx context.Context, id string) (*modelregistry.ModelVersion, error) {
	rows, err := r.read()
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.ID == id {
			copy := row
			return &copy, nil
		}
	}
	return nil, fmt.Errorf("model not found: %s", id)
}

func (r *JSONRepository) Save(ctx context.Context, model modelregistry.ModelVersion) (modelregistry.ModelVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rows, err := r.read()
	if err != nil {
		return modelregistry.ModelVersion{}, err
	}
	replaced := false
	for i := range rows {
		if rows[i].ID == model.ID {
			if model.CreatedAt.IsZero() {
				model.CreatedAt = rows[i].CreatedAt
			}
			rows[i] = model
			replaced = true
			break
		}
	}
	if !replaced {
		rows = append(rows, model)
	}
	if err := os.MkdirAll(filepath.Dir(r.path), 0755); err != nil {
		return modelregistry.ModelVersion{}, err
	}
	raw, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return modelregistry.ModelVersion{}, err
	}
	raw = append(raw, '\n')
	return model, os.WriteFile(r.path, raw, 0644)
}

func (r *JSONRepository) read() ([]modelregistry.ModelVersion, error) {
	raw, err := os.ReadFile(r.path)
	if os.IsNotExist(err) {
		return []modelregistry.ModelVersion{}, nil
	}
	if err != nil {
		return nil, err
	}
	var rows []modelregistry.ModelVersion
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}
