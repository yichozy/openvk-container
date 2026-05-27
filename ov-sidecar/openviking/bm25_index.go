package openviking

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"go.uber.org/zap"

	"github.com/yichozy/openvk-container/ov-sidecar/config"
)

// Indexer manages a Bleve full-text index for BM25 search.
type Indexer struct {
	index bleve.Index
	cfg   *config.Config

	mu    sync.Mutex
	state IndexerState
}

// IndexerState exposes the current state of the indexer for monitoring.
type IndexerState struct {
	DocCount    uint64 `json:"doc_count"`
	LastUpdated string `json:"last_updated,omitempty"`
	Updating    bool   `json:"updating"`
	Error       string `json:"error,omitempty"`
}

// NewIndexer creates or opens a Bleve index at the configured path.
func NewIndexer(ctx context.Context, cfg *config.Config) (*Indexer, error) {
	imap := buildIndexMapping()

	var index bleve.Index
	if _, err := os.Stat(cfg.Bm25IndexPath); os.IsNotExist(err) {
		zap.L().Info("bm25: creating new index", zap.String("path", cfg.Bm25IndexPath))
		index, err = bleve.New(cfg.Bm25IndexPath, imap)
		if err != nil {
			return nil, err
		}
	} else {
		zap.L().Info("bm25: opening existing index", zap.String("path", cfg.Bm25IndexPath))
		index, err = bleve.Open(cfg.Bm25IndexPath)
		if err != nil {
			return nil, err
		}
	}

	count, err := index.DocCount()
	if err != nil {
		zap.L().Warn("bm25: failed to get doc count", zap.Error(err))
		count = 0
	}

	zap.L().Info("bm25: index opened", zap.Uint64("doc_count", count))

	return &Indexer{
		index: index,
		cfg:   cfg,
		state: IndexerState{DocCount: count},
	}, nil
}

func buildIndexMapping() mapping.IndexMapping {
	imap := bleve.NewIndexMapping()
	imap.DefaultAnalyzer = "standard"
	imap.ScoringModel = "bm25"

	docMapping := bleve.NewDocumentMapping()

	// content — TextField with standard analyzer for BM25 search and highlighting
	// Store=false: don't persist full content in index to reduce index size
	contentMapping := bleve.NewTextFieldMapping()
	contentMapping.Analyzer = "standard"
	contentMapping.Store = true
	contentMapping.IncludeTermVectors = true
	docMapping.AddFieldMappingsAt("content", contentMapping)

	// uri — KeywordField (exact match, not analyzed)
	uriMapping := bleve.NewKeywordFieldMapping()
	uriMapping.Store = true
	docMapping.AddFieldMappingsAt("uri", uriMapping)

	// path — TextField for path-based search
	pathMapping := bleve.NewTextFieldMapping()
	pathMapping.Analyzer = "standard"
	pathMapping.Store = true
	docMapping.AddFieldMappingsAt("path", pathMapping)

	// mod_time — NumericField for incremental update queries
	modTimeMapping := bleve.NewNumericFieldMapping()
	modTimeMapping.Store = true
	docMapping.AddFieldMappingsAt("mod_time", modTimeMapping)

	imap.AddDocumentMapping("doc", docMapping)
	imap.DefaultMapping = docMapping

	return imap
}

// BuildIndex performs a full scan and indexes all matching files.
func (idx *Indexer) BuildIndex(ctx context.Context) error {
	idx.mu.Lock()
	idx.state.Updating = true
	idx.mu.Unlock()

	defer func() {
		idx.mu.Lock()
		idx.state.Updating = false
		idx.mu.Unlock()
	}()

	start := time.Now()
	zap.L().Info("bm25: starting full index build")

	docs, err := ScanFiles(idx.cfg.OpenVikingPrefix, idx.cfg)
	if err != nil {
		idx.mu.Lock()
		idx.state.Error = err.Error()
		idx.mu.Unlock()
		return err
	}

	if err := idx.batchIndex(ctx, docs); err != nil {
		idx.mu.Lock()
		idx.state.Error = err.Error()
		idx.mu.Unlock()
		return err
	}

	count, _ := idx.index.DocCount()
	zap.L().Info("bm25: full index build completed",
		zap.Int("docs_indexed", len(docs)),
		zap.Uint64("total_docs", count),
		zap.Duration("duration", time.Since(start)),
	)

	idx.mu.Lock()
	idx.state.DocCount = count
	idx.state.LastUpdated = time.Now().Format(time.RFC3339)
	idx.state.Error = ""
	idx.mu.Unlock()

	return nil
}

