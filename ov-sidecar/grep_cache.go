package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type GrepCache interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Close() error
}

var grepCache GrepCache = &noopGrepCache{}

type noopGrepCache struct{}

func (c *noopGrepCache) Get(ctx context.Context, key string) (string, bool, error) { return "", false, nil }
func (c *noopGrepCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return nil
}
func (c *noopGrepCache) Close() error { return nil }

type redisGrepCache struct {
	client *redis.Client
}

func (c *redisGrepCache) Get(ctx context.Context, key string) (string, bool, error) {
	v, err := c.client.Get(ctx, key).Result()
	if err == nil {
		return v, true, nil
	}
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	return "", false, err
}

func (c *redisGrepCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

func (c *redisGrepCache) Close() error {
	return c.client.Close()
}

func newGrepCacheFromConfig(cfg *Config) GrepCache {
	if cfg == nil || cfg.RedisAddr == "" {
		return &noopGrepCache{}
	}
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	return &redisGrepCache{client: client}
}

func InitGrepCache(cfg *Config) {
	grepCache = newGrepCacheFromConfig(cfg)
}

type grepCacheKeyPayload struct {
	Pattern          string   `json:"pattern"`
	Directories      []string `json:"directories"`
	Glob             string   `json:"glob"`
	Hidden           bool     `json:"hidden"`
	EffectiveMax     int      `json:"effective_max_results"`
	CfgMaxFilesize   string   `json:"cfg_max_filesize"`
	OpenVikingPrefix string   `json:"openviking_prefix"`
}

func buildGrepCacheKey(prefix string, payload grepCacheKeyPayload) (string, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	if prefix == "" {
		prefix = "grep:"
	}
	return prefix + "v1:" + hex.EncodeToString(sum[:]), nil
}

func normalizeDirectories(dirs []string) []string {
	uniq := make(map[string]struct{}, len(dirs))
	for _, d := range dirs {
		uniq[d] = struct{}{}
	}
	out := make([]string, 0, len(uniq))
	for d := range uniq {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

func normalizeGlob(glob string) string {
	return strings.TrimSpace(glob)
}
