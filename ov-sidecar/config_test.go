package main

import "testing"

func TestLoad_CacheEnvAliases(t *testing.T) {
	t.Setenv("OPEN_VIKING_DATA_PATH", "/tmp/viking")
	t.Setenv("OPEN_VIKING_ACCOUNT", "default")

	t.Setenv("CACHE_REDIS_HOST", "redis:6379")
	t.Setenv("CACHE_REDIS_PASSWORD", "secret")
	t.Setenv("CACHE_REDIS_DB", "3")
	t.Setenv("CACHE_GREP_PREFIX", "cachegrep:")
	t.Setenv("CACHE_GREP_TTL", "90s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RedisAddr != "redis:6379" {
		t.Fatalf("expected RedisAddr=redis:6379, got %q", cfg.RedisAddr)
	}
	if cfg.RedisPassword != "secret" {
		t.Fatalf("expected RedisPassword=secret, got %q", cfg.RedisPassword)
	}
	if cfg.RedisDB != 3 {
		t.Fatalf("expected RedisDB=3, got %d", cfg.RedisDB)
	}
	if cfg.GrepCachePrefix != "cachegrep:" {
		t.Fatalf("expected GrepCachePrefix=cachegrep:, got %q", cfg.GrepCachePrefix)
	}
	if cfg.GrepCacheTTL.String() != "1m30s" {
		t.Fatalf("expected GrepCacheTTL=90s, got %s", cfg.GrepCacheTTL)
	}
}

