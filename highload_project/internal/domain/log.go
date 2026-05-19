package domain

import "time"

type LogLevel string

const (
	LogLevelDebug   LogLevel = "DEBUG"
	LogLevelInfo    LogLevel = "INFO"
	LogLevelWarning LogLevel = "WARNING"
	LogLevelError   LogLevel = "ERROR"
	LogLevelFatal   LogLevel = "FATAL"
)

func (l LogLevel) IsValid() bool {
	switch l {
	case LogLevelDebug, LogLevelInfo, LogLevelWarning, LogLevelError, LogLevelFatal:
		return true
	}
	return false
}

type RetentionPolicy string

const (
	RetentionPolicyArchive RetentionPolicy = "archive"
	RetentionPolicyShort   RetentionPolicy = "short"
)

func (r RetentionPolicy) IsValid() bool {
	return r == RetentionPolicyArchive || r == RetentionPolicyShort
}

type Log struct {
	ID              string          `json:"id"`
	Timestamp       time.Time       `json:"timestamp"`
	ServiceName     string          `json:"service_name"`
	HostID          string          `json:"host_id"`
	InstanceID      string          `json:"instance_id,omitempty"`
	Level           LogLevel        `json:"level"`
	Message         string          `json:"message"`
	RetentionPolicy RetentionPolicy `json:"retention_policy"`
}

type LogSearchQuery struct {
	From            time.Time
	To              time.Time
	ServiceName     string
	HostID          string
	Level           LogLevel
	RetentionPolicy RetentionPolicy
	Query           string
	Page            int
	Size            int
}

type LogSearchResult struct {
	Total int64  `json:"total"`
	Logs  []*Log `json:"logs"`
}

type LogStatsQuery struct {
	From        time.Time
	To          time.Time
	ServiceName string
}

type LogStats struct {
	Counts map[string]int64 `json:"counts"`
}
