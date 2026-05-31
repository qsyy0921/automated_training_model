package datasetapp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/qsyy0921/automated_training_model/internal/domain/dataset"
)

type Repository interface {
	List(ctx context.Context) ([]dataset.Dataset, error)
	Get(ctx context.Context, id string) (*dataset.Dataset, error)
	Save(ctx context.Context, ds dataset.Dataset) (dataset.Dataset, error)
}

type DatasetService struct {
	repo Repository
}

func NewDatasetService(repo Repository) *DatasetService {
	return &DatasetService{repo: repo}
}

func (s *DatasetService) List(ctx context.Context) ([]dataset.Dataset, error) {
	return s.repo.List(ctx)
}

func (s *DatasetService) Get(ctx context.Context, id string) (*dataset.Dataset, error) {
	return s.repo.Get(ctx, id)
}

func (s *DatasetService) Register(ctx context.Context, req dataset.RegisterRequest) (dataset.Dataset, error) {
	if strings.TrimSpace(req.Name) == "" {
		req.Name = defaultDatasetName(req)
	}
	if err := validateRegisterRequest(req); err != nil {
		return dataset.Dataset{}, err
	}
	now := time.Now()
	ds := dataset.Dataset{
		ID:             newDatasetID(req.Name),
		Name:           firstNonEmpty(req.Name, "dataset"),
		SourceType:     req.SourceType,
		MergeRoot:      cleanOptional(req.MergeRoot),
		FrameRoot:      cleanOptional(req.FrameRoot),
		MaskRoot:       cleanOptional(req.MaskRoot),
		AnnotationRoot: cleanOptional(req.AnnotationRoot),
		ManifestPath:   cleanOptional(req.ManifestPath),
		UploadPath:     cleanOptional(req.UploadPath),
		Status:         "registered",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if ds.SourceType == "" {
		ds.SourceType = dataset.SourceLocalFolder
	}
	return s.repo.Save(ctx, ds)
}

func validateRegisterRequest(req dataset.RegisterRequest) error {
	switch req.SourceType {
	case "", dataset.SourceLocalFolder:
		if strings.TrimSpace(req.MergeRoot) == "" {
			return fmt.Errorf("merge_root is required for local folder datasets")
		}
		if err := requireDir(req.MergeRoot); err != nil {
			return fmt.Errorf("invalid merge_root: %w", err)
		}
		if req.FrameRoot != "" {
			if err := requireDir(req.FrameRoot); err != nil {
				return fmt.Errorf("invalid frame_root: %w", err)
			}
		}
	case dataset.SourceUpload:
		if strings.TrimSpace(req.UploadPath) == "" {
			return fmt.Errorf("upload_path is required for uploaded datasets")
		}
		if err := requireFile(req.UploadPath); err != nil {
			return fmt.Errorf("invalid upload_path: %w", err)
		}
	case dataset.SourceManifest:
		if strings.TrimSpace(req.ManifestPath) == "" {
			return fmt.Errorf("manifest_path is required for manifest datasets")
		}
		if err := requireFile(req.ManifestPath); err != nil {
			return fmt.Errorf("invalid manifest_path: %w", err)
		}
	default:
		return fmt.Errorf("unsupported dataset source type: %s", req.SourceType)
	}
	return nil
}

func requireDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}
	return nil
}

func requireFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory", path)
	}
	return nil
}

func defaultDatasetName(req dataset.RegisterRequest) string {
	for _, value := range []string{req.MergeRoot, req.ManifestPath, req.UploadPath} {
		value = strings.TrimSpace(value)
		if value != "" {
			return strings.TrimSuffix(filepath.Base(value), filepath.Ext(value))
		}
	}
	return "dataset"
}

func cleanOptional(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	return filepath.Clean(path)
}

func newDatasetID(name string) string {
	slug := strings.ToLower(strings.TrimSpace(name))
	slug = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return '-'
	}, slug)
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "dataset"
	}
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return slug + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return slug + "-" + hex.EncodeToString(buf)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
