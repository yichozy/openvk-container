package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testConfig 基于真实 .env 的路径配置
func testConfig() *Config {
	dataDir := os.Getenv("GREP_DATA_DIR")
	if dataDir == "" {
		dataDir = "/Users/binhuchen/workspace/openvk-container/data"
	}
	openVikingPath := os.Getenv("OPEN_VIKING_DATA_PATH")
	if openVikingPath == "" {
		openVikingPath = "/Users/binhuchen/workspace/openvk-container/data/viking"
	}
	openVikingAccount := os.Getenv("OPEN_VIKING_ACCOUNT")
	if openVikingAccount == "" {
		openVikingAccount = "default"
	}
	return &Config{
		Port:             "1935",
		DataDir:          dataDir,
		Timeout:          10 * time.Second,
		MaxResults:       500,
		MaxFilesize:      "50M",
		OpenVikingPath:   openVikingPath,
		OpenVikingPrefix: filepath.Clean(openVikingPath) + "/" + openVikingAccount + "/",
	}
}

// ==================== resolveDirectory ====================

func TestResolveDirectory_WithVikingScheme(t *testing.T) {
	cfg := testConfig()

	result, err := resolveDirectory(cfg, "viking://resources/curation/TNBC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := cfg.OpenVikingPrefix + "resources/curation/TNBC"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestResolveDirectory_WithoutVikingScheme(t *testing.T) {
	cfg := testConfig()

	result, err := resolveDirectory(cfg, "resources/curation/TNBC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := cfg.OpenVikingPrefix + "resources/curation/TNBC"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestResolveDirectory_LeadingSlash(t *testing.T) {
	cfg := testConfig()

	result, err := resolveDirectory(cfg, "/resources/curation/TNBC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := cfg.OpenVikingPrefix + "resources/curation/TNBC"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestResolveDirectory_PathTraversal(t *testing.T) {
	cfg := testConfig()

	_, err := resolveDirectory(cfg, "viking://../../etc/passwd")
	if err != ErrPathTraversal {
		t.Errorf("expected ErrPathTraversal, got %v", err)
	}
}

func TestResolveDirectory_PathTraversalWithoutScheme(t *testing.T) {
	cfg := testConfig()

	_, err := resolveDirectory(cfg, "../../etc/passwd")
	if err != ErrPathTraversal {
		t.Errorf("expected ErrPathTraversal, got %v", err)
	}
}

// ==================== Search (集成测试，依赖真实数据和 rg) ====================

func TestSearch_BasicPattern(t *testing.T) {
	cfg := testConfig()
	ctx := context.Background()

	result, err := Search(ctx, cfg, &SearchRequest{
		Pattern:   "PFS",
		Directory: "viking://resources/curation/TNBC",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.URIs) == 0 {
		t.Error("expected at least one file matching PFS")
	}
	for _, f := range result.URIs {
		if !strings.HasPrefix(f, "viking://") {
			t.Errorf("expected viking:// prefix, got %q", f)
		}
	}
	if result.Truncated {
		t.Error("unexpected truncation")
	}
}

func TestSearch_WithGlob(t *testing.T) {
	cfg := testConfig()
	ctx := context.Background()

	result, err := Search(ctx, cfg, &SearchRequest{
		Pattern:   "PFS",
		Directory: "viking://resources/curation/TNBC",
		Glob:      "*.md",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.URIs) == 0 {
		t.Error("expected at least one .md file matching PFS")
	}
	for _, f := range result.URIs {
		if filepath.Ext(f) != ".md" {
			t.Errorf("expected .md extension, got %q", f)
		}
	}
}

func TestSearch_MaxResults(t *testing.T) {
	cfg := testConfig()
	ctx := context.Background()

	result, err := Search(ctx, cfg, &SearchRequest{
		Pattern:    "PFS",
		Directory:  "viking://resources/curation/TNBC",
		MaxResults: 3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.URIs) > 3 {
		t.Errorf("expected at most 3 files, got %d", len(result.URIs))
	}
	if len(result.URIs) == 3 && !result.Truncated {
		t.Error("expected truncated=true when hitting max_results")
	}
}

func TestSearch_NoMatch(t *testing.T) {
	cfg := testConfig()
	ctx := context.Background()

	result, err := Search(ctx, cfg, &SearchRequest{
		Pattern:   "ZZZNONEXISTENT_PATTERN_XYZ_999",
		Directory: "viking://resources/curation/TNBC",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.URIs) != 0 {
		t.Errorf("expected 0 files, got %d", len(result.URIs))
	}
}

func TestSearch_InvalidRegex(t *testing.T) {
	cfg := testConfig()
	ctx := context.Background()

	_, err := Search(ctx, cfg, &SearchRequest{
		Pattern:   "[invalid",
		Directory: "viking://resources/curation/TNBC",
	})
	if err == nil {
		t.Fatal("expected error for invalid regex, got nil")
	}
}

func TestSearch_PathTraversal(t *testing.T) {
	cfg := testConfig()
	ctx := context.Background()

	_, err := Search(ctx, cfg, &SearchRequest{
		Pattern:   "test",
		Directory: "viking://../../etc",
	})
	if err != ErrPathTraversal {
		t.Errorf("expected ErrPathTraversal, got %v", err)
	}
}

func TestSearch_RegexPattern(t *testing.T) {
	cfg := testConfig()
	ctx := context.Background()

	result, err := Search(ctx, cfg, &SearchRequest{
		Pattern:   "(progress|disease).*(free|survival)",
		Directory: "viking://resources/curation/TNBC",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.URIs) == 0 {
		t.Error("expected at least one file matching regex pattern")
	}
}

func TestSearch_WithoutVikingScheme(t *testing.T) {
	cfg := testConfig()
	ctx := context.Background()

	withScheme, _ := Search(ctx, cfg, &SearchRequest{
		Pattern:   "PFS",
		Directory: "viking://resources/curation/TNBC",
	})
	withoutScheme, _ := Search(ctx, cfg, &SearchRequest{
		Pattern:   "PFS",
		Directory: "resources/curation/TNBC",
	})
	if len(withScheme.URIs) != len(withoutScheme.URIs) {
		t.Errorf("viking:// and non-viking:// should return same results: %d vs %d",
			len(withScheme.URIs), len(withoutScheme.URIs))
	}
}
