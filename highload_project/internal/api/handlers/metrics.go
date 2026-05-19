package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"logs-metrics-platform/internal/domain"
	"logs-metrics-platform/internal/service"
)

type MetricsHandler struct {
	svc *service.MetricsService
}

func NewMetricsHandler(svc *service.MetricsService) *MetricsHandler {
	return &MetricsHandler{svc: svc}
}

// IngestMetric handles POST /api/v1/metrics
func (h *MetricsHandler) IngestMetric(c *gin.Context) {
	var metric domain.Metric
	if err := c.ShouldBindJSON(&metric); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.IngestMetric(c.Request.Context(), &metric); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"accepted": true})
}

// IngestMetricBatch handles POST /api/v1/metrics/batch
func (h *MetricsHandler) IngestMetricBatch(c *gin.Context) {
	var req struct {
		Metrics []*domain.Metric `json:"metrics" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.IngestMetricBatch(c.Request.Context(), req.Metrics); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"accepted": len(req.Metrics)})
}

// QueryMetrics handles GET /api/v1/metrics/query
// Query params: metric (required), from, to, agg (min|max|avg|sum), window (e.g. 5m),
//
//	and any additional key=value pairs for tag filtering (e.g. service=myservice).
func (h *MetricsHandler) QueryMetrics(c *gin.Context) {
	q, err := parseMetricQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.svc.QueryMetrics(c.Request.Context(), q)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func parseMetricQuery(c *gin.Context) (*domain.MetricQuery, error) {
	metricName := c.Query("metric")
	if metricName == "" {
		return nil, fmt.Errorf("'metric' query parameter is required")
	}

	q := &domain.MetricQuery{
		MetricName: metricName,
		Tags:       make(map[string]string),
	}

	var err error
	if from := c.Query("from"); from != "" {
		q.From, err = time.Parse(time.RFC3339, from)
		if err != nil {
			return nil, fmt.Errorf("invalid 'from' timestamp")
		}
	}
	if to := c.Query("to"); to != "" {
		q.To, err = time.Parse(time.RFC3339, to)
		if err != nil {
			return nil, fmt.Errorf("invalid 'to' timestamp")
		}
	}
	if agg := c.Query("agg"); agg != "" {
		q.AggFunc = domain.AggregationFunc(agg)
		if !q.AggFunc.IsValid() {
			return nil, fmt.Errorf("invalid 'agg' value %q; valid values: min, max, avg, sum", agg)
		}
	}
	if window := c.Query("window"); window != "" {
		q.Window, err = time.ParseDuration(window)
		if err != nil {
			return nil, fmt.Errorf("invalid 'window' duration: %v", err)
		}
	}

	// Collect known tag filter params: service, host, region and any others not reserved.
	reserved := map[string]bool{"metric": true, "from": true, "to": true, "agg": true, "window": true}
	for key, vals := range c.Request.URL.Query() {
		if !reserved[key] && len(vals) > 0 {
			q.Tags[key] = vals[0]
		}
	}

	return q, nil
}
