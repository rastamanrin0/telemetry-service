package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"logs-metrics-platform/internal/domain"
	"logs-metrics-platform/internal/service"
)

type LogsHandler struct {
	svc *service.LogsService
}

func NewLogsHandler(svc *service.LogsService) *LogsHandler {
	return &LogsHandler{svc: svc}
}

func (h *LogsHandler) IngestLog(c *gin.Context) {
	var log domain.Log
	if err := c.ShouldBindJSON(&log); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.IngestLog(c.Request.Context(), &log); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"id": log.ID})
}

func (h *LogsHandler) IngestLogBatch(c *gin.Context) {
	var req struct {
		Logs []*domain.Log `json:"logs" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.IngestLogBatch(c.Request.Context(), req.Logs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"accepted": len(req.Logs)})
}

func (h *LogsHandler) SearchLogs(c *gin.Context) {
	q, err := parseLogSearchQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.svc.SearchLogs(c.Request.Context(), q)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *LogsHandler) GetLogStats(c *gin.Context) {
	q := &domain.LogStatsQuery{
		ServiceName: c.Query("service"),
	}
	var err error
	if from := c.Query("from"); from != "" {
		q.From, err = time.Parse(time.RFC3339, from)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid 'from' timestamp"})
			return
		}
	}
	if to := c.Query("to"); to != "" {
		q.To, err = time.Parse(time.RFC3339, to)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid 'to' timestamp"})
			return
		}
	}
	stats, err := h.svc.GetLogStats(c.Request.Context(), q)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

func parseLogSearchQuery(c *gin.Context) (*domain.LogSearchQuery, error) {
	q := &domain.LogSearchQuery{
		ServiceName: c.Query("service"),
		HostID:      c.Query("host"),
		Query:       c.Query("q"),
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
	if lvl := c.Query("level"); lvl != "" {
		q.Level = domain.LogLevel(lvl)
		if !q.Level.IsValid() {
			return nil, fmt.Errorf("invalid level %q", lvl)
		}
	}
	if rp := c.Query("retention_policy"); rp != "" {
		q.RetentionPolicy = domain.RetentionPolicy(rp)
		if !q.RetentionPolicy.IsValid() {
			return nil, fmt.Errorf("invalid retention_policy %q", rp)
		}
	}
	if page := c.Query("page"); page != "" {
		q.Page, err = strconv.Atoi(page)
		if err != nil || q.Page < 0 {
			return nil, fmt.Errorf("invalid 'page'")
		}
	}
	if size := c.Query("size"); size != "" {
		q.Size, err = strconv.Atoi(size)
		if err != nil || q.Size <= 0 {
			return nil, fmt.Errorf("invalid 'size'")
		}
	}
	return q, nil
}
