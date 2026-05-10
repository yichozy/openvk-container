package main

import (
	"context"
	"net/http"
	"os/signal"
	"sync"
	"syscall"
	"time"

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

	// Root context: cancelled by SIGINT/SIGTERM to stop all goroutines.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	syncer := NewSyncer(cfg)

	// Start background services.
	var wg sync.WaitGroup

	if cfg.RsyncDaemonEnabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			daemon := NewDaemon(cfg.RsyncDaemonPort, cfg.RsyncDaemonConfigPath)
			errCh := make(chan error, 1)
			go func() { errCh <- daemon.Start() }()
			select {
			case <-ctx.Done():
				daemon.Stop()
			case err := <-errCh:
				if err != nil {
					zap.L().Error("rsync daemon exited", zap.Error(err))
				}
			}
		}()
	}

	if cfg.SyncEnabled {
		zap.L().Info("sync enabled",
			zap.String("source", cfg.SyncSource),
			zap.Strings("dests", cfg.SyncDests),
			zap.Duration("interval", cfg.SyncInterval),
		)
		wg.Add(1)
		go func() {
			defer wg.Done()
			syncer.TryStart(ctx)
			ticker := time.NewTicker(cfg.SyncInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					syncer.TryStart(ctx)
				}
			}
		}()
	}

	// HTTP server (always).
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(ginLogger())
	SetupRoutes(r, cfg, syncer)

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: r}

	wg.Add(1)
	go func() {
		defer wg.Done()
		zap.L().Info("sidecar starting",
			zap.String("port", cfg.Port),
			zap.Bool("sync_enabled", cfg.SyncEnabled),
			zap.Bool("rsync_daemon", cfg.RsyncDaemonEnabled),
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zap.L().Error("server failed", zap.Error(err))
		}
	}()

	<-ctx.Done()
	zap.L().Info("shutting down")

	shutCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	srv.Shutdown(shutCtx)

	wg.Wait()
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
