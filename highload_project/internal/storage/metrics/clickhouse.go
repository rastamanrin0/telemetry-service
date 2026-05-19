package metrics

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"logs-metrics-platform/config"
	"logs-metrics-platform/internal/domain"
)

type Repository interface {
	Save(ctx context.Context, metric *domain.Metric) error
	SaveBatch(ctx context.Context, metrics []*domain.Metric) error
	Query(ctx context.Context, q *domain.MetricQuery) (*domain.MetricTimeSeries, error)
	DeleteExpired(ctx context.Context, before time.Time) error
}

type ClickHouseRepository struct {
	conn driver.Conn
}

func NewClickHouseRepository(cfg config.ClickHouseConfig) (*ClickHouseRepository, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: cfg.Addrs,
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		Settings: clickhouse.Settings{
			"async_insert":          1,
			"wait_for_async_insert": 0,
		},
		MaxOpenConns: 20,
		MaxIdleConns: 10,
	})
	if err != nil {
		return nil, fmt.Errorf("opening clickhouse connection: %w", err)
	}

	repo := &ClickHouseRepository{conn: conn}
	if err := repo.ensureSchema(context.Background()); err != nil {
		return nil, fmt.Errorf("ensuring clickhouse schema: %w", err)
	}
	return repo, nil
}

func (r *ClickHouseRepository) ensureSchema(ctx context.Context) error {
	return r.conn.Exec(ctx, `
CREATE TABLE IF NOT EXISTS metrics (
    metric_name  LowCardinality(String),
    timestamp    DateTime64(3, 'UTC'),
    value        Float64,
    tags         Map(String, String)
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (metric_name, timestamp)
SETTINGS index_granularity = 8192`)
}

func (r *ClickHouseRepository) Close() error {
	return r.conn.Close()
}

func (r *ClickHouseRepository) Save(ctx context.Context, metric *domain.Metric) error {
	return r.SaveBatch(ctx, []*domain.Metric{metric})
}

func (r *ClickHouseRepository) SaveBatch(ctx context.Context, metrics []*domain.Metric) error {
	batch, err := r.conn.PrepareBatch(ctx, "INSERT INTO metrics (metric_name, timestamp, value, tags)")
	if err != nil {
		return fmt.Errorf("preparing batch: %w", err)
	}
	for _, m := range metrics {
		if err := batch.Append(m.MetricName, m.Timestamp, m.Value, m.Tags); err != nil {
			return fmt.Errorf("appending metric %s: %w", m.MetricName, err)
		}
	}
	if err := batch.Send(); err != nil {
		return fmt.Errorf("sending batch to clickhouse: %w", err)
	}
	return nil
}

func (r *ClickHouseRepository) Query(ctx context.Context, q *domain.MetricQuery) (*domain.MetricTimeSeries, error) {
	query, args := r.buildQuery(q)

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying clickhouse: %w", err)
	}
	defer rows.Close()

	var points []*domain.MetricPoint
	for rows.Next() {
		var ts time.Time
		var val float64
		if err := rows.Scan(&ts, &val); err != nil {
			return nil, fmt.Errorf("scanning metric row: %w", err)
		}
		points = append(points, &domain.MetricPoint{Timestamp: ts, Value: val})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating metric rows: %w", err)
	}

	return &domain.MetricTimeSeries{
		MetricName: q.MetricName,
		Tags:       q.Tags,
		Points:     points,
	}, nil
}

func (r *ClickHouseRepository) DeleteExpired(ctx context.Context, before time.Time) error {
	err := r.conn.Exec(ctx, "ALTER TABLE metrics DELETE WHERE timestamp < ?", before)
	if err != nil {
		return fmt.Errorf("deleting expired metrics: %w", err)
	}
	return nil
}

func (r *ClickHouseRepository) buildQuery(q *domain.MetricQuery) (string, []interface{}) {
	var sb strings.Builder
	args := []interface{}{q.MetricName, q.From, q.To}

	if q.Window > 0 {
		windowSec := int(q.Window.Seconds())
		fmt.Fprintf(&sb,
			"SELECT toStartOfInterval(timestamp, INTERVAL %d SECOND) AS ts, %s AS value\n",
			windowSec, r.aggExpression(q.AggFunc),
		)
	} else {
		sb.WriteString("SELECT timestamp AS ts, value\n")
	}

	sb.WriteString("FROM metrics\n")
	sb.WriteString("WHERE metric_name = ?\n")
	sb.WriteString("  AND timestamp >= ? AND timestamp <= ?\n")

	for k, v := range q.Tags {
		sb.WriteString("  AND tags[?] = ?\n")
		args = append(args, k, v)
	}

	if q.Window > 0 {
		sb.WriteString("GROUP BY ts\n")
	}

	sb.WriteString("ORDER BY ts ASC")
	return sb.String(), args
}

func (r *ClickHouseRepository) aggExpression(fn domain.AggregationFunc) string {
	switch fn {
	case domain.AggMin:
		return "min(value)"
	case domain.AggMax:
		return "max(value)"
	case domain.AggSum:
		return "sum(value)"
	default:
		return "avg(value)"
	}
}
