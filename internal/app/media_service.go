package app

import (
	"context"

	"github.com/qsyy0921/_video_label_tool/labelserver/internal/domain/media"
	"github.com/qsyy0921/_video_label_tool/labelserver/internal/domain/tracking"
)

type MediaService struct {
	repo VideoRepository
}

func NewMediaService(repo VideoRepository) *MediaService {
	return &MediaService{repo: repo}
}

func (s *MediaService) ListVideos(ctx context.Context) ([]media.VideoSummary, error) {
	return s.repo.ListVideos(ctx)
}

func (s *MediaService) GetVideo(ctx context.Context, scene string) (*media.Video, error) {
	return s.repo.GetVideo(ctx, scene)
}

func (s *MediaService) GetBoxes(ctx context.Context, scene string, frame int) ([]tracking.Box, error) {
	return s.repo.GetBoxes(ctx, scene, frame)
}

func (s *MediaService) OpenFrame(ctx context.Context, scene string, frame int) (ReadSeekCloser, string, error) {
	return s.repo.OpenFrame(ctx, scene, frame)
}

func (s *MediaService) PreviewPath(ctx context.Context, scene string) (string, error) {
	return s.repo.PreviewPath(ctx, scene)
}
