package openviking

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yichozy/openvk-container/ov-sidecar/config"
)

func TestIsTextFile(t *testing.T) {
	t.Run("text file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.txt")
		if err := os.WriteFile(path, []byte("hello world\nline two"), 0644); err != nil {
			t.Fatal(err)
		}
		if !IsTextFile(path) {
			t.Error("expected text file")
		}
	})

	t.Run("binary file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.bin")
		data := []byte{0x89, 0x50, 0x4E, 0x47, 0x00, 0x00} // PNG header with null byte
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
		if IsTextFile(path) {
			t.Error("expected binary file")
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		if IsTextFile("/nonexistent/file.txt") {
			t.Error("expected false for nonexistent file")
		}
	})

	t.Run("empty file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "empty.txt")
		if err := os.WriteFile(path, []byte{}, 0644); err != nil {
			t.Fatal(err)
		}
		if !IsTextFile(path) {
			t.Error("empty file should be text")
		}
	})
}

func TestIsHiddenFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{".hidden", true},
		{"dir/.hidden", true},
		{".config/settings.yaml", true},
		{"src/main.go", false},
		{"docs/README.md", false},
		{"normal.txt", false},
		{"dir/sub/.env", true},
	}
	for _, tc := range tests {
		got := IsHiddenFile(tc.path)
		if got != tc.want {
			t.Errorf("IsHiddenFile(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestShouldExclude(t *testing.T) {
	excludes := []string{"temp/", "*.log", "_system/", "vectordb/"}

	tests := []struct {
		relPath string
		want    bool
	}{
		{"temp/somefile.txt", true},
		{"temp/sub/dir/file.txt", true},
		{"app.log", true},
		{"_system/config.json", true},
		{"vectordb/data", true},
		{"src/main.go", false},
		{"docs/README.md", false},
		{"readme.md", false},
	}
	for _, tc := range tests {
		got := ShouldExclude(tc.relPath, excludes)
		if got != tc.want {
			t.Errorf("ShouldExclude(%q) = %v, want %v", tc.relPath, got, tc.want)
		}
	}
}

func TestShouldExclude_EmptyPattern(t *testing.T) {
	got := ShouldExclude("src/main.go", []string{"", "  ", "*.log"})
	if got {
		t.Error("empty patterns should not match")
	}
}

func TestScanFiles(t *testing.T) {
	dir := t.TempDir()

	// Create test files
	os.MkdirAll(filepath.Join(dir, "src"), 0755)
	os.WriteFile(filepath.Join(dir, "src", "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(dir, "src", "util.go"), []byte("package main\n\nfunc util() {}"), 0644)

	os.MkdirAll(filepath.Join(dir, "temp"), 0755)
	os.WriteFile(filepath.Join(dir, "temp", "cache.tmp"), []byte("temp data"), 0644)

	// Binary file
	os.WriteFile(filepath.Join(dir, "binary.bin"), []byte{0x00, 0x01, 0x02}, 0644)

	cfg := &config.Config{
		OpenVikingPath:    dir,
		OpenVikingAccount: "default",
		OpenVikingPrefix:  dir + "/",
		Bm25MaxIndexFilesize: 1024 * 1024, // 1M
		Bm25Excludes:      []string{"temp/"},
	}

	docs, err := ScanFiles(dir, cfg)
	if err != nil {
		t.Fatalf("ScanFiles error: %v", err)
	}

	// Should include main.go and util.go, exclude temp/ and binary.bin
	if len(docs) != 2 {
		t.Fatalf("expected 2 docs, got %d", len(docs))
	}

	for _, d := range docs {
		t.Logf("doc URI: %s, Path: %s", d.URI, d.Path)
	}

	uris := make(map[string]bool)
	for _, d := range docs {
		uris[d.URI] = true
	}
	if !uris["viking://src/main.go"] {
		t.Error("missing viking://src/main.go")
	}
	if !uris["viking://src/util.go"] {
		t.Error("missing viking://src/util.go")
	}
}

func TestScanFiles_RespectsFileSizeLimit(t *testing.T) {
	dir := t.TempDir()

	// Create a file larger than limit
	largeContent := make([]byte, 100)
	for i := range largeContent {
		largeContent[i] = 'a'
	}
	os.WriteFile(filepath.Join(dir, "large.txt"), largeContent, 0644)

	cfg := &config.Config{
		OpenVikingPath:    dir,
		OpenVikingAccount: "default",
		OpenVikingPrefix:  dir + "/",
		Bm25MaxIndexFilesize: 50, // only 50 bytes
		Bm25Excludes:      nil,
	}

	docs, err := ScanFiles(dir, cfg)
	if err != nil {
		t.Fatalf("ScanFiles error: %v", err)
	}
	if len(docs) != 0 {
		t.Errorf("expected 0 docs (file too large), got %d", len(docs))
	}
}

func TestScanFiles_SkipsEmptyFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "empty.txt"), []byte{}, 0644)

	cfg := &config.Config{
		OpenVikingPath:    dir,
		OpenVikingAccount: "default",
		OpenVikingPrefix:  dir + "/",
		Bm25MaxIndexFilesize: 1024,
		Bm25Excludes:      nil,
	}

	docs, err := ScanFiles(dir, cfg)
	if err != nil {
		t.Fatalf("ScanFiles error: %v", err)
	}
	if len(docs) != 0 {
		t.Errorf("expected 0 docs (empty file), got %d", len(docs))
	}
}
