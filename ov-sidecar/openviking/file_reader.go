package openviking

import (
	"context"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/yichozy/openvk-container/ov-sidecar/cache"
	"github.com/yichozy/openvk-container/ov-sidecar/config"
)

type ReadBatchRequest struct {
	URIs []string `json:"uris" binding:"required,min=1"`
}

type ReadBatchData struct {
	Results map[string]*ReadResult `json:"results"`
}

// ReadResult holds the content of a single file read.
// For text files, Content is the file content and Encoding is "utf-8".
// For binary files (images etc.), Content is base64-encoded and Encoding is "base64".
type ReadResult struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

var binaryExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
	".bmp": true, ".webp": true, ".svg": true, ".ico": true,
	".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
	".ppt": true, ".pptx": true, ".zip": true, ".tar": true, ".gz": true,
	".mp3": true, ".mp4": true, ".wav": true, ".avi": true, ".mov": true,
}

func isBinaryFile(uri string) bool {
	ext := strings.ToLower(filepath.Ext(uri))
	return binaryExtensions[ext]
}

// ReadBatch reads multiple files concurrently, returning a map of URI -> ReadResult.
// Missing or unreadable files yield a nil value.
func ReadBatch(ctx context.Context, cfg *config.Config, c cache.Cache, uris []string) (map[string]*ReadResult, error) {
	results := make(map[string]*ReadResult, len(uris))
	var mu sync.Mutex

	g, gCtx := errgroup.WithContext(ctx)
	for _, uri := range uris {
		uri := uri
		g.Go(func() error {
			content := readOne(gCtx, cfg, c, uri)
			mu.Lock()
			results[uri] = content
			mu.Unlock()
			return nil // never return error — individual failures are nil values
		})
	}
	_ = g.Wait()
	return results, nil
}

// readOne reads a single file. Returns nil if the file does not exist,
// exceeds MaxReadFilesize, or any I/O error occurs.
func readOne(ctx context.Context, cfg *config.Config, c cache.Cache, uri string) *ReadResult {
	filePath, err := ResolveURI(cfg, uri)
	if err != nil {
		zap.L().Warn("read: path traversal", zap.String("uri", uri))
		return nil
	}

	binary := isBinaryFile(uri)

	// Check cache first (text files only — binary images are too large for Redis).
	cacheEnabled := c != nil && cfg.ReadCacheTTL > 0 && !binary
	if cacheEnabled {
		cacheKey := cache.BuildReadCacheKey(uri)
		if cached, ok, getErr := c.Get(ctx, cacheKey); getErr == nil && ok {
			return &ReadResult{Content: cached, Encoding: "utf-8"}
		} else if getErr != nil {
			zap.L().Warn("read cache get failed", zap.Error(getErr))
		}
	}

	// Stat the file.
	info, err := os.Stat(filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			zap.L().Warn("read: stat failed", zap.String("uri", uri), zap.Error(err))
		}
		return nil
	}
	if info.IsDir() {
		return nil
	}
	if info.Size() > cfg.MaxReadFilesize {
		zap.L().Warn("read: file exceeds max size",
			zap.String("uri", uri),
			zap.Int64("size", info.Size()),
			zap.Int64("max", cfg.MaxReadFilesize),
		)
		return nil
	}

	// Open and read.
	f, err := os.Open(filePath)
	if err != nil {
		zap.L().Warn("read: open failed", zap.String("uri", uri), zap.Error(err))
		return nil
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		zap.L().Warn("read: read failed", zap.String("uri", uri), zap.Error(err))
		return nil
	}

	if binary {
		encoded := base64.StdEncoding.EncodeToString(data)
		return &ReadResult{Content: encoded, Encoding: "base64"}
	}

	content := string(data)

	if cacheEnabled {
		cacheKey := cache.BuildReadCacheKey(uri)
		if setErr := c.Set(ctx, cacheKey, content, cfg.ReadCacheTTL); setErr != nil {
			zap.L().Warn("read cache set failed", zap.Error(setErr))
		}
	}

	return &ReadResult{Content: content, Encoding: "utf-8"}
}
