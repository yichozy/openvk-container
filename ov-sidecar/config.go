package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

type Config struct {
	Port              string
	Timeout           time.Duration
	MaxGrepResults    int
	MaxGrepFilesize   string
	GrepThreads       int
	MaxConcurrency    int
	OpenVikingPath    string
	OpenVikingAccount string
	OpenVikingPrefix  string // OpenVikingPath + "/" + OpenVikingAccount + "/"

	SyncEnabled  bool
	SyncSource   string
	SyncDests    []string
	SyncExcludes []string
	SyncInterval time.Duration

	RsyncDaemonEnabled    bool
	RsyncDaemonPort       string
	RsyncDaemonConfigPath string

	RedisAddr        string
	RedisPassword    string
	RedisDB          int
	GrepCachePrefix  string
	GrepCacheTTL     time.Duration
}

const defaultExcludes = ".openviking.pid,temp/,_system/queue/,_system/redo/,log/,vectordb/"

func Load() (*Config, error) {
	cfg := &Config{
		Port:              getEnvAny([]string{"SIDECAR_PORT", "GREP_PORT"}, "1935"),
		Timeout:           parseDurationAny([]string{"SIDECAR_TIMEOUT", "GREP_TIMEOUT"}, "30s"),
		OpenVikingPath:    os.Getenv("OPEN_VIKING_DATA_PATH"),
		OpenVikingAccount: getEnv("OPEN_VIKING_ACCOUNT", "default"),
	}

	if cfg.OpenVikingPath == "" {
		return nil, fmt.Errorf("OPEN_VIKING_DATA_PATH is required")
	}
	cfg.OpenVikingPrefix = filepath.Clean(cfg.OpenVikingPath) + "/" + cfg.OpenVikingAccount + "/"

	maxResultsStr := getEnvAny([]string{"MAX_GREP_RESULTS", "GREP_MAX_RESULTS"}, "500")
	maxResults, err := strconv.Atoi(maxResultsStr)
	if err != nil {
		return nil, fmt.Errorf("invalid MAX_GREP_RESULTS: %w", err)
	}
	cfg.MaxGrepResults = maxResults

	cfg.MaxGrepFilesize = getEnvAny([]string{"MAX_GREP_FILESIZE", "GREP_MAX_FILESIZE"}, "50M")

	grepThreadsStr := getEnvAny([]string{"SIDECAR_GREP_THREADS", "GREP_THREADS", "RG_THREADS"}, "2")
	grepThreads, err := strconv.Atoi(grepThreadsStr)
	if err != nil {
		return nil, fmt.Errorf("invalid SIDECAR_GREP_THREADS: %w", err)
	}
	cfg.GrepThreads = grepThreads

	maxConcStr := getEnvAny([]string{"SIDECAR_MAX_CONCURRENCY", "GREP_MAX_CONCURRENCY"}, "2")
	maxConc, err := strconv.Atoi(maxConcStr)
	if err != nil {
		return nil, fmt.Errorf("invalid SIDECAR_MAX_CONCURRENCY: %w", err)
	}
	if maxConc < 1 {
		maxConc = 1
	}
	cfg.MaxConcurrency = maxConc

	// Sync config
	if syncSource := os.Getenv("SYNC_SOURCE_DIR"); syncSource != "" {
		if syncDests := os.Getenv("SYNC_DEST_DIRS"); syncDests != "" {
			cfg.SyncEnabled = true
			cfg.SyncSource = syncSource
			cfg.SyncDests = splitPaths(syncDests)
			cfg.SyncInterval = parseDuration("SYNC_INTERVAL", "5m")
			cfg.SyncExcludes = splitPaths(getEnv("SYNC_EXCLUDES", defaultExcludes))
		}
	}

	// Rsync daemon config (replica mode)
	if os.Getenv("RSYNC_DAEMON_ENABLED") == "true" {
		cfg.RsyncDaemonEnabled = true
		cfg.RsyncDaemonPort = getEnv("RSYNC_DAEMON_PORT", "873")
		cfg.RsyncDaemonConfigPath = getEnv("RSYNC_DAEMON_CONFIG_PATH", "/etc/rsyncd.conf")
	}

	cfg.RedisAddr = getEnvAny([]string{"CACHE_REDIS_HOST", "REDIS_ADDR"}, "")
	cfg.RedisPassword = getEnvAny([]string{"CACHE_REDIS_PASSWORD", "REDIS_PASSWORD"}, "")

	redisDBStr := getEnvAny([]string{"CACHE_REDIS_DB", "REDIS_DB"}, "0")
	redisDB, err := strconv.Atoi(redisDBStr)
	if err != nil {
		return nil, fmt.Errorf("invalid CACHE_REDIS_DB/REDIS_DB: %w", err)
	}
	cfg.RedisDB = redisDB

	cfg.GrepCachePrefix = getEnvAny([]string{"CACHE_GREP_PREFIX", "GREP_CACHE_PREFIX"}, "grep:")
	cfg.GrepCacheTTL = parseDurationAny([]string{"CACHE_GREP_TTL", "GREP_CACHE_TTL"}, "120s")

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvAny(keys []string, fallback string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return fallback
}

func parseDurationAny(keys []string, fallback string) time.Duration {
	value := getEnvAny(keys, fallback)
	duration, err := time.ParseDuration(value)
	if err != nil {
		duration, _ = time.ParseDuration(fallback)
		zap.L().Warn("invalid duration, using fallback",
			zap.Strings("keys", keys),
			zap.String("value", value),
			zap.String("fallback", fallback),
			zap.Error(err),
		)
	}
	return duration
}

func parseDuration(key, fallback string) time.Duration {
	return parseDurationAny([]string{key}, fallback)
}

func splitPaths(s string) []string {
	var paths []string
	for _, path := range strings.Split(s, ",") {
		path = strings.TrimSpace(path)
		if path != "" {
			paths = append(paths, path)
		}
	}
	return paths
}
