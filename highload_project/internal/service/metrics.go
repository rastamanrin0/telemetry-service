package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"logs-metrics-platform/internal/domain"
	storagemetrics "logs-metrics-platform/internal/storage/metrics"
)

type MetricsService struct {
	repo storagemetrics.Repository
}

func NewMetricsService(repo storagemetrics.Repository) *MetricsService {
	return &MetricsService{repo: repo}
}

func (s *MetricsService) IngestMetric(ctx context.Context, metric *domain.Metric) error {
	if err := validateMetric(metric); err != nil {
		return err
	}
	if metric.Timestamp.IsZero() {
		metric.Timestamp = time.Now().UTC()
	}
	return s.repo.Save(ctx, metric)
}

func (s *MetricsService) IngestMetricBatch(ctx context.Context, metrics []*domain.Metric) error {
	if len(metrics) == 0 {
		return errors.New("batch must not be empty")
	}
	for i, m := range metrics {
		if err := validateMetric(m); err != nil {
			return fmt.Errorf("metric[%d]: %w", i, err)
		}
		if m.Timestamp.IsZero() {
			m.Timestamp = time.Now().UTC()
		}
	}
	return s.repo.SaveBatch(ctx, metrics)
}

func (s *MetricsService) QueryMetrics(ctx context.Context, q *domain.MetricQuery) (*domain.MetricTimeSeries, error) {
	if q.MetricName == "" {
		return nil, errors.New("metric_name is required")
	}
	if q.From.IsZero() {
		q.From = time.Now().UTC().Add(-time.Hour)
	}
	if q.To.IsZero() {
		q.To = time.Now().UTC()
	}
	if q.AggFunc != "" && !q.AggFunc.IsValid() {
		return nil, fmt.Errorf("invalid aggregation function %q; valid values: min, max, avg, sum", q.AggFunc)
	}
	return s.repo.Query(ctx, q)
}

func validateMetric(m *domain.Metric) error {
	if m.MetricName == "" {
		return errors.New("metric_name is required")
	}
	return nil
}
