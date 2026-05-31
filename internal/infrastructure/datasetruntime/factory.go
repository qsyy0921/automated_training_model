package datasetruntime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/qsyy0921/automated_training_model/internal/app/annotationapp"
	"github.com/qsyy0921/automated_training_model/internal/app/mediaapp"
	"github.com/qsyy0921/automated_training_model/internal/domain/dataset"
	"github.com/qsyy0921/automated_training_model/internal/infrastructure/jsonannotation"
	"github.com/qsyy0921/automated_training_model/internal/infrastructure/mergecsv"
)

type VideoRepositoryFactory struct{}

func NewVideoRepositoryFactory() *VideoRepositoryFactory {
	return &VideoRepositoryFactory{}
}

func (f *VideoRepositoryFactory) NewVideoRepository(ds dataset.Dataset) (mediaapp.VideoRepository, error) {
	effective, err := resolveDataset(ds)
	if err != nil {
		return nil, err
	}
	if effective.MergeRoot == "" {
		return nil, fmt.Errorf("dataset %s has no activatable merge_root", ds.ID)
	}
	return mergecsv.NewRepository(effective.MergeRoot, effective.FrameRoot, effective.MaskRoot)
}

type AnnotationRepositoryFactory struct{}

func NewAnnotationRepositoryFactory() *AnnotationRepositoryFactory {
	return &AnnotationRepositoryFactory{}
}

func (f *AnnotationRepositoryFactory) NewAnnotationRepository(ds dataset.Dataset) annotationapp.Repository {
	root := ds.AnnotationRoot
	if root == "" && ds.MergeRoot != "" {
		root = filepath.Join(ds.MergeRoot, "annotations_review")
	}
	return jsonannotation.NewRepository(root)
}

type manifestDataset struct {
	MergeRoot      string `json:"merge_root"`
	FrameRoot      string `json:"frame_root"`
	MaskRoot       string `json:"mask_root"`
	AnnotationRoot string `json:"annotation_root"`
}

func resolveDataset(ds dataset.Dataset) (dataset.Dataset, error) {
	if ds.SourceType != dataset.SourceManifest || ds.ManifestPath == "" {
		return ds, nil
	}
	raw, err := os.ReadFile(ds.ManifestPath)
	if err != nil {
		return dataset.Dataset{}, err
	}
	var manifest manifestDataset
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return dataset.Dataset{}, err
	}
	if ds.MergeRoot == "" {
		ds.MergeRoot = manifest.MergeRoot
	}
	if ds.FrameRoot == "" {
		ds.FrameRoot = manifest.FrameRoot
	}
	if ds.MaskRoot == "" {
		ds.MaskRoot = manifest.MaskRoot
	}
	if ds.AnnotationRoot == "" {
		ds.AnnotationRoot = manifest.AnnotationRoot
	}
	return ds, nil
}
