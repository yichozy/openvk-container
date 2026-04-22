package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/yichozy/hopebox/env"
	"github.com/yichozy/hopebox/log"
)

func main() {
	if os.Getenv("ENV") != "prod" {
		env.LoadEnvVariable()
	}

	cfg, err := Load()
	if err != nil {
		log.Fatalf(context.Background(), "failed to load config: %v", err)
	}

	log.Infow(context.Background(), "grep service starting",
		"port", cfg.Port,
		"data_dir", cfg.DataDir,
		"viking_prefix", cfg.OpenVikingPrefix,
	)

	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = log.NewZapWriter()
	r := gin.New()
	r.Use(gin.Recovery())

	SetupRoutes(r, cfg)

	go func() {
		if err := r.Run(":" + cfg.Port); err != nil {
			log.Fatalf(context.Background(), "server failed: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Info(context.Background(), "shutting down")
	_ = log.Sync()
}
