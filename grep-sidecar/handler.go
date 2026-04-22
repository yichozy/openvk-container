package main

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/gin-gonic/gin"
)

type SearchResponse struct {
	Status string      `json:"status"`
	Data   *SearchData `json:"data,omitempty"`
	Error  string      `json:"error,omitempty"`
}

func timeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func SetupRoutes(r *gin.Engine, cfg *Config) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	search := r.Group("", timeoutMiddleware(cfg.Timeout))
	{
		search.POST("/grep", searchHandler(cfg))
	}
}

func searchHandler(cfg *Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req SearchRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			zap.L().Warn("invalid search request", zap.Error(err))
			c.JSON(http.StatusBadRequest, SearchResponse{
				Status: "error",
				Error:  "invalid request: " + err.Error(),
			})
			return
		}

		zap.L().Info("search request",
			zap.String("pattern", req.Pattern),
			zap.String("directory", req.Directory),
			zap.String("glob", req.Glob),
		)

		result, err := Search(c.Request.Context(), cfg, &req)
		if err != nil {
			if errors.Is(err, ErrPathTraversal) {
				zap.L().Warn("path traversal denied",
					zap.String("pattern", req.Pattern),
					zap.String("directory", req.Directory),
				)
				c.JSON(http.StatusForbidden, SearchResponse{
					Status: "error",
					Error:  "path traversal denied",
				})
				return
			}
			if strings.Contains(err.Error(), "invalid regex") {
				zap.L().Warn("invalid regex", zap.Error(err))
				c.JSON(http.StatusBadRequest, SearchResponse{
					Status: "error",
					Error:  err.Error(),
				})
				return
			}
			if errors.Is(err, context.DeadlineExceeded) {
				zap.L().Warn("search timed out",
					zap.String("pattern", req.Pattern),
					zap.String("directory", req.Directory),
				)
				c.JSON(http.StatusGatewayTimeout, SearchResponse{
					Status: "error",
					Error:  "search timed out",
				})
				return
			}
			zap.L().Error("search failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, SearchResponse{
				Status: "error",
				Error:  err.Error(),
			})
			return
		}

		zap.L().Info("search completed",
			zap.String("pattern", req.Pattern),
			zap.Int("result_count", len(result.URIs)),
			zap.Bool("truncated", result.Truncated),
		)

		c.JSON(http.StatusOK, SearchResponse{
			Status: "success",
			Data:   result,
		})
	}
}
