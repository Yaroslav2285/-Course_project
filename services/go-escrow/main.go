// LR #9: Graceful shutdown with signal.NotifyContext, context cancellation
// LR #5: Highload — connection pools, rate limiting, structured logging (zap)

package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/marketplace/go-escrow/internal/api"
	"github.com/marketplace/go-escrow/internal/clients"
	"github.com/marketplace/go-escrow/internal/config"
	"github.com/marketplace/go-escrow/internal/db"
	"github.com/marketplace/go-escrow/internal/repository"
	"github.com/marketplace/go-escrow/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		// LR #5: Fail-fast on missing critical config
		panic("Failed to load config: " + err.Error())
	}

	logLevel := zap.InfoLevel
	if cfg.LogLevel == "debug" {
		logLevel = zap.DebugLevel
	}

	logger, err := zap.Config{
		Level:            zap.NewAtomicLevelAt(logLevel),
		Encoding:         "json",
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
	}.Build()
	if err != nil {
		panic("Failed to create logger: " + err.Error())
	}
	defer logger.Sync()

	logger.Info("Starting go-escrow service",
		zap.String("port", cfg.Port),
		zap.Int("rate_limit_rps", cfg.RateLimitRPS),
	)

	database, err := db.Connect(cfg.DBURL)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer database.Close()
	logger.Info("Connected to database")

	repo := repository.NewEscrowRepository(database.DB, logger)
	blockchainCli := clients.NewBlockchainClient(cfg.BlockchainSimURL, logger)
	svc := service.NewEscrowService(repo, database.DB, logger, blockchainCli)

	rateLimiter := api.NewRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst)
	defer rateLimiter.Stop()

	idempStore := api.NewIdempotencyStore(cfg.IdempotencyTTL)
	defer idempStore.Stop()

	router := api.NewRouter(svc, logger, rateLimiter, idempStore)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// LR #9: Graceful shutdown via signal.NotifyContext
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("HTTP server listening", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	logger.Info("Shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown error", zap.Error(err))
	}

	if err := database.Close(); err != nil {
		logger.Error("database close error", zap.Error(err))
	}
	logger.Info("Server stopped")

	os.Exit(0)
}
