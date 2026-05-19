package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"logs-metrics-platform/internal/domain"
	storagelogs "logs-metrics-platform/internal/storage/logs"
)

type LogsService struct {
	repo storagelogs.Repository
}

func NewLogsService(repo storagelogs.Repository) *LogsService {
	return &LogsService{repo: repo}
}

func (s *LogsService) IngestLog(ctx context.Context, log *domain.Log) error {
	if err := validateLog(log); err != nil {
		return err
	}
	if log.ID == "" {
		log.ID = uuid.New().String()
	}
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now().UTC()
	}
	return s.repo.Save(ctx, log)
}

func (s *LogsService) IngestLogBatch(ctx context.Context, logs []*domain.Log) error {
	if len(logs) == 0 {
		return errors.New("batch must not be empty")
	}
	for i, l := range logs {
		if err := validateLog(l); err != nil {
			return fmt.Errorf("log[%d]: %w", i, err)
		}
		if l.ID == "" {
			l.ID = uuid.New().String()
		}
		if l.Timestamp.IsZero() {
			l.Timestamp = time.Now().UTC()
		}
	}
	return s.repo.SaveBatch(ctx, logs)
}

func (s *LogsService) SearchLogs(ctx context.Context, q *domain.LogSearchQuery) (*domain.LogSearchResult, error) {
	if q.From.IsZero() {
		q.From = time.Now().UTC().Add(-24 * time.Hour)
	}
	if q.To.IsZero() {
		q.To = time.Now().UTC()
	}
	if q.Size <= 0 {
		q.Size = 100
	}
	return s.repo.Search(ctx, q)
}

func (s *LogsService) GetLogStats(ctx context.Context, q *domain.LogStatsQuery) (*domain.LogStats, error) {
	if q.From.IsZero() {
		q.From = time.Now().UTC().Add(-24 * time.Hour)
	}
	if q.To.IsZero() {
		q.To = time.Now().UTC()
	}
	return s.repo.GetStats(ctx, q)
}

func validateLog(l *domain.Log) error {
	if l.ServiceName == "" {
		return errors.New("service_name is required")
	}
	if l.HostID == "" {
		return errors.New("host_id is required")
	}
	if l.Message == "" {
		return errors.New("message is required")
	}
	if !l.Level.IsValid() {
		return fmt.Errorf("invalid level %q; valid values: DEBUG, INFO, WARNING, ERROR, FATAL", l.Level)
	}
	if !l.RetentionPolicy.IsValid() {
		return fmt.Errorf("invalid retention_policy %q; valid values: archive, short", l.RetentionPolicy)
	}
	return nil
}
