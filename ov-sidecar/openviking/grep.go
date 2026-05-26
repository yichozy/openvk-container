package openviking

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	"github.com/yichozy/openvk-container/ov-sidecar/cache"
	"github.com/yichozy/openvk-container/ov-sidecar/config"
)

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

func Search(ctx context.Context, cfg *config.Config, c cache.Cache, req *SearchRequest) (*SearchData, error) {
	searchDirs := make([]string, 0, len(req.Directories))
	for _, dir := range req.Directories {
		resolved, err := ResolveURI(cfg, dir)
		if err != nil {
			return nil, err
		}
		searchDirs = append(searchDirs, resolved)
	}

	maxResults := cfg.MaxGrepResults
	if req.MaxResults > 0 && req.MaxResults < maxResults {
		maxResults = req.MaxResults
	}

	cacheEnabled := c != nil && cfg.GrepCacheTTL > 0
	normalized := cache.NormalizeDirectories(searchDirs)

	sfKey := buildSingleflightKey(req.Pattern, normalized, NormalizeGlob(req.Glob), req.Hidden, maxResults, cfg.MaxGrepFilesize)

	cacheKey, cacheKeyErr := cache.BuildGrepCacheKey(cfg.GrepCachePrefix, cache.GrepCacheKeyPayload{
		Pattern:          req.Pattern,
		Directories:      normalized,
		Glob:             NormalizeGlob(req.Glob),
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
			if cached, ok, getErr := c.Get(ctx, cacheKey); getErr == nil && ok {
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
				if setErr := c.Set(ctx, cacheKey, string(encoded), cfg.GrepCacheTTL); setErr != nil {
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

func executeRipgrep(ctx context.Context, cfg *config.Config, req *SearchRequest, searchDirs []string, maxResults int) (*SearchData, error) {
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

// NormalizeGlob trims whitespace from a glob pattern.
func NormalizeGlob(glob string) string {
	return strings.TrimSpace(glob)
}
