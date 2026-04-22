package main

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yichozy/hopebox/log"
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
			log.Warnw(c.Request.Context(), "invalid search request", "error", err.Error())
			c.JSON(http.StatusBadRequest, SearchResponse{
				Status: "error",
				Error:  "invalid request: " + err.Error(),
			})
			return
		}

		log.Infow(c.Request.Context(), "search request",
			"pattern", req.Pattern,
			"directory", req.Directory,
			"glob", req.Glob,
		)

		result, err := Search(c.Request.Context(), cfg, &req)
		if err != nil {
			if errors.Is(err, ErrPathTraversal) {
				log.Warnw(c.Request.Context(), "path traversal denied",
					"pattern", req.Pattern,
					"directory", req.Directory,
				)
				c.JSON(http.StatusForbidden, SearchResponse{
					Status: "error",
					Error:  "path traversal denied",
				})
				return
			}
			if strings.Contains(err.Error(), "invalid regex") {
				log.Warnw(c.Request.Context(), "invalid regex", "error", err.Error())
				c.JSON(http.StatusBadRequest, SearchResponse{
					Status: "error",
					Error:  err.Error(),
				})
				return
			}
			if errors.Is(err, context.DeadlineExceeded) {
				log.Warnw(c.Request.Context(), "search timed out",
					"pattern", req.Pattern,
					"directory", req.Directory,
				)
				c.JSON(http.StatusGatewayTimeout, SearchResponse{
					Status: "error",
					Error:  "search timed out",
				})
				return
			}
			log.Errorw(c.Request.Context(), "search failed", "error", err.Error())
			c.JSON(http.StatusInternalServerError, SearchResponse{
				Status: "error",
				Error:  err.Error(),
			})
			return
		}

		log.Infow(c.Request.Context(), "search completed",
			"pattern", req.Pattern,
			"result_count", len(result.Files),
			"truncated", result.Truncated,
		)

		c.JSON(http.StatusOK, SearchResponse{
			Status: "success",
			Data:   result,
		})
	}
}
