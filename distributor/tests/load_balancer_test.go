package tests

import (
	"logs-distributor/distributor/implementations"
	"logs-distributor/models"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadBalancer_WeightedSelection(t *testing.T) {
	logger := createTestLogger()
	defer logger.Sync()

	// Create analyzers with different weights
	analyzers := map[string]*models.Analyzer{
		"analyzer-1": {
			ID:        "analyzer-1",
			Name:      "Analyzer 1",
			Weight:    0.6,
			IsHealthy: true,
		},
		"analyzer-2": {
			ID:        "analyzer-2",
			Name:      "Analyzer 2",
			Weight:    0.4,
			IsHealthy: true,
		},
	}

	lb := implementations.NewLoadBalancer(analyzers, logger)

	// Test multiple selections to verify weighted distribution
	selections := make(map[string]int)
	for i := 0; i < 100; i++ {
		selected := lb.SelectAnalyzer()
		require.NotNil(t, selected)
		selections[selected.ID]++
	}

	// Verify both analyzers were selected
	assert.Greater(t, selections["analyzer-1"], 0)
	assert.Greater(t, selections["analyzer-2"], 0)

	// Verify weighted distribution (analyzer-1 should get more)
	assert.Greater(t, selections["analyzer-1"], selections["analyzer-2"])
}

func TestLoadBalancer_HealthyOnly(t *testing.T) {
	logger := createTestLogger()
	defer logger.Sync()

	analyzers := map[string]*models.Analyzer{
		"healthy": {
			ID:        "healthy",
			Name:      "Healthy Analyzer",
			Weight:    0.5,
			IsHealthy: true,
		},
		"unhealthy": {
			ID:        "unhealthy",
			Name:      "Unhealthy Analyzer",
			Weight:    0.5,
			IsHealthy: false, // Unhealthy
		},
	}

	lb := implementations.NewLoadBalancer(analyzers, logger)

	// Test that only healthy analyzers are selected
	selections := make(map[string]int)
	for i := 0; i < 10; i++ {
		selected := lb.SelectAnalyzer()
		if selected != nil {
			selections[selected.ID]++
		}
	}

	// Should only select healthy analyzer
	assert.Greater(t, selections["healthy"], 0)
	assert.Equal(t, 0, selections["unhealthy"])
}

func TestLoadBalancer_NoHealthyAnalyzers(t *testing.T) {
	logger := createTestLogger()
	defer logger.Sync()

	// All analyzers unhealthy
	analyzers := map[string]*models.Analyzer{
		"unhealthy": {
			ID:        "unhealthy",
			Name:      "Unhealthy Analyzer",
			Weight:    1.0,
			IsHealthy: false,
		},
	}

	lb := implementations.NewLoadBalancer(analyzers, logger)
	selected := lb.SelectAnalyzer()
	assert.Nil(t, selected, "Should return nil when no healthy analyzers")
}

func TestLoadBalancer_UpdateWeights(t *testing.T) {
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

	lb := implementations.NewLoadBalancer(analyzers, logger)

	// Should not panic when updating weights
	lb.UpdateWeights()

	// Should still work after weight update
	selected := lb.SelectAnalyzer()
	assert.NotNil(t, selected)
}
