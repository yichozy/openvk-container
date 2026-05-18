package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

var ErrPathTraversal = errors.New("path traversal denied")

const vikingScheme = "viking://"

type SearchRequest struct {
	Pattern     string   `json:"pattern" binding:"required"`
	Directories []string `json:"directories" binding:"required,min=1"`
	Glob        string   `json:"glob,omitempty"`
	MaxResults  int      `json:"max_results,omitempty"`
	Hidden      bool     `json:"hidden,omitempty"`
}

type SearchData struct {
	URIs      []string `json:"uris"`
	Truncated bool     `json:"truncated"`
}

// resolveDirectory 将 directory 解析为完整的文件系统路径。
// 输入 "viking://resources/curation/cardio" → "/data/workspace/viking/default/resources/curation/cardio"
// 输入 "resources/curation/cardio"       → "/data/workspace/viking/default/resources/curation/cardio"
func resolveDirectory(cfg *Config, dir string) (string, error) {
	cleanDir := strings.TrimPrefix(dir, vikingScheme)
	cleanDir = strings.TrimPrefix(cleanDir, "/")

	fullPath := filepath.Join(cfg.OpenVikingPrefix, cleanDir)

	rel, err := filepath.Rel(cfg.OpenVikingPrefix, fullPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", ErrPathTraversal
	}
	return fullPath, nil
}

func Search(ctx context.Context, cfg *Config, req *SearchRequest) (*SearchData, error) {
	searchDirs := make([]string, 0, len(req.Directories))
	for _, dir := range req.Directories {
		resolved, err := resolveDirectory(cfg, dir)
		if err != nil {
			return nil, err
		}
		searchDirs = append(searchDirs, resolved)
	}

	args := []string{
		"-l",
		"--engine", "auto",
		"--max-filesize", cfg.MaxGrepFilesize,
	}
	if cfg.GrepThreads > 0 {
		args = append(args, "--threads", fmt.Sprintf("%d", cfg.GrepThreads))
	}
	if req.Hidden {
		args = append(args, "--hidden", "--no-ignore-vcs")
	}
	if req.Glob != "" {
		args = append(args, "--glob", req.Glob)
	}
	args = append(args, "--", req.Pattern)
	args = append(args, searchDirs...)

	zap.L().Info("executing ripgrep", zap.Strings("args", args))

	cmd := exec.CommandContext(ctx, "rg", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("create pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start ripgrep: %w", err)
	}

	maxResults := cfg.MaxGrepResults
	if req.MaxResults > 0 && req.MaxResults < maxResults {
		maxResults = req.MaxResults
	}

	var uris []string
	truncated := false
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			cmd.Process.Kill()
			io.Copy(io.Discard, stdout)
			cmd.Wait()
			return nil, ctx.Err()
		default:
		}

		if len(uris) >= maxResults {
			truncated = true
			cmd.Process.Kill()
			io.Copy(io.Discard, stdout)
			break
		}

		rawPath := scanner.Text()
		if rel, err := filepath.Rel(cfg.OpenVikingPrefix, rawPath); err == nil && !strings.HasPrefix(rel, "..") {
			uris = append(uris, "viking://"+rel)
		} else {
			uris = append(uris, rawPath)
		}
	}

	// Drain stderr
	stderrBytes, _ := io.ReadAll(stderr)
	waitErr := cmd.Wait()

	if !truncated && waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			exitCode := exitErr.ExitCode()
			stderrStr := strings.TrimSpace(string(stderrBytes))
			if exitCode == 1 {
				waitErr = nil
			} else if exitCode == 2 && isInvalidRegex(stderrStr) {
				return nil, fmt.Errorf("invalid regex: %s", stderrStr)
			} else if stderrStr != "" {
				return nil, fmt.Errorf("ripgrep failed: %s", stderrStr)
			}
		}
		if waitErr != nil {
			return nil, fmt.Errorf("ripgrep failed: %w", waitErr)
		}
	}

	return &SearchData{
		URIs:      uris,
		Truncated: truncated,
	}, nil
}

func isInvalidRegex(stderr string) bool {
	s := strings.ToLower(stderr)
	return strings.Contains(s, "regex parse error") ||
		strings.Contains(s, "error parsing") ||
		strings.Contains(s, "unclosed") ||
		strings.Contains(s, "invalid utf-8") ||
		strings.Contains(s, "pcre2") && strings.Contains(s, "error")
}
