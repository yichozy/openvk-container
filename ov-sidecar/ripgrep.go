package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
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

var grepSF singleflight.Group

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

	maxResults := cfg.MaxGrepResults
	if req.MaxResults > 0 && req.MaxResults < maxResults {
		maxResults = req.MaxResults
	}

	cacheEnabled := cfg.RedisAddr != "" && cfg.GrepCacheTTL > 0 && grepCache != nil
	normalized := normalizeDirectories(searchDirs)

	sfKey := buildSingleflightKey(req.Pattern, normalized, normalizeGlob(req.Glob), req.Hidden, maxResults, cfg.MaxGrepFilesize)

	cacheKey, cacheKeyErr := buildGrepCacheKey(cfg.GrepCachePrefix, grepCacheKeyPayload{
		Pattern:          req.Pattern,
		Directories:      normalized,
		Glob:             normalizeGlob(req.Glob),
		Hidden:           req.Hidden,
		EffectiveMax:     maxResults,
		CfgMaxFilesize:   cfg.MaxGrepFilesize,
		OpenVikingPrefix: cfg.OpenVikingPrefix,
	})
	if cacheKeyErr != nil {
		cacheEnabled = false
	}

	v, err, _ := grepSF.Do(sfKey, func() (any, error) {
		if cacheEnabled {
			if cached, ok, getErr := grepCache.Get(ctx, cacheKey); getErr == nil && ok {
				var d SearchData
				if err := json.Unmarshal([]byte(cached), &d); err == nil {
					keyLog := cacheKey
					if len(cacheKey) > 8 {
						keyLog = cacheKey[len(cacheKey)-8:]
					}
					zap.L().Info("grep cache hit", zap.String("key", keyLog))
					return &d, nil
				}
			} else if getErr != nil {
				zap.L().Warn("grep cache get failed", zap.Error(getErr))
			}
		}

		res, err := executeRipgrep(ctx, cfg, req, searchDirs, maxResults)
		if err != nil {
			return nil, err
		}

		if cacheEnabled {
			encoded, encErr := json.Marshal(res)
			if encErr == nil {
				if setErr := grepCache.Set(ctx, cacheKey, string(encoded), cfg.GrepCacheTTL); setErr != nil {
					zap.L().Warn("grep cache set failed", zap.Error(setErr))
				}
			}
		}

		return res, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*SearchData), nil
}

func executeRipgrep(ctx context.Context, cfg *Config, req *SearchRequest, searchDirs []string, maxResults int) (*SearchData, error) {
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

// buildSingleflightKey produces a dedupe key from request parameters,
// independent of cache key construction. Uses \x00 as separator to prevent collisions.
func buildSingleflightKey(pattern string, normalizedDirs []string, glob string, hidden bool, maxResults int, maxFilesize string) string {
	b := strings.Builder{}
	b.WriteString(pattern)
	b.WriteByte(0)
	for _, d := range normalizedDirs {
		b.WriteString(d)
		b.WriteByte(0)
	}
	b.WriteString(glob)
	b.WriteByte(0)
	if hidden {
		b.WriteString("1")
	}
	b.WriteByte(0)
	b.WriteString(fmt.Sprintf("%d", maxResults))
	b.WriteByte(0)
	b.WriteString(maxFilesize)
	return b.String()
}
