package config

import (
	"flag"
	"path/filepath"
)

type Config struct {
	Addr           string
	MergeRoot      string
	FrameRoot      string
	MaskRoot       string
	AnnotationRoot string
	WebRoot        string
	DataRoot       string
	ModelRoot      string
	AgentRoot      string
	TaxonomyPath   string
}

func FromFlags() Config {
	addr := flag.String("addr", "127.0.0.1:7870", "listen address")
	mergeRoot := flag.String("merge-root", ".", "merge directory with csv and vis_videos")
	frameRoot := flag.String("frame-root", filepath.Clean(filepath.Join("..", "..", "data", "testing", "frames")), "frame directory")
	maskRoot := flag.String("mask-root", filepath.Clean(filepath.Join("..", "..", "data", "testframemask")), "frame-level anomaly mask directory")
	annotationRoot := flag.String("annotation-root", filepath.Join(".", "annotations_review"), "annotation output directory")
	webRoot := flag.String("web-root", filepath.Join(".", "web"), "static web directory")
	dataRoot := flag.String("data-root", filepath.Join(".", "data_lake"), "dataset registry and upload storage directory")
	modelRoot := flag.String("model-root", filepath.Join(".", "data_lake", "models"), "model registry and artifact metadata directory")
	agentRoot := flag.String("agent-root", filepath.Join(".", "data_lake", "agents"), "agent registry and workflow metadata directory")
	taxonomyPath := flag.String("taxonomy-path", filepath.Join(".", "configs", "taxonomy.review.json"), "review taxonomy JSON path")
	flag.Parse()
	return Config{
		Addr:           *addr,
		MergeRoot:      filepath.Clean(*mergeRoot),
		FrameRoot:      filepath.Clean(*frameRoot),
		MaskRoot:       filepath.Clean(*maskRoot),
		AnnotationRoot: filepath.Clean(*annotationRoot),
		WebRoot:        filepath.Clean(*webRoot),
		DataRoot:       filepath.Clean(*dataRoot),
		ModelRoot:      filepath.Clean(*modelRoot),
		AgentRoot:      filepath.Clean(*agentRoot),
		TaxonomyPath:   filepath.Clean(*taxonomyPath),
	}
}
