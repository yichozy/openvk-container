package openviking

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/yichozy/openvk-container/ov-sidecar/config"
)

// IndexedDocument represents a file indexed in the BM25 search engine.
type IndexedDocument struct {
	URI      string `json:"uri"`
	Path     string `json:"path"`
	Content  string `json:"content"`
	ModTime  int64  `json:"mod_time"`
	FileSize int64  `json:"file_size"`
}

// IsHiddenFile checks if any path component starts with "." (Unix hidden convention).
func IsHiddenFile(path string) bool {
	for _, part := range strings.Split(path, string(filepath.Separator)) {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}

// IsTextFile checks if a file is text by reading the first 512 bytes
// and looking for null bytes (binary indicator).
func IsTextFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if n == 0 {
		return true // empty file is text
	}
	if err != nil {
		return false
	}
	return !bytes.Contains(buf[:n], []byte{0})
}

// ShouldExclude checks if a file path matches any of the exclude patterns.
func ShouldExclude(relPath string, excludes []string) bool {
	base := filepath.Base(relPath)
	for _, pattern := range excludes {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		matched, err := filepath.Match(pattern, relPath)
		if err != nil {
			continue
		}
		if matched {
			return true
		}
		// Also try matching against basename (e.g., ".relations.json" should match any path)
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
		// Also try matching against individual path components for directory-level patterns
		// e.g., "temp/" should match "temp/somefile.txt"
		if strings.HasSuffix(pattern, "/") || strings.HasSuffix(pattern, string(filepath.Separator)) {
			prefix := strings.TrimSuffix(pattern, string(filepath.Separator)) + string(filepath.Separator)
			if strings.HasPrefix(relPath, prefix) {
				return true
			}
		}
	}
	return false
}

// ScanFiles walks the root directory and returns a list of IndexedDocument
// for all text files that pass the size and exclude filters.
func ScanFiles(rootDir string, cfg *config.Config) ([]IndexedDocument, error) {
	var docs []IndexedDocument

	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}

		if d.IsDir() {
			// Compute relative path for directory-level exclude check
			rel, relErr := filepath.Rel(rootDir, path)
			if relErr == nil && ShouldExclude(rel, cfg.Bm25Excludes) {
				return filepath.SkipDir
			}
			return nil
		}

		// Compute relative path for exclude check
		rel, relErr := filepath.Rel(rootDir, path)
		if relErr != nil {
			return nil
		}
		if ShouldExclude(rel, cfg.Bm25Excludes) {
			return nil
		}

		// Size check
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.Size() > cfg.Bm25MaxIndexFilesize {
			return nil
		}
		if info.Size() == 0 {
			return nil
		}

		// Binary check
		if !IsTextFile(path) {
			return nil
		}

		// Read content
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		// Build viking:// URI
		vikingRel, relErr2 := filepath.Rel(cfg.OpenVikingPrefix, path)
		if relErr2 != nil || strings.HasPrefix(vikingRel, "..") {
			return nil
		}

		docs = append(docs, IndexedDocument{
			URI:      vikingScheme + vikingRel,
			Path:     path,
			Content:  string(data),
			ModTime:  info.ModTime().UnixNano(),
			FileSize: info.Size(),
		})

		return nil
	})

	return docs, err
}
