package main

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sync/singleflight"
)

type errorCache struct{}

func (c *errorCache) Get(ctx context.Context, key string) (string, bool, error) {
	return "", false, errors.New("boom")
}

func (c *errorCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return errors.New("boom")
}

func (c *errorCache) Close() error { return nil }

type mapCache struct {
	mu   sync.Mutex
	data map[string]string
}

func (c *mapCache) Get(ctx context.Context, key string) (string, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.data[key]
	return v, ok, nil
}

func (c *mapCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = value
	return nil
}

func (c *mapCache) Close() error { return nil }

func TestSearchCache_HitSkipsRipgrep(t *testing.T) {
	cfg := testConfig(t)
	cfg.RedisAddr = "enabled"
	cfg.GrepCacheTTL = 120 * time.Second

	cache := &mapCache{data: make(map[string]string)}

	prevCache := grepCache
	t.Cleanup(func() {
		grepCache = prevCache
		grepSF = singleflight.Group{}
	})
	grepCache = cache
	grepSF = singleflight.Group{}

	ctx := context.Background()

	dirs1, err := resolveDirectory(cfg, "viking://resources/curation/TNBC")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	key, err := buildGrepCacheKey(cfg.GrepCachePrefix, grepCacheKeyPayload{
		Pattern:          "[invalid",
		Directories:      normalizeDirectories([]string{dirs1}),
		Glob:             "",
		Hidden:           false,
		EffectiveMax:     cfg.MaxGrepResults,
		CfgMaxFilesize:   cfg.MaxGrepFilesize,
		OpenVikingPrefix: cfg.OpenVikingPrefix,
	})
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	if err := cache.Set(ctx, key, `{"uris":["viking://a"],"truncated":false}`, cfg.GrepCacheTTL); err != nil {
		t.Fatalf("cache set: %v", err)
	}

	res, err := Search(ctx, cfg, &SearchRequest{Pattern: "[invalid", Directories: []string{"viking://resources/curation/TNBC"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.URIs) != 1 || res.URIs[0] != "viking://a" {
		t.Fatalf("unexpected uris: %+v", res.URIs)
	}
}

type blockingSetCache struct {
	started chan struct{}
	release chan struct{}
	calls   int32
}

func (c *blockingSetCache) Get(ctx context.Context, key string) (string, bool, error) {
	return "", false, nil
}

func (c *blockingSetCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	if atomic.AddInt32(&c.calls, 1) == 1 {
		close(c.started)
	}
	<-c.release
	return nil
}

func (c *blockingSetCache) Close() error { return nil }

func TestSearchCache_SingleflightDedup(t *testing.T) {
	cfg := testConfig(t)
	cfg.RedisAddr = "enabled"
	cfg.GrepCacheTTL = 120 * time.Second

	cache := &blockingSetCache{started: make(chan struct{}), release: make(chan struct{})}

	prevCache := grepCache
	t.Cleanup(func() {
		grepCache = prevCache
		grepSF = singleflight.Group{}
	})
	grepCache = cache
	grepSF = singleflight.Group{}

	ctx := context.Background()
	var wg sync.WaitGroup
	errCh := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := Search(ctx, cfg, &SearchRequest{Pattern: "PFS", Directories: []string{"viking://resources/curation/TNBC"}})
			errCh <- err
		}()
	}

	<-cache.started
	close(cache.release)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
	}
	if got := atomic.LoadInt32(&cache.calls); got != 1 {
		t.Fatalf("expected cache set calls=1, got %d", got)
	}
}

func TestSearchCache_ErrorFallsBack(t *testing.T) {
	cfg := testConfig(t)
	cfg.RedisAddr = "enabled"
	cfg.GrepCacheTTL = 120 * time.Second

	prevCache := grepCache
	t.Cleanup(func() {
		grepCache = prevCache
		grepSF = singleflight.Group{}
	})
	grepCache = &errorCache{}
	grepSF = singleflight.Group{}

	ctx := context.Background()
	res, err := Search(ctx, cfg, &SearchRequest{Pattern: "PFS", Directories: []string{"viking://resources/curation/TNBC"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.URIs) == 0 {
		t.Fatal("expected results")
	}
}
