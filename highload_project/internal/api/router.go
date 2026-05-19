package api

import (
	"github.com/gin-gonic/gin"
	"logs-metrics-platform/internal/api/handlers"
	"logs-metrics-platform/internal/service"
)

func NewRouter(logsSvc *service.LogsService, metricsSvc *service.MetricsService) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())

	r.GET("/health", handlers.Health)

	v1 := r.Group("/api/v1")
	{
		logsH := handlers.NewLogsHandler(logsSvc)
		v1.POST("/logs", logsH.IngestLog)
		v1.POST("/logs/batch", logsH.IngestLogBatch)
		v1.GET("/logs/search", logsH.SearchLogs)
		v1.GET("/logs/stats", logsH.GetLogStats)

		metricsH := handlers.NewMetricsHandler(metricsSvc)
		v1.POST("/metrics", metricsH.IngestMetric)
		v1.POST("/metrics/batch", metricsH.IngestMetricBatch)
		v1.GET("/metrics/query", metricsH.QueryMetrics)
	}

	return r
}
