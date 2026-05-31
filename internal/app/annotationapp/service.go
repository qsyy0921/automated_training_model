package annotationapp

import (
	"context"
	"sync"

	"github.com/qsyy0921/automated_training_model/internal/domain/annotation"
)

type Repository interface {
	List(ctx context.Context, scene string) ([]annotation.Annotation, error)
	Save(ctx context.Context, scene string, ann annotation.Annotation) (annotation.Annotation, error)
	Delete(ctx context.Context, scene string, id string) error
	RejectedTrackKeys(ctx context.Context, scene string) (map[string]bool, error)
}

type AnnotationService struct {
	mu   sync.RWMutex
	repo Repository
}

func NewAnnotationService(repo Repository) *AnnotationService {
	return &AnnotationService{repo: repo}
}

func (s *AnnotationService) SetRepository(repo Repository) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.repo = repo
}

func (s *AnnotationService) List(ctx context.Context, scene string) ([]annotation.Annotation, error) {
	repo := s.repository()
	return repo.List(ctx, scene)
}

func (s *AnnotationService) Save(ctx context.Context, scene string, ann annotation.Annotation) (annotation.Annotation, error) {
	repo := s.repository()
	return repo.Save(ctx, scene, ann)
}

func (s *AnnotationService) Delete(ctx context.Context, scene string, id string) error {
	repo := s.repository()
	return repo.Delete(ctx, scene, id)
}

func (s *AnnotationService) RejectedTrackKeys(ctx context.Context, scene string) (map[string]bool, error) {
	repo := s.repository()
	return repo.RejectedTrackKeys(ctx, scene)
}

func (s *AnnotationService) repository() Repository {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.repo
}
