package config

import (
	"os"
	"strings"
	"time"
)

type Config struct {
	Server     ServerConfig
	Elastic    ElasticConfig
	ClickHouse ClickHouseConfig
	Retention  RetentionConfig
}

type ServerConfig struct {
	Address string
}

type ElasticConfig struct {
	Addresses []string
}

type ClickHouseConfig struct {
	Addrs    []string
	Database string
	Username string
	Password string
}

type RetentionConfig struct {
	LogsRetention    time.Duration
	MetricsRetention time.Duration
	CheckInterval    time.Duration
}

func Load() (*Config, error) {
	return &Config{
		Server: ServerConfig{
			Address: getEnv("SERVER_ADDRESS", ":8080"),
		},
		Elastic: ElasticConfig{
			Addresses: strings.Split(getEnv("ELASTIC_URL", "http://localhost:9200"), ","),
		},
		ClickHouse: ClickHouseConfig{
			Addrs:    strings.Split(getEnv("CLICKHOUSE_ADDRS", "localhost:9000"), ","),
			Database: getEnv("CLICKHOUSE_DB", "telemetry"),
			Username: getEnv("CLICKHOUSE_USER", "default"),
			Password: getEnv("CLICKHOUSE_PASSWORD", ""),
		},
		Retention: RetentionConfig{
			LogsRetention:    getDuration("LOGS_RETENTION_DURATION", 30*24*time.Hour),
			MetricsRetention: getDuration("METRICS_RETENTION_DURATION", 7*24*time.Hour),
			CheckInterval:    getDuration("RETENTION_CHECK_INTERVAL", 1*time.Hour),
		},
	}, nil
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getDuration(key string, defaultVal time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		d, err := time.ParseDuration(val)
		if err == nil {
			return d
		}
	}
	return defaultVal
}
