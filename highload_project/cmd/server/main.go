package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"logs-metrics-platform/config"
	"logs-metrics-platform/internal/api"
	"logs-metrics-platform/internal/retention"
	storagelogs "logs-metrics-platform/internal/storage/logs"
	storagemetrics "logs-metrics-platform/internal/storage/metrics"
	"logs-metrics-platform/internal/service"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	logsRepo, err := storagelogs.NewElasticsearchRepository(cfg.Elastic)
	if err != nil {
		slog.Error("failed to create logs repository", "err", err)
		os.Exit(1)
	}

	metricsRepo, err := storagemetrics.NewClickHouseRepository(cfg.ClickHouse)
	if err != nil {
		slog.Error("failed to create metrics repository", "err", err)
		os.Exit(1)
	}
	defer metricsRepo.Close()

	logsSvc := service.NewLogsService(logsRepo)
	metricsSvc := service.NewMetricsService(metricsRepo)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	retentionMgr := retention.NewManager(logsRepo, metricsRepo, cfg.Retention)
	go retentionMgr.Start(ctx)

	router := api.NewRouter(logsSvc, metricsSvc)

	go func() {
		slog.Info("HTTP server starting", "address", cfg.Server.Address)
		if err := router.Run(cfg.Server.Address); err != nil {
			slog.Error("HTTP server failed", "err", err)
			cancel()
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down")
	cancel()
}
