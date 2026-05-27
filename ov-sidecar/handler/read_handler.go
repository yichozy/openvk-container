package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/yichozy/openvk-container/ov-sidecar/cache"
	"github.com/yichozy/openvk-container/ov-sidecar/config"
	"github.com/yichozy/openvk-container/ov-sidecar/openviking"
)

func ReadBatchHandler(cfg *config.Config, c cache.Cache) gin.HandlerFunc {
	return func(gc *gin.Context) {
		var req openviking.ReadBatchRequest
		if err := gc.ShouldBindJSON(&req); err != nil {
			zap.L().Warn("invalid read_batch request", zap.Error(err))
			gc.JSON(http.StatusBadRequest, ReadBatchResponse{
				Status: "error",
				Error:  "invalid request: " + err.Error(),
			})
			return
		}

		if len(req.URIs) > cfg.MaxReadBatchSize {
			gc.JSON(http.StatusBadRequest, ReadBatchResponse{
				Status: "error",
				Error:  fmt.Sprintf("too many URIs: %d exceeds max %d", len(req.URIs), cfg.MaxReadBatchSize),
			})
			return
		}

		// Validate all URIs for path traversal before reading any files.
		if err := openviking.ValidateURIs(cfg, req.URIs); err != nil {
			zap.L().Warn("read_batch path traversal denied", zap.Strings("uris", req.URIs))
			gc.JSON(http.StatusForbidden, ReadBatchResponse{
				Status: "error",
				Error:  "path traversal denied",
			})
			return
		}

		results, err := openviking.ReadBatch(gc.Request.Context(), cfg, c, req.URIs)
		if err != nil {
			zap.L().Error("read_batch failed", zap.Error(err))
			gc.JSON(http.StatusInternalServerError, ReadBatchResponse{
				Status: "error",
				Error:  err.Error(),
			})
			return
		}

		gc.JSON(http.StatusOK, ReadBatchResponse{
			Status: "success",
			Data:   &openviking.ReadBatchData{Results: results},
		})
	}
}
