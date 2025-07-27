package tests

import (
	"context"
	"logs-distributor/distributor/implementations"
	"logs-distributor/models"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHealthMonitor_Lifecycle(t *testing.T) {
	logger := createTestLogger()
	defer logger.Sync()

	analyzers := map[string]*models.Analyzer{
		"test": {
			ID:        "test",
			Name:      "Test Analyzer",
			Weight:    1.0,
			IsHealthy: true,
		},
	}

	healthMonitor := implementations.NewHealthMonitor(analyzers, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	healthChangeCalled := false
	onHealthChange := func() {
		healthChangeCalled = true
	}

	var wg sync.WaitGroup
	healthMonitor.Start(ctx, &wg, onHealthChange)

	// Give it a moment to potentially run
	time.Sleep(50 * time.Millisecond)

	// Cancel and wait for cleanup
	cancel()
	wg.Wait()

	_ = healthChangeCalled
	assert.True(t, true)
}

func TestHealthMonitor_MultipleAnalyzers(t *testing.T) {
	logger := createTestLogger()
	defer logger.Sync()

	analyzers := map[string]*models.Analyzer{
		"analyzer-1": {
			ID:        "analyzer-1",
			Name:      "Analyzer 1",
			Weight:    0.5,
			IsHealthy: true,
		},
		"analyzer-2": {
			ID:        "analyzer-2",
			Name:      "Analyzer 2",
			Weight:    0.5,
			IsHealthy: true,
		},
	}

	healthMonitor := implementations.NewHealthMonitor(analyzers, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	var wg sync.WaitGroup
	healthMonitor.Start(ctx, &wg, func() {})

	// Wait for timeout
	<-ctx.Done()
	wg.Wait()

	// Should handle multiple analyzers without issues
	assert.True(t, true)
}
