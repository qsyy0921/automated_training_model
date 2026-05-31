package datasetrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/qsyy0921/automated_training_model/internal/domain/dataset"
)

type JSONRepository struct {
	path string
	mu   sync.Mutex
}

func NewJSONRepository(root string) *JSONRepository {
	return &JSONRepository{path: filepath.Join(root, "datasets.json")}
}

func (r *JSONRepository) List(ctx context.Context) ([]dataset.Dataset, error) {
	rows, err := r.read()
	if err != nil {
		return nil, err
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].CreatedAt.Before(rows[j].CreatedAt) })
	return rows, nil
}

func (r *JSONRepository) Get(ctx context.Context, id string) (*dataset.Dataset, error) {
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
	return nil, fmt.Errorf("dataset not found: %s", id)
}

func (r *JSONRepository) Save(ctx context.Context, ds dataset.Dataset) (dataset.Dataset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rows, err := r.read()
	if err != nil {
		return dataset.Dataset{}, err
	}
	replaced := false
	for i := range rows {
		if rows[i].ID == ds.ID {
			rows[i] = ds
			replaced = true
			break
		}
	}
	if !replaced {
		rows = append(rows, ds)
	}
	if err := os.MkdirAll(filepath.Dir(r.path), 0755); err != nil {
		return dataset.Dataset{}, err
	}
	raw, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return dataset.Dataset{}, err
	}
	raw = append(raw, '\n')
	return ds, os.WriteFile(r.path, raw, 0644)
}

func (r *JSONRepository) read() ([]dataset.Dataset, error) {
	raw, err := os.ReadFile(r.path)
	if os.IsNotExist(err) {
		return []dataset.Dataset{}, nil
	}
	if err != nil {
		return nil, err
	}
	var rows []dataset.Dataset
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}
