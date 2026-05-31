package workspaceapp

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/qsyy0921/automated_training_model/internal/app/annotationapp"
	"github.com/qsyy0921/automated_training_model/internal/app/datasetapp"
	"github.com/qsyy0921/automated_training_model/internal/app/mediaapp"
	"github.com/qsyy0921/automated_training_model/internal/domain/dataset"
)

type MediaRepositoryFactory interface {
	NewVideoRepository(ds dataset.Dataset) (mediaapp.VideoRepository, error)
}

type AnnotationRepositoryFactory interface {
	NewAnnotationRepository(ds dataset.Dataset) annotationapp.Repository
}

type RuntimeService struct {
	datasets          *datasetapp.DatasetService
	media             *mediaapp.MediaService
	annotations       *annotationapp.AnnotationService
	mediaFactory      MediaRepositoryFactory
	annotationFactory AnnotationRepositoryFactory

	mu     sync.RWMutex
	active *dataset.Dataset
}

func NewRuntimeService(datasets *datasetapp.DatasetService, media *mediaapp.MediaService, annotations *annotationapp.AnnotationService, mediaFactory MediaRepositoryFactory, annotationFactory AnnotationRepositoryFactory) *RuntimeService {
	return &RuntimeService{
		datasets:          datasets,
		media:             media,
		annotations:       annotations,
		mediaFactory:      mediaFactory,
		annotationFactory: annotationFactory,
	}
}

func (s *RuntimeService) Activate(ctx context.Context, datasetID string) (dataset.Dataset, error) {
	ds, err := s.datasets.Get(ctx, datasetID)
	if err != nil {
		return dataset.Dataset{}, err
	}
	videoRepo, err := s.mediaFactory.NewVideoRepository(*ds)
	if err != nil {
		return dataset.Dataset{}, err
	}
	s.media.SetRepository(videoRepo)
	s.annotations.SetRepository(s.annotationFactory.NewAnnotationRepository(*ds))

	active := *ds
	active.Status = "active"
	if active.AnnotationRoot == "" && active.MergeRoot != "" {
		active.AnnotationRoot = filepath.Join(active.MergeRoot, "annotations_review")
	}
	s.mu.Lock()
	s.active = &active
	s.mu.Unlock()
	return active, nil
}

func (s *RuntimeService) Active(ctx context.Context) (*dataset.Dataset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.active == nil {
		return nil, fmt.Errorf("no active dataset")
	}
	copy := *s.active
	return &copy, nil
}
