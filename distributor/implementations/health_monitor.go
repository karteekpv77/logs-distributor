package implementations

import (
	"context"
	"logs-distributor/config"
	"logs-distributor/distributor/interfaces"
	"logs-distributor/models"
	"math/rand"
	"sync"
	"time"

	"go.uber.org/zap"
)

// HealthMonitor implements the HealthMonitor interface
type HealthMonitor struct {
	analyzers map[string]*models.Analyzer
	logger    *zap.Logger
}

// Ensure HealthMonitor implements HealthMonitor interface
var _ interfaces.HealthMonitor = (*HealthMonitor)(nil)

func NewHealthMonitor(analyzers map[string]*models.Analyzer, logger *zap.Logger) interfaces.HealthMonitor {
	return &HealthMonitor{
		analyzers: analyzers,
		logger:    logger,
	}
}

// Start begins health monitoring
func (h *HealthMonitor) Start(ctx context.Context, wg *sync.WaitGroup, onHealthChange func()) {
	go h.healthChecker(ctx, wg, onHealthChange)
}

// healthChecker periodically checks analyzer health
func (h *HealthMonitor) healthChecker(ctx context.Context, wg *sync.WaitGroup, onHealthChange func()) {
	wg.Add(1)
	defer wg.Done()

	ticker := time.NewTicker(config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if h.checkAnalyzerHealth() {
				onHealthChange()
			}
		case <-ctx.Done():
			return
		}
	}
}

// checkAnalyzerHealth checks and updates analyzer health status
func (h *HealthMonitor) checkAnalyzerHealth() bool {
	healthChanged := false

	for _, analyzer := range h.analyzers {
		oldHealth := analyzer.IsHealthy

		// Simulate health check
		analyzer.IsHealthy = rand.Float64() > config.HealthFailureRate
		analyzer.LastHealthCheck = time.Now()

		if oldHealth != analyzer.IsHealthy {
			healthChanged = true

			h.logger.Info("Analyzer health changed",
				zap.String("analyzer", analyzer.ID),
				zap.String("name", analyzer.Name),
				zap.Bool("healthy", analyzer.IsHealthy),
			)
		}
	}

	return healthChanged
}
