package main

import (
	"context"
	"net/http"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/gin-gonic/gin"

	"github.com/yichozy/openvk-container/ov-sidecar/cache"
	"github.com/yichozy/openvk-container/ov-sidecar/config"
	"github.com/yichozy/openvk-container/ov-sidecar/handler"
	"github.com/yichozy/openvk-container/ov-sidecar/openviking"
	syncpkg "github.com/yichozy/openvk-container/ov-sidecar/sync"
)

func main() {
	loggerCfg := zap.NewProductionConfig()
	loggerCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := loggerCfg.Build()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()
	zap.ReplaceGlobals(logger)

	cfg, err := config.Load()
	if err != nil {
		zap.L().Fatal("failed to load config", zap.Error(err))
	}

	// Initialize cache (fails if Redis not configured).
	c, cacheErr := cache.NewCache(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if cacheErr != nil {
		zap.L().Warn("cache unavailable, skipping", zap.Error(cacheErr))
	}
	if c != nil {
		defer c.Close()
	}

	// Root context: cancelled by SIGINT/SIGTERM to stop all goroutines.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	syncer := syncpkg.NewSyncer(cfg)

	// Start background services.
	var wg sync.WaitGroup

	if cfg.RsyncDaemonEnabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			daemon := syncpkg.NewDaemon(cfg.RsyncDaemonPort, cfg.RsyncDaemonConfigPath)
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

	// Initialize BM25 indexer.
	indexer, err := openviking.NewIndexer(ctx, cfg)
	if err != nil {
		zap.L().Fatal("failed to initialize BM25 indexer", zap.Error(err))
	}
	defer indexer.Close()

	// Only build/update index on primary. Replica receives index via rsync.
	if !cfg.RsyncDaemonEnabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := indexer.UpdateIndex(ctx); err != nil {
				if ctx.Err() != nil {
					return
				}
				zap.L().Error("BM25 initial index update failed", zap.Error(err))
			}
			indexer.Start(ctx)
		}()
	} else {
		zap.L().Info("bm25: rsync daemon mode, skipping index build")
	}

	// HTTP server (always).
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(handler.GinLogger())
	handler.SetupRoutes(r, cfg, c, syncer, indexer)

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
