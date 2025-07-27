package main

import (
	"context"
	"fmt"
	"log"
	"logs-distributor/api"
	"logs-distributor/config"
	"logs-distributor/distributor/implementations"
	"logs-distributor/distributor/interfaces"
	"logs-distributor/models"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	ServiceName = "logs-distributor"
	Version     = "1.0.0"
)

func main() {
	// Initialize logger
	logger := initLogger()
	defer func() {
		if err := logger.Sync(); err != nil {
			// Ignore sync errors for stdout/stderr
		}
	}()

	logger.Info("Starting Logs Distributor Service",
		zap.String("service", ServiceName),
		zap.String("version", Version),
	)

	// Create distributor with explicit dependency injection
	dist := createDistributor(logger)

	// Start distributor
	if err := dist.Start(); err != nil {
		logger.Fatal("Failed to start distributor", zap.Error(err))
	}

	// Setup API handlers
	handler := api.NewHandler(dist, logger)
	router := handler.SetupRoutes()

	// Configure HTTP server
	port := os.Getenv("PORT")
	if port == "" {
		port = config.DefaultPort
	}

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		IdleTimeout:  config.IdleTimeout,
	}

	// Start HTTP server in a goroutine
	go func() {
		logger.Info("Starting HTTP server", zap.String("port", port))
		printStartupMessage(port, dist, logger)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start HTTP server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("Service ready - Press Ctrl+C to shutdown")
	sig := <-quit
	logger.Info("Received shutdown signal", zap.String("signal", sig.String()))

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
	defer cancel()

	// Shutdown HTTP server first
	logger.Info("Shutting down HTTP server...")
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Failed to shutdown HTTP server gracefully", zap.Error(err))
	}

	// Shutdown distributor
	logger.Info("Shutting down distributor...")
	if err := dist.Stop(); err != nil {
		logger.Error("Failed to shutdown distributor gracefully", zap.Error(err))
	}

	logger.Info("Service shutdown complete")
}

// initLogger initializes the zap logger with appropriate configuration
func initLogger() *zap.Logger {
	// Configure logger for production-like output
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	config.DisableCaller = true
	config.DisableStacktrace = true

	// Use console encoder for better readability in development
	config.Encoding = "console"
	config.EncoderConfig = zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		MessageKey:     "message",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
	}

	logger, err := config.Build()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	return logger
}

// createDistributor creates a distributor with explicit dependency injection
func createDistributor(logger *zap.Logger) interfaces.Distributor {
	// Create analyzer configuration
	analyzers := make(map[string]*models.Analyzer)
	analyzerConfigs := config.GetDefaultAnalyzers()

	for _, cfg := range analyzerConfigs {
		analyzer := &models.Analyzer{
			ID:               cfg.ID,
			Name:             cfg.Name,
			Weight:           cfg.Weight,
			ProcessingTimeMs: cfg.ProcessingTimeMs,
			IsHealthy:        true,
			LastHealthCheck:  time.Now(),
		}
		analyzers[analyzer.ID] = analyzer
	}

	// Create channels
	retryChannel := make(chan models.LogPacket, config.RetryChannelBuffer)
	ctx, _ := context.WithCancel(context.Background())

	// Create implementations with dependency injection
	distributorConfig := &implementations.DistributorConfig{
		LoadBalancer:    implementations.NewLoadBalancer(analyzers, logger),
		HealthMonitor:   implementations.NewHealthMonitor(analyzers, logger),
		PersistenceMgr:  implementations.NewPersistenceManager(logger),
		RetryHandler:    implementations.NewRetryHandler(retryChannel, logger, ctx),
		PacketProcessor: implementations.NewPacketProcessor(logger),
		PacketValidator: implementations.NewPacketValidator(),
	}

	return implementations.NewDistributor(logger, distributorConfig)
}

// printStartupMessage prints service information
func printStartupMessage(port string, dist interfaces.Distributor, logger *zap.Logger) {
	logger.Info("=== Logs Distributor Configuration ===")
	logger.Info("Service Details",
		zap.String("service", ServiceName),
		zap.String("version", Version),
		zap.String("port", port),
	)
	logger.Info("System Configuration",
		zap.Int("packet_workers", config.PacketWorkers),
		zap.String("health_check_interval", config.HealthCheckInterval.String()),
		zap.String("checkpoint_interval", config.CheckpointInterval.String()),
	)

	// Get dynamic analyzer information
	stats := dist.GetStats()
	analyzerConfigs := config.GetDefaultAnalyzers()
	weights := make([]string, len(analyzerConfigs))
	for i, cfg := range analyzerConfigs {
		weights[i] = fmt.Sprintf("%.0f%%", cfg.Weight*100)
	}

	logger.Info("Analyzer Configuration",
		zap.Int("analyzer_count", len(stats.AnalyzerStats)),
		zap.Strings("weights", weights),
		zap.Int("retry_attempts", config.MaxRetries),
	)
	logger.Info("API Endpoints Available",
		zap.String("health", "GET /api/v1/health"),
		zap.String("stats", "GET /api/v1/stats"),
		zap.String("logs", "POST /api/v1/logs"),
		zap.String("dead_letter", "GET /api/v1/dead-letter"),
	)
	logger.Info("=========================================")
}
