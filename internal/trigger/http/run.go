package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	httpapi "github.com/qsyy0921/_video_label_tool/labelserver/internal/api/httpapi"
	"github.com/qsyy0921/_video_label_tool/labelserver/internal/app"
	"github.com/qsyy0921/_video_label_tool/labelserver/internal/infrastructure/config"
	"github.com/qsyy0921/_video_label_tool/labelserver/internal/infrastructure/mergecsv"
	"github.com/qsyy0921/_video_label_tool/labelserver/internal/infrastructure/providerrepo"
	"github.com/qsyy0921/_video_label_tool/labelserver/internal/infrastructure/secrets"
)

func Run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	repo, err := mergecsv.NewRepository(cfg.MergeRoot, cfg.FrameRoot)
	if err != nil {
		return err
	}
	mediaSvc := app.NewMediaService(repo)
	providerSvc := app.NewProviderService(providerrepo.NewMemoryRepository(), secrets.NewEnvStore())
	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpapi.NewRouter(mediaSvc, providerSvc, logger),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("labelserver listening", "addr", cfg.Addr, "merge_root", cfg.MergeRoot, "frame_root", cfg.FrameRoot)
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
