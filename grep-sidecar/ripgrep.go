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

	"github.com/yichozy/hopebox/log"
)

var ErrPathTraversal = errors.New("path traversal denied")

const vikingScheme = "viking://"

type SearchRequest struct {
	Pattern    string `json:"pattern" binding:"required"`
	Directory  string `json:"directory" binding:"required"`
	Glob       string `json:"glob,omitempty"`
	MaxResults int    `json:"max_results,omitempty"`
	Hidden     bool   `json:"hidden,omitempty"`
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
	searchDir, err := resolveDirectory(cfg, req.Directory)
	if err != nil {
		return nil, err
	}

	args := []string{
		"-l",
		"--max-filesize", cfg.MaxFilesize,
	}
	if req.Hidden {
		args = append(args, "--hidden", "--no-ignore-vcs")
	}
	if req.Glob != "" {
		args = append(args, "--glob", req.Glob)
	}
	args = append(args, "--", req.Pattern, searchDir)

	log.Infow(ctx, "executing ripgrep", "args", args)

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

	maxResults := cfg.MaxResults
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
			cmd.Wait()
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
	cmd.Wait()

	// Check for ripgrep error (exit code 2 = regex error, bad args, etc.)
	if len(uris) == 0 && !truncated {
		if exitErr, ok := cmd.ProcessState.Sys().(interface{ ExitStatus() int }); ok {
			if exitErr.ExitStatus() == 2 {
				return nil, fmt.Errorf("invalid regex: %s", strings.TrimSpace(string(stderrBytes)))
			}
		}
	}

	return &SearchData{
		URIs:      uris,
		Truncated: truncated,
	}, nil
}
