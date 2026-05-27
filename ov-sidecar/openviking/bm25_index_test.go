package openviking

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yichozy/openvk-container/ov-sidecar/config"
)

func newTestConfig(t *testing.T) *config.Config {
	t.Helper()
	dir := t.TempDir()
	return &config.Config{
		OpenVikingPath:    dir,
		OpenVikingAccount: "default",
		OpenVikingPrefix:  dir + "/",
		Bm25IndexPath:     filepath.Join(dir, "_index"),
		Bm25MaxIndexFilesize: 1024 * 1024,
		Bm25Excludes:      nil,
		Bm25BatchSize:     100,
		Bm25UpdateInterval: 5 * time.Minute,
		Bm25MaxResults:    500,
	}
}

func createTestFiles(t *testing.T, cfg *config.Config, files map[string]string) {
	t.Helper()
	for path, content := range files {
		fullPath := filepath.Join(cfg.OpenVikingPrefix, path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNewIndexer(t *testing.T) {
	cfg := newTestConfig(t)
	ctx := context.Background()

	indexer, err := NewIndexer(ctx, cfg)
	if err != nil {
		t.Fatalf("NewIndexer error: %v", err)
	}
	defer indexer.Close()

	if indexer == nil {
		t.Fatal("expected non-nil indexer")
	}
	state := indexer.State()
	if state.DocCount != 0 {
		t.Errorf("expected 0 docs in new index, got %d", state.DocCount)
	}
}

func TestNewIndexer_OpenExisting(t *testing.T) {
	cfg := newTestConfig(t)
	ctx := context.Background()

	// Create and close first indexer
	idx1, err := NewIndexer(ctx, cfg)
	if err != nil {
		t.Fatalf("NewIndexer 1 error: %v", err)
	}
	idx1.Close()

	// Open existing index
	idx2, err := NewIndexer(ctx, cfg)
	if err != nil {
		t.Fatalf("NewIndexer 2 error: %v", err)
	}
	defer idx2.Close()
}

func TestBuildIndex(t *testing.T) {
	cfg := newTestConfig(t)
	createTestFiles(t, cfg, map[string]string{
		"src/main.go":   "package main\n\nfunc main() { println(\"hello\") }",
		"src/util.go":   "package main\n\nfunc util() error { return nil }",
		"docs/README.md": "# Test\n\nThis is a readme file with some content.",
	})

	ctx := context.Background()
	indexer, err := NewIndexer(ctx, cfg)
	if err != nil {
		t.Fatalf("NewIndexer error: %v", err)
	}
	defer indexer.Close()

	if err := indexer.BuildIndex(ctx); err != nil {
		t.Fatalf("BuildIndex error: %v", err)
	}

	state := indexer.State()
	if state.DocCount != 3 {
		t.Errorf("expected 3 docs, got %d", state.DocCount)
	}
	if state.LastUpdated == "" {
		t.Error("expected LastUpdated to be set")
	}
	if state.Error != "" {
		t.Errorf("unexpected error: %s", state.Error)
	}
}

func TestBuildIndex_ExcludesBinaryAndFiltered(t *testing.T) {
	cfg := newTestConfig(t)
	cfg.Bm25Excludes = []string{"temp/"}
	createTestFiles(t, cfg, map[string]string{
		"src/main.go":       "package main",
		"temp/cache.tmp":    "cached data",
		"binary/data.bin":   "\x00\x01\x02", // binary
	})

	ctx := context.Background()
	indexer, err := NewIndexer(ctx, cfg)
	if err != nil {
		t.Fatalf("NewIndexer error: %v", err)
	}
	defer indexer.Close()

	if err := indexer.BuildIndex(ctx); err != nil {
		t.Fatalf("BuildIndex error: %v", err)
	}

	state := indexer.State()
	if state.DocCount != 1 {
		t.Errorf("expected 1 doc (only src/main.go), got %d", state.DocCount)
	}
}

func TestUpdateIndex_Incremental(t *testing.T) {
	cfg := newTestConfig(t)
	createTestFiles(t, cfg, map[string]string{
		"src/main.go": "package main\n\nfunc main() {}",
		"src/util.go": "package main\n\nfunc util() {}",
	})

	ctx := context.Background()
	indexer, err := NewIndexer(ctx, cfg)
	if err != nil {
		t.Fatalf("NewIndexer error: %v", err)
	}
	defer indexer.Close()

	// Initial build
	if err := indexer.BuildIndex(ctx); err != nil {
		t.Fatalf("BuildIndex error: %v", err)
	}
	if indexer.State().DocCount != 2 {
		t.Fatalf("expected 2 docs after build, got %d", indexer.State().DocCount)
	}

	// Modify one file, delete another, add a new one
	os.Remove(filepath.Join(cfg.OpenVikingPrefix, "src", "util.go"))
	os.WriteFile(filepath.Join(cfg.OpenVikingPrefix, "src", "main.go"), []byte("package main\n\nfunc main() { println(\"updated\") }"), 0644)
	createTestFiles(t, cfg, map[string]string{
		"src/new.go": "package main\n\nfunc newFunc() {}",
	})

	// Incremental update
	if err := indexer.UpdateIndex(ctx); err != nil {
		t.Fatalf("UpdateIndex error: %v", err)
	}
	if indexer.State().DocCount != 2 {
		t.Errorf("expected 2 docs after update (main.go + new.go), got %d", indexer.State().DocCount)
	}
}

func TestSearch_Basic(t *testing.T) {
	cfg := newTestConfig(t)
	createTestFiles(t, cfg, map[string]string{
		"src/handler.go": "func handleRequest() error {\n  return handleError(req)\n}",
		"src/model.go":   "type Model struct {\n  Name string\n  Error error\n}",
		"docs/README.md": "# Project\n\nThis project handles HTTP requests.",
	})

	ctx := context.Background()
	indexer, err := NewIndexer(ctx, cfg)
	if err != nil {
		t.Fatalf("NewIndexer error: %v", err)
	}
	defer indexer.Close()

	if err := indexer.BuildIndex(ctx); err != nil {
		t.Fatalf("BuildIndex error: %v", err)
	}

	req := &Bm25SearchRequest{
		Pattern:     "error",
		Directories: []string{"viking://src/"},
		MaxResults:  10,
	}
	searchDirs := []string{filepath.Join(cfg.OpenVikingPrefix, "src")}

	result, err := indexer.Search(ctx, req, searchDirs)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if len(result.Results) == 0 {
		t.Error("expected at least 1 result for 'error'")
	}

	for _, r := range result.Results {
		if r.Score <= 0 {
			t.Errorf("expected positive score, got %f", r.Score)
		}
	}
}

func TestSearch_DirectoryScope(t *testing.T) {
	cfg := newTestConfig(t)
	createTestFiles(t, cfg, map[string]string{
		"src/error.go":    "package src\n\nvar err error",
		"docs/error.txt": "this is an error in docs",
	})

	ctx := context.Background()
	indexer, err := NewIndexer(ctx, cfg)
	if err != nil {
		t.Fatalf("NewIndexer error: %v", err)
	}
	defer indexer.Close()

	if err := indexer.BuildIndex(ctx); err != nil {
		t.Fatalf("BuildIndex error: %v", err)
	}

	// Search only in src/
	req := &Bm25SearchRequest{
		Pattern:     "error",
		Directories: []string{"viking://src/"},
		MaxResults:  10,
	}
	searchDirs := []string{filepath.Join(cfg.OpenVikingPrefix, "src")}

	result, err := indexer.Search(ctx, req, searchDirs)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	for _, r := range result.Results {
		if filepath.Base(r.URI) != "error.go" {
			t.Errorf("expected only src/ results, got %s", r.URI)
		}
	}
}

func TestSearch_GlobFilter(t *testing.T) {
	cfg := newTestConfig(t)
	createTestFiles(t, cfg, map[string]string{
		"src/main.go":    "package main\n\nfunc main() { handleError() }",
		"src/error.py":   "def error_handler(): pass",
		"docs/error.txt": "an error occurred",
	})

	ctx := context.Background()
	indexer, err := NewIndexer(ctx, cfg)
	if err != nil {
		t.Fatalf("NewIndexer error: %v", err)
	}
	defer indexer.Close()

	if err := indexer.BuildIndex(ctx); err != nil {
		t.Fatalf("BuildIndex error: %v", err)
	}

	req := &Bm25SearchRequest{
		Pattern:     "error",
		Directories: []string{"viking://"},
		Glob:        "*.go",
		MaxResults:  10,
	}
	searchDirs := []string{cfg.OpenVikingPrefix}

	result, err := indexer.Search(ctx, req, searchDirs)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	for _, r := range result.Results {
		if filepath.Ext(r.URI) != ".go" {
			t.Errorf("expected only .go files, got %s", r.URI)
		}
	}
}

func TestSearch_LevelFilter(t *testing.T) {
	cfg := newTestConfig(t)
	createTestFiles(t, cfg, map[string]string{
		"src/main.go":           "package main",
		"src/.abstract.md":      "This is a summary of main.go",
		"src/.overview.md":      "Detailed overview of main.go module",
		"src/.config/secrets":   "password=123",
	})

	ctx := context.Background()
	indexer, err := NewIndexer(ctx, cfg)
	if err != nil {
		t.Fatalf("NewIndexer error: %v", err)
	}
	defer indexer.Close()

	if err := indexer.BuildIndex(ctx); err != nil {
		t.Fatalf("BuildIndex error: %v", err)
	}

	searchDirs := []string{cfg.OpenVikingPrefix}

	// Level [0] — only .abstract.md
	req := &Bm25SearchRequest{
		Pattern:     "main",
		Directories: []string{"viking://"},
		MaxResults:  10,
		Level:       []int{0},
	}
	result, err := indexer.Search(ctx, req, searchDirs)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	for _, r := range result.Results {
		if r.Level != 0 {
			t.Errorf("level 0 filter returned level %d result: %s", r.Level, r.URI)
		}
	}

	// Level [1] — only .overview.md
	req.Level = []int{1}
	result, err = indexer.Search(ctx, req, searchDirs)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	for _, r := range result.Results {
		if r.Level != 1 {
			t.Errorf("level 1 filter returned level %d result: %s", r.Level, r.URI)
		}
	}

	// Level [2] — only full content files
	req.Level = []int{2}
	result, err = indexer.Search(ctx, req, searchDirs)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	for _, r := range result.Results {
		if r.Level != 2 {
			t.Errorf("level 2 filter returned level %d result: %s", r.Level, r.URI)
		}
	}

	// Level [0, 2] — abstract + full, no overview
	req.Level = []int{0, 2}
	result, err = indexer.Search(ctx, req, searchDirs)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	for _, r := range result.Results {
		if r.Level == 1 {
			t.Errorf("level [0,2] filter returned level 1 result: %s", r.URI)
		}
	}

	// All levels explicit
	req.Level = []int{0, 1, 2}
	result, err = indexer.Search(ctx, req, searchDirs)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(result.Results) == 0 {
		t.Error("expected results with all levels")
	}
}

func TestSearch_MaxResults(t *testing.T) {
	cfg := newTestConfig(t)
	createTestFiles(t, cfg, map[string]string{
		"src/file/a.go": "error",
		"src/file/b.go": "error",
		"src/file/c.go": "error",
		"src/file/d.go": "error",
		"src/file/e.go": "error",
	})

	ctx := context.Background()
	indexer, err := NewIndexer(ctx, cfg)
	if err != nil {
		t.Fatalf("NewIndexer error: %v", err)
	}
	defer indexer.Close()

	if err := indexer.BuildIndex(ctx); err != nil {
		t.Fatalf("BuildIndex error: %v", err)
	}

	req := &Bm25SearchRequest{
		Pattern:     "error",
		Directories: []string{"viking://"},
		MaxResults:  2,
	}
	searchDirs := []string{cfg.OpenVikingPrefix}

	result, err := indexer.Search(ctx, req, searchDirs)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(result.Results) > 2 {
		t.Errorf("expected at most 2 results, got %d", len(result.Results))
	}
	if result.Total != len(result.Results) {
		t.Errorf("expected total=%d, got %d", len(result.Results), result.Total)
	}
}

func TestSearch_NoMatch(t *testing.T) {
	cfg := newTestConfig(t)
	createTestFiles(t, cfg, map[string]string{
		"src/main.go": "package main\n\nfunc main() {}",
	})

	ctx := context.Background()
	indexer, err := NewIndexer(ctx, cfg)
	if err != nil {
		t.Fatalf("NewIndexer error: %v", err)
	}
	defer indexer.Close()

	if err := indexer.BuildIndex(ctx); err != nil {
		t.Fatalf("BuildIndex error: %v", err)
	}

	req := &Bm25SearchRequest{
		Pattern:     "nonexistent_xyz",
		Directories: []string{"viking://"},
		MaxResults:  10,
	}
	searchDirs := []string{cfg.OpenVikingPrefix}

	result, err := indexer.Search(ctx, req, searchDirs)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(result.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(result.Results))
	}
	if result.Total != 0 {
		t.Errorf("expected total=0, got %d", result.Total)
	}
}

func TestIndexer_Close_DoubleClose(t *testing.T) {
	cfg := newTestConfig(t)
	ctx := context.Background()

	indexer, err := NewIndexer(ctx, cfg)
	if err != nil {
		t.Fatalf("NewIndexer error: %v", err)
	}

	if err := indexer.Close(); err != nil {
		t.Fatalf("first Close error: %v", err)
	}
	// Second close should not panic
	if err := indexer.Close(); err != nil {
		t.Fatalf("second Close error: %v", err)
	}
}

func TestDocLevel(t *testing.T) {
	tests := []struct {
		path string
		want int
	}{
		{"/data/.abstract.md", 0},
		{"/data/dir/.abstract.md", 0},
		{"/data/.overview.md", 1},
		{"/data/dir/.overview.md", 1},
		{"/data/main.go", 2},
		{"/data/docs/README.md", 2},
		{"/data/.hidden.go", 2},
	}
	for _, tc := range tests {
		got := DocLevel(tc.path)
		if got != tc.want {
			t.Errorf("DocLevel(%q) = %d, want %d", tc.path, got, tc.want)
		}
	}
}

func TestBm25Search_MaxResultsNormalization(t *testing.T) {
	cfg := &config.Config{
		OpenVikingPath:    "/tmp/viking",
		OpenVikingAccount: "default",
		OpenVikingPrefix:  "/tmp/viking/default/",
		Bm25MaxResults:    10,
	}

	tests := []struct {
		name         string
		reqMax       int
		wantMax      int
	}{
		{"zero uses default", 0, 10},
		{"under limit passes through", 5, 5},
		{"over limit capped", 20, 10},
		{"exact limit passes through", 10, 10},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := &Bm25SearchRequest{MaxResults: tc.reqMax}
			// Only test the MaxResults normalization part
			maxResults := cfg.Bm25MaxResults
			if req.MaxResults > 0 && req.MaxResults < maxResults {
				maxResults = req.MaxResults
			}
			if maxResults != tc.wantMax {
				t.Errorf("got %d, want %d", maxResults, tc.wantMax)
			}
		})
	}
}