// UpdateIndex performs incremental index update: adds changed files, removes deleted files.
func (idx *Indexer) UpdateIndex(ctx context.Context) error {
	idx.mu.Lock()
	idx.state.Updating = true
	idx.mu.Unlock()

	defer func() {
		idx.mu.Lock()
		idx.state.Updating = false
		idx.mu.Unlock()
	}()

	start := time.Now()

	// Scan current files on disk
	docs, err := ScanFiles(idx.cfg.OpenVikingPrefix, idx.cfg)
	if err != nil {
		idx.mu.Lock()
		idx.state.Error = err.Error()
		idx.mu.Unlock()
		return err
	}

	// Build set of current URIs for change detection and deletion detection
	currentPaths := make(map[string]bool, len(docs))
	for _, doc := range docs {
		currentPaths[doc.URI] = true
	}

	// Enumerate all indexed URIs and their mod_times via search
	indexedURIs := make(map[string]int64)
	count, _ := idx.index.DocCount()
	allReq := bleve.NewSearchRequest(bleve.NewMatchAllQuery())
	allReq.Fields = []string{"uri", "mod_time"}
	allReq.Size = int(count) + 1000

	searchResult, err := idx.index.Search(allReq)
	if err != nil {
		zap.L().Warn("bm25: failed to enumerate indexed docs", zap.Error(err))
	} else {
		for _, hit := range searchResult.Hits {
			uri, _ := hit.Fields["uri"].(string)
			modTime, _ := hit.Fields["mod_time"].(float64)
			if uri != "" {
				indexedURIs[uri] = int64(modTime)
			}
		}
	}

	// Index new/changed documents
	var toIndex []IndexedDocument
	for _, doc := range docs {
		if indexedModTime, ok := indexedURIs[doc.URI]; !ok || indexedModTime != doc.ModTime {
			toIndex = append(toIndex, doc)
		}
	}

	if len(toIndex) > 0 {
		if err := idx.batchIndex(ctx, toIndex); err != nil {
			idx.mu.Lock()
			idx.state.Error = err.Error()
			idx.mu.Unlock()
			return err
		}
	}

	// Delete stale documents (indexed but no longer on disk)
	batch := idx.index.NewBatch()
	deleted := 0
	for uri := range indexedURIs {
		if !currentPaths[uri] {
			batch.Delete(uri)
			deleted++
		}
	}
	if deleted > 0 {
		if err := idx.index.Batch(batch); err != nil {
			zap.L().Warn("bm25: failed to delete stale docs", zap.Error(err))
		}
	}

	count, _ = idx.index.DocCount()
	zap.L().Info("bm25: incremental update completed",
		zap.Int("updated", len(toIndex)),
		zap.Int("deleted", deleted),
		zap.Uint64("total_docs", count),
		zap.Duration("duration", time.Since(start)),
	)

	idx.mu.Lock()
	idx.state.DocCount = count
	idx.state.LastUpdated = time.Now().Format(time.RFC3339)
	idx.state.Error = ""
	idx.mu.Unlock()

	return nil
}

// batchIndex indexes a batch of documents using Bleve's batch API.
func (idx *Indexer) batchIndex(ctx context.Context, docs []IndexedDocument) error {
	batchSize := idx.cfg.Bm25BatchSize
	for i := 0; i < len(docs); i += batchSize {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		end := i + batchSize
		if end > len(docs) {
			end = len(docs)
		}

		batch := idx.index.NewBatch()
		for _, doc := range docs[i:end] {
			batch.Index(doc.URI, doc)
		}
		if err := idx.index.Batch(batch); err != nil {
			return err
		}
	}
	return nil
}

