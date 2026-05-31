package app

import (
	"context"
	"io"

	"github.com/qsyy0921/_video_label_tool/labelserver/internal/domain/media"
	"github.com/qsyy0921/_video_label_tool/labelserver/internal/domain/tracking"
	"github.com/qsyy0921/_video_label_tool/labelserver/internal/domain/workflow"
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
}

type TaskQueue interface {
	Enqueue(ctx context.Context, spec workflow.TaskSpec) (string, error)
	Status(ctx context.Context, id string) (*workflow.Task, error)
	Cancel(ctx context.Context, id string) error
}

type ModelGateway interface {
	Submit(ctx context.Context, taskType string, payload map[string]string) (string, error)
	Status(ctx context.Context, id string) (*workflow.Task, error)
	Cancel(ctx context.Context, id string) error
}
