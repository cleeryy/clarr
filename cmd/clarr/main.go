package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cleeryy/clarr/internal/cleaner"
	"github.com/cleeryy/clarr/internal/config"
	"github.com/cleeryy/clarr/internal/qbittorrent"
	"github.com/cleeryy/clarr/internal/radarr"
	"github.com/cleeryy/clarr/internal/sonarr"
	"github.com/cleeryy/clarr/internal/webhook"
	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

var version = "dev"

func main() {
	// ─── Logger ───────────────────────────────────────────────────────
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	logger.Info("starting clarr", zap.String("version", version))

	// ─── Config ───────────────────────────────────────────────────────
	cfgPath := "config.yaml"
	if v := os.Getenv("CLARR_CONFIG_PATH"); v != "" {
		cfgPath = v
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	// ─── Clients ──────────────────────────────────────────────────────
	radarrClient := radarr.New(cfg.Radarr.URL, cfg.Radarr.APIKey)
	sonarrClient := sonarr.New(cfg.Sonarr.URL, cfg.Sonarr.APIKey)

	qbitClient, err := qbittorrent.New(
		cfg.Qbittorrent.URL,
		cfg.Qbittorrent.Username,
		cfg.Qbittorrent.Password,
	)
	if err != nil {
		logger.Fatal("failed to connect to qbittorrent", zap.Error(err))
	}
	_ = qbitClient // TODO: intégrer dans le cleaner (feat/cleaner-qbit-integration)

	// ─── Cleaner ──────────────────────────────────────────────────────
	cleanerSvc := cleaner.New(cfg.Cleaner.DownloadDir, cfg.Cleaner.DryRun, logger)

	// ─── Scheduler ────────────────────────────────────────────────────
	c := cron.New()
	_, err = c.AddFunc(cfg.Cleaner.Schedule, func() {
		logger.Info("scheduled cleanup starting")
		result, err := cleanerSvc.Cleanup()
		if err != nil {
			logger.Error("scheduled cleanup failed", zap.Error(err))
			return
		}
		logger.Info("scheduled cleanup done",
			zap.Int("orphans", len(result.OrphanFiles)),
			zap.String("freed", result.FreedBytesHuman()),
			zap.Int("errors", len(result.Errors)),
		)
	})
	if err != nil {
		logger.Fatal("invalid cron schedule", zap.Error(err))
	}
	c.Start()
	defer c.Stop()

	// ─── Router ───────────────────────────────────────────────────────
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(ginZapLogger(logger))

	// Health check
	r.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "clarr",
			"version": version,
		})
	})

	// Webhook Jellyfin
	webhookHandler := webhook.New(cfg.Jellyfin.WebhookSecret, radarrClient, sonarrClient, logger)
	webhookHandler.Register(r)

	// Cleanup manuel via API
	r.POST("/api/cleanup", func(ctx *gin.Context) {
		go func() {
			result, err := cleanerSvc.Cleanup()
			if err != nil {
				logger.Error("manual cleanup failed", zap.Error(err))
				return
			}
			logger.Info("manual cleanup done",
				zap.Int("orphans", len(result.OrphanFiles)),
				zap.String("freed", result.FreedBytesHuman()),
			)
		}()
		ctx.JSON(http.StatusAccepted, gin.H{"status": "cleanup started"})
	})

	// Rescan manuel Radarr + Sonarr
	r.POST("/api/rescan", func(ctx *gin.Context) {
		go func() {
			if err := radarrClient.RescanAll(); err != nil {
				logger.Error("radarr rescan failed", zap.Error(err))
			}
			if err := sonarrClient.RescanAll(); err != nil {
				logger.Error("sonarr rescan failed", zap.Error(err))
			}
			logger.Info("manual rescan done")
		}()
		ctx.JSON(http.StatusAccepted, gin.H{"status": "rescan started"})
	})

	// Stats disque
	r.GET("/api/stats", func(ctx *gin.Context) {
		orphans, err := cleanerSvc.FindOrphans()
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		var totalSize int64
		for _, o := range orphans {
			totalSize += o.Size
		}

		result := &cleaner.CleanupResult{
			OrphanFiles: orphans,
			FreedBytes:  totalSize,
		}

		ctx.JSON(http.StatusOK, gin.H{
			"orphan_count":    len(orphans),
			"orphan_size":     result.FreedBytesHuman(),
			"orphan_size_raw": totalSize,
			"dry_run":         cfg.Cleaner.DryRun,
			"download_dir":    cfg.Cleaner.DownloadDir,
		})
	})

	// ─── Graceful Shutdown ────────────────────────────────────────────
	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port),
		Handler: r,
	}

	go func() {
		logger.Info("clarr listening", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	// Attendre signal OS (SIGINT, SIGTERM)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down clarr...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("forced shutdown", zap.Error(err))
	}

	logger.Info("clarr stopped cleanly")
}

// ─── Middleware ───────────────────────────────────────────────────────

func ginZapLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logger.Info("request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("ip", c.ClientIP()),
		)
	}
}
