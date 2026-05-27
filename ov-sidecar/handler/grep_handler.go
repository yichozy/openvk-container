package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/yichozy/openvk-container/ov-sidecar/cache"
	"github.com/yichozy/openvk-container/ov-sidecar/config"
	"github.com/yichozy/openvk-container/ov-sidecar/openviking"
)

func GrepHandler(cfg *config.Config, c cache.Cache) gin.HandlerFunc {
	return func(gc *gin.Context) {
		var req openviking.SearchRequest
		if err := gc.ShouldBindJSON(&req); err != nil {
			zap.L().Warn("invalid search request", zap.Error(err))
			gc.JSON(http.StatusBadRequest, SearchResponse{
				Status: "error",
				Error:  "invalid request: " + err.Error(),
			})
			return
		}

		zap.L().Info("search request",
			zap.String("pattern", req.Pattern),
			zap.Strings("directories", req.Directories),
			zap.String("glob", req.Glob),
		)

		result, err := openviking.Search(gc.Request.Context(), cfg, c, &req)
		if err != nil {
			if errors.Is(err, openviking.ErrPathTraversal) {
				zap.L().Warn("path traversal denied",
					zap.String("pattern", req.Pattern),
					zap.Strings("directories", req.Directories),
				)
				gc.JSON(http.StatusForbidden, SearchResponse{
					Status: "error",
					Error:  "path traversal denied",
				})
				return
			}
			if strings.Contains(err.Error(), "invalid regex") {
				zap.L().Warn("invalid regex", zap.Error(err))
				gc.JSON(http.StatusBadRequest, SearchResponse{
					Status: "error",
					Error:  err.Error(),
				})
				return
			}
			if errors.Is(err, context.DeadlineExceeded) {
				zap.L().Warn("search timed out",
					zap.String("pattern", req.Pattern),
					zap.Strings("directories", req.Directories),
				)
				gc.JSON(http.StatusGatewayTimeout, SearchResponse{
					Status: "error",
					Error:  "search timed out",
				})
				return
			}
			zap.L().Error("search failed", zap.Error(err))
			gc.JSON(http.StatusInternalServerError, SearchResponse{
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

		gc.JSON(http.StatusOK, SearchResponse{
			Status: "success",
			Data:   result,
		})
	}
}
