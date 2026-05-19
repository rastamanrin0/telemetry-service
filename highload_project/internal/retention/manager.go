package retention

import (
	"context"
	"log/slog"
	"time"

	"logs-metrics-platform/config"
	"logs-metrics-platform/internal/domain"
	storagelogs "logs-metrics-platform/internal/storage/logs"
	storagemetrics "logs-metrics-platform/internal/storage/metrics"
)

type Manager struct {
	logsRepo         storagelogs.Repository
	metricsRepo      storagemetrics.Repository
	logsRetention    time.Duration
	metricsRetention time.Duration
	checkInterval    time.Duration
}

func NewManager(
	logsRepo storagelogs.Repository,
	metricsRepo storagemetrics.Repository,
	cfg config.RetentionConfig,
) *Manager {
	return &Manager{
		logsRepo:         logsRepo,
		metricsRepo:      metricsRepo,
		logsRetention:    cfg.LogsRetention,
		metricsRetention: cfg.MetricsRetention,
		checkInterval:    cfg.CheckInterval,
	}
}

func (m *Manager) Start(ctx context.Context) {
	slog.Info("retention manager started",
		"logs_retention", m.logsRetention,
		"metrics_retention", m.metricsRetention,
		"check_interval", m.checkInterval,
	)

	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	// run once on startup so we don't wait a full interval after a restart
	m.run(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.run(ctx)
		}
	}
}

func (m *Manager) run(ctx context.Context) {
	logsCutoff := time.Now().UTC().Add(-m.logsRetention)

	if err := m.logsRepo.ArchiveExpired(ctx, logsCutoff); err != nil {
		slog.Error("retention: archive logs failed", "err", err)
	}

	if err := m.logsRepo.DeleteExpired(ctx, domain.RetentionPolicyShort, logsCutoff); err != nil {
		slog.Error("retention: delete short logs failed", "err", err)
	}

	metricsCutoff := time.Now().UTC().Add(-m.metricsRetention)
	if err := m.metricsRepo.DeleteExpired(ctx, metricsCutoff); err != nil {
		slog.Error("retention: delete metrics failed", "err", err)
	}
}
