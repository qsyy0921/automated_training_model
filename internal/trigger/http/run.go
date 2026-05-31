package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	httpapi "github.com/qsyy0921/automated_training_model/internal/api/httpapi"
	"github.com/qsyy0921/automated_training_model/internal/app/annotationapp"
	"github.com/qsyy0921/automated_training_model/internal/app/datasetapp"
	"github.com/qsyy0921/automated_training_model/internal/app/mediaapp"
	"github.com/qsyy0921/automated_training_model/internal/app/providerapp"
	"github.com/qsyy0921/automated_training_model/internal/app/workspaceapp"
	"github.com/qsyy0921/automated_training_model/internal/infrastructure/config"
	"github.com/qsyy0921/automated_training_model/internal/infrastructure/datasetrepo"
	"github.com/qsyy0921/automated_training_model/internal/infrastructure/datasetruntime"
	"github.com/qsyy0921/automated_training_model/internal/infrastructure/jsonannotation"
	"github.com/qsyy0921/automated_training_model/internal/infrastructure/mergecsv"
	"github.com/qsyy0921/automated_training_model/internal/infrastructure/providerrepo"
	"github.com/qsyy0921/automated_training_model/internal/infrastructure/secrets"
)

func Run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	repo, err := mergecsv.NewRepository(cfg.MergeRoot, cfg.FrameRoot, cfg.MaskRoot)
	if err != nil {
		return err
	}
	annRepo := jsonannotation.NewRepository(cfg.AnnotationRoot)
	mediaSvc := mediaapp.NewMediaService(repo)
	annotationSvc := annotationapp.NewAnnotationService(annRepo)
	datasetSvc := datasetapp.NewDatasetService(datasetrepo.NewJSONRepository(cfg.DataRoot))
	providerSvc := providerapp.NewProviderService(providerrepo.NewMemoryRepository(), secrets.NewEnvStore())
	workspaceSvc := workspaceapp.NewRuntimeService(
		datasetSvc,
		mediaSvc,
		annotationSvc,
		datasetruntime.NewVideoRepositoryFactory(),
		datasetruntime.NewAnnotationRepositoryFactory(),
	)
	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpapi.NewRouter(mediaSvc, annotationSvc, datasetSvc, workspaceSvc, providerSvc, cfg.WebRoot, cfg.DataRoot, logger),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("labelserver listening", "addr", cfg.Addr, "merge_root", cfg.MergeRoot, "frame_root", cfg.FrameRoot, "mask_root", cfg.MaskRoot, "annotation_root", cfg.AnnotationRoot, "web_root", cfg.WebRoot)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
