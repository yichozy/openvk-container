package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/yichozy/openvk-container/ov-sidecar/openviking"
)

// Bm25Handler returns a gin.HandlerFunc that handles POST /search/bm25 requests.
func Bm25Handler(indexer *openviking.Indexer) gin.HandlerFunc {
	return func(gc *gin.Context) {
		var req openviking.Bm25SearchRequest
		if err := gc.ShouldBindJSON(&req); err != nil {
			zap.L().Warn("invalid bm25 request", zap.Error(err))
			gc.JSON(http.StatusBadRequest, SearchResponse{
				Status: "error",
				Error:  "invalid request: " + err.Error(),
			})
			return
		}

		zap.L().Info("bm25 search request",
			zap.String("pattern", req.Pattern),
			zap.Strings("directories", req.Directories),
			zap.String("glob", req.Glob),
		)

		searchResult, err := openviking.Bm25Search(gc.Request.Context(), indexer.Cfg(), indexer, &req)
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
			if strings.Contains(err.Error(), "query") {
				zap.L().Warn("invalid query", zap.Error(err))
				gc.JSON(http.StatusBadRequest, SearchResponse{
					Status: "error",
					Error:  err.Error(),
				})
				return
			}
			if errors.Is(err, context.DeadlineExceeded) {
				zap.L().Warn("bm25 search timed out",
					zap.String("pattern", req.Pattern),
					zap.Strings("directories", req.Directories),
				)
				gc.JSON(http.StatusGatewayTimeout, SearchResponse{
					Status: "error",
					Error:  "search timed out",
				})
				return
			}
			zap.L().Error("bm25 search failed", zap.Error(err))
			gc.JSON(http.StatusInternalServerError, SearchResponse{
				Status: "error",
				Error:  err.Error(),
			})
			return
		}

		zap.L().Info("bm25 search completed",
			zap.String("pattern", req.Pattern),
			zap.Int("result_count", len(searchResult.Results)),
			zap.Int("total", searchResult.Total),
			zap.Bool("truncated", searchResult.Truncated),
		)

		gc.JSON(http.StatusOK, SearchResponse{
			Status: "success",
			Data:   searchResult,
		})
	}
}
