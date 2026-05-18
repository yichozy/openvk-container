package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testConfig(t *testing.T) *Config {
	t.Helper()

	openVikingAccount := "default"
	root := t.TempDir()
	openVikingPath := filepath.Join(root, "viking")
	baseDir := filepath.Join(openVikingPath, openVikingAccount, "resources", "curation", "TNBC")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	files := map[string]string{
		filepath.Join(baseDir, "a.md"):      "PFS overall survival\n",
		filepath.Join(baseDir, "b.md"):      "overall PFS\n",
		filepath.Join(baseDir, "c.md"):      "PFS\n",
		filepath.Join(baseDir, "d.txt"):     "PFS something\n",
		filepath.Join(baseDir, "e.txt"):     "PFS and disease free survival\n",
		filepath.Join(baseDir, "regex.txt"): "progress something survival\n",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write file %s: %v", path, err)
		}
	}

	return &Config{
		Port:             "1935",
		Timeout:          10 * time.Second,
		MaxGrepResults:   500,
		MaxGrepFilesize:  "50M",
		GrepThreads:      1,
		MaxConcurrency:   1,
		OpenVikingPath:   openVikingPath,
		OpenVikingPrefix: filepath.Clean(openVikingPath) + "/" + openVikingAccount + "/",
	}
}

// ==================== resolveDirectory ====================

func TestResolveDirectory_WithVikingScheme(t *testing.T) {
	cfg := testConfig(t)

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
	cfg := testConfig(t)

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
	cfg := testConfig(t)

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
	cfg := testConfig(t)

	_, err := resolveDirectory(cfg, "viking://../../etc/passwd")
	if err != ErrPathTraversal {
		t.Errorf("expected ErrPathTraversal, got %v", err)
	}
}

func TestResolveDirectory_PathTraversalWithoutScheme(t *testing.T) {
	cfg := testConfig(t)

	_, err := resolveDirectory(cfg, "../../etc/passwd")
	if err != ErrPathTraversal {
		t.Errorf("expected ErrPathTraversal, got %v", err)
	}
}

// ==================== Search (集成测试，依赖真实数据和 rg) ====================

