package config

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
	OpenVikingPath    string
	OpenVikingAccount string
	OpenVikingPrefix  string // OpenVikingPath + "/" + OpenVikingAccount + "/"

	MaxReadFilesize  int64
	MaxReadBatchSize int
	ReadCacheTTL     time.Duration

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

	Bm25IndexPath       string
	Bm25UpdateInterval  time.Duration
	Bm25MaxIndexFilesize int64
	Bm25Excludes        []string
	Bm25BatchSize       int
	Bm25MaxResults      int
}

const defaultSyncExcludes = ".openviking.pid,temp/,_system/,log/,vectordb/,usage_audit.sqlite3*"

const defaultBm25Excludes = ".openviking.pid,temp/,_system/,log/,vectordb/,usage_audit.sqlite3*,.relations.json,_index/"

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

	maxReadFilesizeStr := getEnv("MAX_READ_FILESIZE", "100M")
	maxReadFilesize, err := parseBytes(maxReadFilesizeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid MAX_READ_FILESIZE: %w", err)
	}
	cfg.MaxReadFilesize = maxReadFilesize

	maxReadBatchStr := getEnv("MAX_READ_BATCH_SIZE", "100")
	maxReadBatch, err := strconv.Atoi(maxReadBatchStr)
	if err != nil {
		return nil, fmt.Errorf("invalid MAX_READ_BATCH_SIZE: %w", err)
	}
	cfg.MaxReadBatchSize = maxReadBatch

	cfg.ReadCacheTTL = parseDuration("READ_CACHE_TTL", "5m")

	// Sync config
	if syncSource := os.Getenv("SYNC_SOURCE_DIR"); syncSource != "" {
		if syncDests := os.Getenv("SYNC_DEST_DIRS"); syncDests != "" {
			cfg.SyncEnabled = true
			cfg.SyncSource = syncSource
			cfg.SyncDests = splitPaths(syncDests)
			cfg.SyncInterval = parseDuration("SYNC_INTERVAL", "5m")
			cfg.SyncExcludes = splitPaths(getEnv("SYNC_EXCLUDES", defaultSyncExcludes))
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

	// BM25 config
	bm25IndexPath := getEnv("BM25_INDEX_PATH", "")
	if bm25IndexPath == "" {
		bm25IndexPath = cfg.OpenVikingPrefix + "_index"
	}
	cfg.Bm25IndexPath = bm25IndexPath
	cfg.Bm25UpdateInterval = parseDuration("BM25_UPDATE_INTERVAL", "5m")

	maxIdxFilesizeStr := getEnv("BM25_MAX_INDEX_FILESIZE", "10M")
	maxIdxFilesize, err := parseBytes(maxIdxFilesizeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid BM25_MAX_INDEX_FILESIZE: %w", err)
	}
	cfg.Bm25MaxIndexFilesize = maxIdxFilesize

	cfg.Bm25Excludes = splitPaths(getEnv("BM25_EXCLUDES", defaultBm25Excludes))

	bm25BatchStr := getEnv("BM25_BATCH_SIZE", "1000")
	bm25Batch, err := strconv.Atoi(bm25BatchStr)
	if err != nil {
		return nil, fmt.Errorf("invalid BM25_BATCH_SIZE: %w", err)
	}
	if bm25Batch < 1 {
		bm25Batch = 1000
	}
	cfg.Bm25BatchSize = bm25Batch

	maxBm25ResultsStr := getEnv("BM25_MAX_RESULTS", "500")
	maxBm25Results, err := strconv.Atoi(maxBm25ResultsStr)
	if err != nil {
		return nil, fmt.Errorf("invalid BM25_MAX_RESULTS: %w", err)
	}
	cfg.Bm25MaxResults = maxBm25Results

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

// parseBytes parses a human-readable byte size string (e.g., "100M", "512K", "1G") into int64.
func parseBytes(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return 0, fmt.Errorf("empty size string")
	}
	multiplier := int64(1)
	switch s[len(s)-1] {
	case 'K', 'k':
		multiplier = 1024
	case 'M', 'm':
		multiplier = 1024 * 1024
	case 'G', 'g':
		multiplier = 1024 * 1024 * 1024
	default:
		// no suffix, treat as raw bytes
	}
	numStr := s
	if multiplier > 1 {
		numStr = s[:len(s)-1]
	}
	val, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q: %w", s, err)
	}
	return val * multiplier, nil
}
