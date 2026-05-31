package config

import (
	"flag"
	"path/filepath"
)

type Config struct {
	Addr           string
	MergeRoot      string
	FrameRoot      string
	AnnotationRoot string
}

func FromFlags() Config {
	addr := flag.String("addr", "127.0.0.1:7870", "listen address")
	mergeRoot := flag.String("merge-root", ".", "merge directory with csv and vis_videos")
	frameRoot := flag.String("frame-root", filepath.Clean(filepath.Join("..", "..", "data", "testing", "frames")), "frame directory")
	annotationRoot := flag.String("annotation-root", filepath.Join(".", "annotations_review"), "annotation output directory")
	flag.Parse()
	return Config{
		Addr:           *addr,
		MergeRoot:      filepath.Clean(*mergeRoot),
		FrameRoot:      filepath.Clean(*frameRoot),
		AnnotationRoot: filepath.Clean(*annotationRoot),
	}
}
