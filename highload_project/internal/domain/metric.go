package domain

import "time"

type AggregationFunc string

const (
	AggMin AggregationFunc = "min"
	AggMax AggregationFunc = "max"
	AggAvg AggregationFunc = "avg"
	AggSum AggregationFunc = "sum"
)

func (a AggregationFunc) IsValid() bool {
	switch a {
	case AggMin, AggMax, AggAvg, AggSum:
		return true
	}
	return false
}

type Metric struct {
	MetricName string            `json:"metric_name"`
	Timestamp  time.Time         `json:"timestamp"`
	Value      float64           `json:"value"`
	Tags       map[string]string `json:"tags"`
}

type MetricQuery struct {
	MetricName string
	From       time.Time
	To         time.Time
	Tags       map[string]string
	AggFunc    AggregationFunc
	// Window is the aggregation bucket size. Zero means no aggregation.
	Window time.Duration
}

type MetricPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

type MetricTimeSeries struct {
	MetricName string            `json:"metric_name"`
	Tags       map[string]string `json:"tags,omitempty"`
	Points     []*MetricPoint    `json:"points"`
}
