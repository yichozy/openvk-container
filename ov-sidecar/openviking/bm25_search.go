package openviking

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/yichozy/openvk-container/ov-sidecar/config"
)

// Bm25SearchRequest represents a BM25 search request.
// Level controls which content levels to search:
//   - 0: .abstract.md files (concise summaries)
//   - 1: .overview.md files (structured overviews)
//   - 2: full content files
//
// Default: [0, 1, 2] (all levels).
// Highlight defaults to true; set to false to disable.
type Bm25SearchRequest struct {
	Pattern     string `json:"pattern" binding:"required"`
	Directories []string `json:"directories" binding:"required,min=1"`
	Glob        string `json:"glob,omitempty"`
	Level       []int  `json:"level,omitempty"`
	MaxResults  int    `json:"max_results,omitempty"`
	Highlight   *bool  `json:"highlight,omitempty"`
}

// Bm25Result represents a single BM25 search result.
type Bm25Result struct {
	URI       string   `json:"uri"`
	Score     float64  `json:"score"`
	Fragments []string `json:"fragments"`
	Level     int      `json:"level"`
}

// Bm25SearchData holds the full BM25 search response data.
type Bm25SearchData struct {
	Results   []Bm25Result `json:"results"`
	Total     int          `json:"total"`
	Truncated bool         `json:"truncated"`
}

// Bm25Search performs a BM25 search: resolves directories, then delegates to the Indexer.
func Bm25Search(ctx context.Context, cfg *config.Config, indexer *Indexer, req *Bm25SearchRequest) (*Bm25SearchData, error) {
	// Default to all levels if not specified
	if len(req.Level) == 0 {
		req.Level = []int{0, 1, 2}
	}

	// Default highlight to true
	highlight := true
	if req.Highlight != nil {
		highlight = *req.Highlight
	}

	searchDirs := make([]string, 0, len(req.Directories))
	for _, dir := range req.Directories {
		resolved, err := ResolveURI(cfg, dir)
		if err != nil {
			return nil, err
		}
		searchDirs = append(searchDirs, resolved)
	}

	maxResults := cfg.Bm25MaxResults
	if req.MaxResults > 0 && req.MaxResults < maxResults {
		maxResults = req.MaxResults
	}
	req.MaxResults = maxResults
	req.Highlight = &highlight

	result, err := indexer.Search(ctx, req, searchDirs)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("search timed out: %w", context.DeadlineExceeded)
		}
		return nil, err
	}

	// Strip highlight tags from fragments
	for i := range result.Results {
		for j, frag := range result.Results[i].Fragments {
			result.Results[i].Fragments[j] = stripMarkTags(frag)
		}
	}

	return result, nil
}

// stripMarkTags removes <mark> and </mark> tags from highlight fragments.
func stripMarkTags(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "<mark>", ""), "</mark>", "")
}

// DocLevel determines the content level of a document from its filename.
//   - ".abstract.md" → 0
//   - ".overview.md" → 1
//   - everything else → 2
func DocLevel(path string) int {
	base := filepath.Base(path)
	switch base {
	case ".abstract.md":
		return 0
	case ".overview.md":
		return 1
	default:
		return 2
	}
}
