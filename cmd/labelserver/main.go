package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/qsyy0921/automated_training_model/internal/infrastructure/config"
	httptrigger "github.com/qsyy0921/automated_training_model/internal/trigger/http"
)

func main() {
	cfg := config.FromFlags()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := httptrigger.Run(ctx, cfg, logger); err != nil {
		logger.Error("labelserver stopped", "error", err)
		os.Exit(1)
	}
}
