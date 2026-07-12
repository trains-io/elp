package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/trains-io/elp/apps/z21-gateway/internal/config"
	"github.com/trains-io/elp/apps/z21-gateway/internal/gateway"
	"github.com/trains-io/elp/apps/z21-gateway/internal/health"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	// controller-runtime client logs API warnings during status patches; without this
	// you get a noisy "log.SetLogger(...) was never called" stack trace.
	ctrl.SetLogger(zap.New(zap.UseDevMode(false)))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	healthSrv := health.New(cfg.HealthAddr)
	go func() {
		if err := healthSrv.Start(ctx); err != nil {
			slog.Error("health server stopped", "error", err)
			stop()
		}
	}()

	if err := gateway.Run(ctx, cfg, healthSrv); err != nil {
		slog.Error("gateway stopped", "error", err)
		os.Exit(1)
	}
}