func TestSearch_BasicPattern(t *testing.T) {
	cfg := testConfig(t)
	ctx := context.Background()

	result, err := Search(ctx, cfg, &SearchRequest{
		Pattern:     "PFS",
		Directories: []string{"viking://resources/curation/TNBC"},
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
	cfg := testConfig(t)
	ctx := context.Background()

	result, err := Search(ctx, cfg, &SearchRequest{
		Pattern:     "PFS",
		Directories: []string{"viking://resources/curation/TNBC"},
		Glob:        "*.md",
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
	cfg := testConfig(t)
	ctx := context.Background()

	result, err := Search(ctx, cfg, &SearchRequest{
		Pattern:     "PFS",
		Directories: []string{"viking://resources/curation/TNBC"},
		MaxResults:  3,
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
	cfg := testConfig(t)
	ctx := context.Background()

	result, err := Search(ctx, cfg, &SearchRequest{
		Pattern:     "ZZZNONEXISTENT_PATTERN_XYZ_999",
		Directories: []string{"viking://resources/curation/TNBC"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.URIs) != 0 {
		t.Errorf("expected 0 files, got %d", len(result.URIs))
	}
}

func TestSearch_InvalidRegex(t *testing.T) {
	cfg := testConfig(t)
	ctx := context.Background()

	_, err := Search(ctx, cfg, &SearchRequest{
		Pattern:     "[invalid",
		Directories: []string{"viking://resources/curation/TNBC"},
	})
	if err == nil {
		t.Fatal("expected error for invalid regex, got nil")
	}
}

func TestSearch_PathTraversal(t *testing.T) {
	cfg := testConfig(t)
	ctx := context.Background()

	_, err := Search(ctx, cfg, &SearchRequest{
		Pattern:     "test",
		Directories: []string{"viking://../../etc"},
	})
	if err != ErrPathTraversal {
		t.Errorf("expected ErrPathTraversal, got %v", err)
	}
}

func TestSearch_RegexPattern(t *testing.T) {
	cfg := testConfig(t)
	ctx := context.Background()

	result, err := Search(ctx, cfg, &SearchRequest{
		Pattern:     "(progress|disease).*(free|survival)",
		Directories: []string{"viking://resources/curation/TNBC"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.URIs) == 0 {
		t.Error("expected at least one file matching regex pattern")
	}
}

func TestSearch_WithoutVikingScheme(t *testing.T) {
	cfg := testConfig(t)
	ctx := context.Background()

	withScheme, _ := Search(ctx, cfg, &SearchRequest{
		Pattern:     "PFS",
		Directories: []string{"viking://resources/curation/TNBC"},
	})
	withoutScheme, _ := Search(ctx, cfg, &SearchRequest{
		Pattern:     "PFS",
		Directories: []string{"resources/curation/TNBC"},
	})
	if len(withScheme.URIs) != len(withoutScheme.URIs) {
		t.Errorf("viking:// and non-viking:// should return same results: %d vs %d",
			len(withScheme.URIs), len(withoutScheme.URIs))
	}
}

// ==================== PCRE2 / multi-word patterns ====================

// TestSearch_PCRE2Lookahead tests the AND-in-any-order pattern using PCRE2 lookaheads.
// Equivalent to: grep -P '(?=.*PFS)(?=.*overall)' file.txt
// Requires --engine auto to transparently switch to PCRE2.
func TestSearch_PCRE2Lookahead(t *testing.T) {
	cfg := testConfig(t)
	ctx := context.Background()

	result, err := Search(ctx, cfg, &SearchRequest{
		Pattern:     "(?=.*PFS)(?=.*overall)",
		Directories: []string{"viking://resources/curation/TNBC"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.URIs) == 0 {
		t.Error("expected at least one file matching PCRE2 lookahead pattern")
	}
	for _, f := range result.URIs {
		if !strings.HasPrefix(f, "viking://") {
			t.Errorf("expected viking:// prefix, got %q", f)
		}
	}
	t.Logf("PCRE2 lookahead matched %d files", len(result.URIs))
}

// TestSearch_AlternationOrder tests the OR-with-order pattern using standard regex.
// Equivalent to: grep -E "PFS.*overall|overall.*PFS" file.txt
func TestSearch_AlternationOrder(t *testing.T) {
	cfg := testConfig(t)
	ctx := context.Background()

	result, err := Search(ctx, cfg, &SearchRequest{
		Pattern:     "PFS.*overall|overall.*PFS",
		Directories: []string{"viking://resources/curation/TNBC"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.URIs) == 0 {
		t.Error("expected at least one file matching alternation pattern")
	}
	for _, f := range result.URIs {
		if !strings.HasPrefix(f, "viking://") {
			t.Errorf("expected viking:// prefix, got %q", f)
		}
	}
	t.Logf("alternation order matched %d files", len(result.URIs))
}

// TestSearch_PCRE2LookaheadSubsetOfAlternation verifies that the lookahead (AND)
// pattern returns a superset of the alternation (ordered OR) pattern, since
// AND-in-any-order is strictly more permissive than requiring a specific order.
func TestSearch_PCRE2LookaheadSubsetOfAlternation(t *testing.T) {
	cfg := testConfig(t)
	ctx := context.Background()

	lookahead, err := Search(ctx, cfg, &SearchRequest{
		Pattern:     "(?=.*PFS)(?=.*overall)",
		Directories: []string{"viking://resources/curation/TNBC"},
	})
	if err != nil {
		t.Fatalf("lookahead search failed: %v", err)
	}

	alternation, err := Search(ctx, cfg, &SearchRequest{
		Pattern:     "PFS.*overall|overall.*PFS",
		Directories: []string{"viking://resources/curation/TNBC"},
	})
	if err != nil {
		t.Fatalf("alternation search failed: %v", err)
	}

	// Build a set from lookahead results
	lookaheadSet := make(map[string]bool, len(lookahead.URIs))
	for _, uri := range lookahead.URIs {
		lookaheadSet[uri] = true
	}

	// Every alternation result should also appear in the lookahead results
	for _, uri := range alternation.URIs {
		if !lookaheadSet[uri] {
			t.Errorf("alternation result %q not found in lookahead results", uri)
		}
	}

	t.Logf("lookahead=%d files, alternation=%d files (lookahead >= alternation ✓)",
		len(lookahead.URIs), len(alternation.URIs))
}
