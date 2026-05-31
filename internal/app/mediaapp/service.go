package mediaapp

import (
	"context"
	"io"
	"sync"

	"github.com/qsyy0921/automated_training_model/internal/domain/media"
	"github.com/qsyy0921/automated_training_model/internal/domain/tracking"
)

type ReadSeekCloser interface {
	io.Reader
	io.Seeker
	io.Closer
}

type VideoRepository interface {
	ListVideos(ctx context.Context) ([]media.VideoSummary, error)
	GetVideo(ctx context.Context, scene string) (*media.Video, error)
	GetBoxes(ctx context.Context, scene string, frame int) ([]tracking.Box, error)
	OpenFrame(ctx context.Context, scene string, frame int) (ReadSeekCloser, string, error)
	PreviewPath(ctx context.Context, scene string) (string, error)
	PurgeTracks(ctx context.Context, scene string, trackKeys []string) (int, error)
}

type MediaService struct {
	mu   sync.RWMutex
	repo VideoRepository
}

func NewMediaService(repo VideoRepository) *MediaService {
	return &MediaService{repo: repo}
}

func (s *MediaService) SetRepository(repo VideoRepository) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.repo = repo
}

func (s *MediaService) ListVideos(ctx context.Context) ([]media.VideoSummary, error) {
	repo := s.repository()
	return repo.ListVideos(ctx)
}

func (s *MediaService) GetVideo(ctx context.Context, scene string) (*media.Video, error) {
	repo := s.repository()
	return repo.GetVideo(ctx, scene)
}

func (s *MediaService) GetBoxes(ctx context.Context, scene string, frame int) ([]tracking.Box, error) {
	repo := s.repository()
	return repo.GetBoxes(ctx, scene, frame)
}

func (s *MediaService) OpenFrame(ctx context.Context, scene string, frame int) (ReadSeekCloser, string, error) {
	repo := s.repository()
	return repo.OpenFrame(ctx, scene, frame)
}

func (s *MediaService) PreviewPath(ctx context.Context, scene string) (string, error) {
	repo := s.repository()
	return repo.PreviewPath(ctx, scene)
}

func (s *MediaService) PurgeTracks(ctx context.Context, scene string, trackKeys []string) (int, error) {
	repo := s.repository()
	return repo.PurgeTracks(ctx, scene, trackKeys)
}

func (s *MediaService) repository() VideoRepository {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.repo
}