// Start runs the incremental update loop in the background.
// Stops when the context is cancelled.
func (idx *Indexer) Start(ctx context.Context) {
	interval := idx.cfg.Bm25UpdateInterval
	zap.L().Info("bm25: starting incremental update loop", zap.Duration("interval", interval))

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			zap.L().Info("bm25: stopping incremental update loop")
			return
		case <-ticker.C:
			if err := idx.UpdateIndex(ctx); err != nil {
				if ctx.Err() != nil {
					return
				}
				zap.L().Warn("bm25: incremental update failed", zap.Error(err))
			}
		}
	}
}

// State returns the current indexer state for monitoring.
func (idx *Indexer) State() IndexerState {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return idx.state
}

// Cfg returns the config used by this indexer.
func (idx *Indexer) Cfg() *config.Config {
	return idx.cfg
}

// Search performs a BM25 search on the index.
func (idx *Indexer) Search(ctx context.Context, req *Bm25SearchRequest, searchDirs []string) (*Bm25SearchData, error) {
	// Build prefix set for scope filtering
	dirPrefixes := make(map[string]bool, len(searchDirs))
	for _, dir := range searchDirs {
		if !strings.HasSuffix(dir, string(filepath.Separator)) {
			dir += string(filepath.Separator)
		}
		dirPrefixes[dir] = true
	}

	// Build level set for filtering (default: all levels)
	levels := req.Level
	if len(levels) == 0 {
		levels = []int{0, 1, 2}
	}
	levelSet := make(map[int]bool, len(levels))
	for _, l := range levels {
		levelSet[l] = true
	}

	// Use MatchQuery instead of QueryStringQuery to avoid query injection.
	// MatchQuery tokenizes the input with the same analyzer and OR's the terms.
	query := bleve.NewMatchQuery(req.Pattern)
	query.SetField("content")

	searchReq := bleve.NewSearchRequest(query)
	searchReq.Fields = []string{"uri", "path"}
	if req.Highlight != nil && *req.Highlight {
		searchReq.Highlight = bleve.NewHighlight()
		searchReq.Highlight.AddField("content")
	}
	// Fetch more candidates than needed to account for post-filtering
	// (level, glob, directory scope). Use total docs as upper bound.
	fetchSize := req.MaxResults * 10
	totalDocs, _ := idx.index.DocCount()
	if fetchSize > int(totalDocs)+1000 {
		fetchSize = int(totalDocs) + 1000
	}
	searchReq.Size = fetchSize

	result, err := idx.index.SearchInContext(ctx, searchReq)
	if err != nil {
		return nil, err
	}

	results := make([]Bm25Result, 0)
	for _, hit := range result.Hits {
		uri, _ := hit.Fields["uri"].(string)
		path, _ := hit.Fields["path"].(string)

		// Filter by directory scope
		if len(dirPrefixes) > 0 {
			inScope := false
			for prefix := range dirPrefixes {
				if strings.HasPrefix(path, prefix) {
					inScope = true
					break
				}
			}
			if !inScope {
				continue
			}
		}

		// Apply glob filter
		if req.Glob != "" {
			if matched, _ := filepath.Match(req.Glob, filepath.Base(path)); !matched {
				continue
			}
		}

		// Filter by content level
		docLevel := DocLevel(path)
		if !levelSet[docLevel] {
			continue
		}

		var fragments []string
		if contentFragments, ok := hit.Fragments["content"]; ok {
			fragments = contentFragments
		}

		results = append(results, Bm25Result{
			URI:       uri,
			Score:     hit.Score,
			Fragments: fragments,
			Level:     docLevel,
		})
	}

	truncated := len(results) > req.MaxResults
	if truncated {
		results = results[:req.MaxResults]
	}

	return &Bm25SearchData{
		Results:   results,
		Total:     len(results),
		Truncated: truncated,
	}, nil
}

// Close closes the underlying Bleve index. Safe to call multiple times.
func (idx *Indexer) Close() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if idx.index == nil {
		return nil
	}
	err := idx.index.Close()
	idx.index = nil
	return err
}
