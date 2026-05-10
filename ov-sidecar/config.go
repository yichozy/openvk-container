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
	OpenVikingPath    string
	OpenVikingAccount string
	OpenVikingPrefix  string // OpenVikingPath + "/" + OpenVikingAccount + "/"

	SyncEnabled  bool
	SyncSource   string
	SyncDests    []string
	SyncExcludes []string
	SyncInterval time.Duration

	RsyncDaemonEnabled bool
	RsyncDaemonPort    string
	RsyncModulePath    string
}

const defaultExcludes = ".openviking.pid,temp/,_system/queue/,_system/redo/,log/"

func Load() (*Config, error) {
	cfg := &Config{
		Port:              getEnv("SIDECAR_PORT", "1935"),
		Timeout:           parseDuration("SIDECAR_TIMEOUT", "30s"),
		OpenVikingPath:    os.Getenv("OPEN_VIKING_DATA_PATH"),
		OpenVikingAccount: getEnv("OPEN_VIKING_ACCOUNT", "default"),
	}

	if cfg.OpenVikingPath == "" {
		return nil, fmt.Errorf("OPEN_VIKING_DATA_PATH is required")
	}
	cfg.OpenVikingPrefix = filepath.Clean(cfg.OpenVikingPath) + "/" + cfg.OpenVikingAccount + "/"

	maxResultsStr := getEnv("MAX_GREP_RESULTS", "500")
	maxResults, err := strconv.Atoi(maxResultsStr)
	if err != nil {
		return nil, fmt.Errorf("invalid MAX_GREP_RESULTS: %w", err)
	}
	cfg.MaxGrepResults = maxResults

	cfg.MaxGrepFilesize = getEnv("MAX_GREP_FILESIZE", "50M")

	// Sync config
	if syncSource := os.Getenv("SYNC_SOURCE_DIR"); syncSource != "" {
		if syncDests := os.Getenv("SYNC_DEST_DIRS"); syncDests != "" {
			cfg.SyncEnabled = true
			cfg.SyncSource = syncSource
			cfg.SyncDests = splitPaths(syncDests)
			cfg.SyncInterval = parseDuration("SYNC_INTERVAL", "30m")
			cfg.SyncExcludes = splitPaths(getEnv("SYNC_EXCLUDES", defaultExcludes))
		}
	}

	// Rsync daemon config (replica mode)
	if os.Getenv("RSYNC_DAEMON_ENABLED") == "true" {
		cfg.RsyncDaemonEnabled = true
		cfg.RsyncDaemonPort = getEnv("RSYNC_DAEMON_PORT", "873")
		cfg.RsyncModulePath = getEnv("RSYNC_MODULE_PATH", cfg.OpenVikingPath)
		if cfg.RsyncModulePath == "" {
			return nil, fmt.Errorf("RSYNC_MODULE_PATH is required when rsync daemon is enabled")
		}
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func parseDuration(key, fallback string) time.Duration {
	value := getEnv(key, fallback)
	duration, err := time.ParseDuration(value)
	if err != nil {
		duration, _ = time.ParseDuration(fallback)
		zap.L().Warn("invalid duration, using fallback",
			zap.String("key", key),
			zap.String("value", value),
			zap.String("fallback", fallback),
			zap.Error(err),
		)
	}
	return duration
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
