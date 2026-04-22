package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Config struct {
	Port             string
	DataDir          string
	Timeout          time.Duration
	MaxResults       int
	MaxFilesize      string
	OpenVikingPath   string // OPEN_VIKING_DATA_PATH, e.g. /data/workspace/viking
	OpenVikingPrefix string // computed: OpenVikingPath + "/" + OpenVikingAccount + "/"
}

func Load() (*Config, error) {
	port := os.Getenv("GREP_PORT")
	if port == "" {
		port = "1935"
	}

	dataDir := os.Getenv("GREP_DATA_DIR")
	if dataDir == "" {
		dataDir = "/data"
	}

	timeoutStr := os.Getenv("GREP_TIMEOUT")
	if timeoutStr == "" {
		timeoutStr = "30s"
	}
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return nil, fmt.Errorf("invalid GREP_TIMEOUT: %w", err)
	}

	maxResultsStr := os.Getenv("GREP_MAX_RESULTS")
	if maxResultsStr == "" {
		maxResultsStr = "500"
	}
	maxResults, err := strconv.Atoi(maxResultsStr)
	if err != nil {
		return nil, fmt.Errorf("invalid GREP_MAX_RESULTS: %w", err)
	}

	maxFilesize := os.Getenv("GREP_MAX_FILESIZE")
	if maxFilesize == "" {
		maxFilesize = "10M"
	}

	openVikingPath := os.Getenv("OPEN_VIKING_DATA_PATH")
	if openVikingPath == "" {
		openVikingPath = "/data/workspace/viking"
	}

	openVikingAccount := os.Getenv("OPEN_VIKING_ACCOUNT")
	if openVikingAccount == "" {
		openVikingAccount = "default"
	}
	openVikingPrefix := filepath.Clean(openVikingPath) + "/" + openVikingAccount + "/"

	info, err := os.Stat(dataDir)
	if err != nil {
		return nil, fmt.Errorf("data directory %s: %w", dataDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("data directory %s is not a directory", dataDir)
	}

	return &Config{
		Port:             port,
		DataDir:          dataDir,
		Timeout:          timeout,
		MaxResults:       maxResults,
		MaxFilesize:      maxFilesize,
		OpenVikingPath:   openVikingPath,
		OpenVikingPrefix: openVikingPrefix,
	}, nil
}
