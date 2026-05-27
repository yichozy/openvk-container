package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache provides Get/Set/Close operations backed by Redis.
type Cache interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Close() error
}

type redisCache struct {
	client *redis.Client
}

func (c *redisCache) Get(ctx context.Context, key string) (string, bool, error) {
	v, err := c.client.Get(ctx, key).Result()
	if err == nil {
		return v, true, nil
	}
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	return "", false, err
}

func (c *redisCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

func (c *redisCache) Close() error {
	return c.client.Close()
}

// NewCache creates a Cache backed by Redis. Returns error if Redis is not configured.
func NewCache(redisAddr, redisPassword string, redisDB int) (*redisCache, error) {
	if redisAddr == "" {
		return nil, errors.New("REDIS_ADDR is required")
	}
	client := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	})
	return &redisCache{client: client}, nil
}

// BuildGrepCacheKey generates a stable Redis key for a grep request.
func BuildGrepCacheKey(prefix string, payload GrepCacheKeyPayload) (string, error) {
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

// BuildReadCacheKey generates a stable Redis key for a file URI.
func BuildReadCacheKey(uri string) string {
	sum := sha256.Sum256([]byte(uri))
	return "read:" + hex.EncodeToString(sum[:])
}

// NormalizeDirectories deduplicates and sorts a list of directory paths.
func NormalizeDirectories(dirs []string) []string {
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

// GrepCacheKeyPayload holds the fields used to build a deterministic grep cache key.
type GrepCacheKeyPayload struct {
	Pattern          string   `json:"pattern"`
	Directories      []string `json:"directories"`
	Glob             string   `json:"glob"`
	Hidden           bool     `json:"hidden"`
	EffectiveMax     int      `json:"effective_max_results"`
	CfgMaxFilesize   string   `json:"cfg_max_filesize"`
	OpenVikingPrefix string   `json:"openviking_prefix"`
}
