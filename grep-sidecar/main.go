package main

import (
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/gin-gonic/gin"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()
	zap.ReplaceGlobals(logger)

	cfg, err := Load()
	if err != nil {
		zap.L().Fatal("failed to load config", zap.Error(err))
	}

	zap.L().Info("grep service starting",
		zap.String("port", cfg.Port),
		zap.String("viking_prefix", cfg.OpenVikingPrefix),
	)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(ginLogger())

	SetupRoutes(r, cfg)

	go func() {
		if err := r.Run(":" + cfg.Port); err != nil {
			zap.L().Fatal("server failed", zap.Error(err))
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	zap.L().Info("shutting down")
}

func ginLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		zap.L().Info("http",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
		)
	}
}
