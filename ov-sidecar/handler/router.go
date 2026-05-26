package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/yichozy/openvk-container/ov-sidecar/cache"
	"github.com/yichozy/openvk-container/ov-sidecar/config"
	"github.com/yichozy/openvk-container/ov-sidecar/sync"
)

type SearchResponse struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data,omitempty"`
	Error  string      `json:"error,omitempty"`
}

type ReadBatchResponse struct {
	Status string         `json:"status"`
	Data   interface{}    `json:"data,omitempty"`
	Error  string         `json:"error,omitempty"`
}

type StatusResponse struct {
	Status      string          `json:"status"`
	SyncEnabled bool            `json:"sync_enabled"`
	Replicas    []string        `json:"replicas,omitempty"`
	SyncState   sync.SyncState  `json:"sync_state,omitempty"`
}

func SetupRoutes(r *gin.Engine, cfg *config.Config, c cache.Cache, syncer *sync.Syncer) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/status", func(c *gin.Context) {
		resp := StatusResponse{
			Status:      "ok",
			SyncEnabled: cfg.SyncEnabled,
			Replicas:    cfg.SyncDests,
			SyncState:   syncer.State(),
		}
		c.JSON(http.StatusOK, resp)
	})

	sem := make(chan struct{}, cfg.MaxConcurrency)
	search := r.Group("", timeoutMiddleware(cfg.Timeout))
	{
		search.POST("/grep", GrepHandler(cfg, c, sem))
		search.POST("/read_batch", ReadBatchHandler(cfg, c, sem))
	}

	if cfg.SyncEnabled {
		r.POST("/sync", func(c *gin.Context) {
			if !syncer.TryStart(c.Request.Context()) {
				c.JSON(http.StatusConflict, gin.H{"status": "sync already running"})
				return
			}
			c.JSON(http.StatusAccepted, gin.H{"status": "sync started"})
		})
	}
}

func timeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// GinLogger returns a gin middleware that logs HTTP requests using zap.
func GinLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		zap.L().Info("http",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
		)
	}
}
