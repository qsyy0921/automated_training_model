package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"time"

	httpapi "github.com/qsyy0921/automated_training_model/internal/api/httpapi"
	"github.com/qsyy0921/automated_training_model/internal/app/agentapp"
	"github.com/qsyy0921/automated_training_model/internal/app/agentruntime"
	"github.com/qsyy0921/automated_training_model/internal/app/annotationapp"
	"github.com/qsyy0921/automated_training_model/internal/app/datasetapp"
	"github.com/qsyy0921/automated_training_model/internal/app/lifecycleapp"
	"github.com/qsyy0921/automated_training_model/internal/app/mediaapp"
	"github.com/qsyy0921/automated_training_model/internal/app/providerapp"
	"github.com/qsyy0921/automated_training_model/internal/app/workspaceapp"
	"github.com/qsyy0921/automated_training_model/internal/infrastructure/agentrepo"
	"github.com/qsyy0921/automated_training_model/internal/infrastructure/config"
	"github.com/qsyy0921/automated_training_model/internal/infrastructure/datasetrepo"
	"github.com/qsyy0921/automated_training_model/internal/infrastructure/datasetruntime"
	"github.com/qsyy0921/automated_training_model/internal/infrastructure/intakerepo"
	"github.com/qsyy0921/automated_training_model/internal/infrastructure/jsonannotation"
	"github.com/qsyy0921/automated_training_model/internal/infrastructure/mergecsv"
	"github.com/qsyy0921/automated_training_model/internal/infrastructure/modelgateway"
	"github.com/qsyy0921/automated_training_model/internal/infrastructure/modelrepo"
	"github.com/qsyy0921/automated_training_model/internal/infrastructure/providerrepo"
	"github.com/qsyy0921/automated_training_model/internal/infrastructure/queue"
	"github.com/qsyy0921/automated_training_model/internal/infrastructure/runtimerepo"
	"github.com/qsyy0921/automated_training_model/internal/infrastructure/secrets"
	"github.com/qsyy0921/automated_training_model/internal/infrastructure/taxonomyrepo"
)

func Run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	repo, err := mergecsv.NewRepository(cfg.MergeRoot, cfg.FrameRoot, cfg.MaskRoot)
	if err != nil {
		return err
	}
	taxonomyCfg, err := taxonomyrepo.Load(cfg.TaxonomyPath)
	if err != nil {
		return err
	}
	annRepo := jsonannotation.NewRepositoryWithRejectStatuses(cfg.AnnotationRoot, taxonomyCfg.TrackingRejectStatuses)
	mediaSvc := mediaapp.NewMediaService(repo)
	annotationSvc := annotationapp.NewAnnotationService(annRepo)
	datasetSvc := datasetapp.NewDatasetService(datasetrepo.NewJSONRepository(cfg.DataRoot))
	taskQueue := queue.NewMemoryQueue()
	modelGateway := modelgateway.NewNoopGateway(taskQueue)
	lifecycleSvc := lifecycleapp.NewServiceWithModelRepository(modelGateway, modelrepo.NewJSONRepository(cfg.ModelRoot))
	agentSvc := agentapp.NewService(agentrepo.NewJSONRepository(cfg.AgentRoot), modelGateway)
	if err := agentSvc.BootstrapDefaults(ctx); err != nil {
		return err
	}
	runtimeStore, err := runtimerepo.NewJSONRuntimeStore(cfg.RuntimeRoot, time.Now())
	if err != nil {
		return err
	}
	modelJobStore, err := runtimerepo.NewJSONModelJobStore(filepath.Join(cfg.RuntimeRoot, "model_jobs.json"), time.Now)
	if err != nil {
		return err
	}
	intakeRepo, err := intakerepo.NewJSONRepository(filepath.Join(cfg.RuntimeRoot, "intake"))
	if err != nil {
		return err
	}
	agentRuntimeSvc := agentruntime.NewServiceWithRuntimeStores(agentSvc, runtimeStore, modelJobStore, intakeRepo)
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
		Handler:           httpapi.NewRouter(mediaSvc, annotationSvc, datasetSvc, workspaceSvc, lifecycleSvc, agentSvc, agentRuntimeSvc, providerSvc, taxonomyCfg, cfg.WebRoot, cfg.DataRoot, logger),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("labelserver listening", "addr", cfg.Addr, "merge_root", cfg.MergeRoot, "frame_root", cfg.FrameRoot, "mask_root", cfg.MaskRoot, "annotation_root", cfg.AnnotationRoot, "model_root", cfg.ModelRoot, "agent_root", cfg.AgentRoot, "runtime_root", cfg.RuntimeRoot, "web_root", cfg.WebRoot)
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
