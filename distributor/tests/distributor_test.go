package tests

import (
	"context"
	"logs-distributor/config"
	"logs-distributor/distributor/implementations"
	"logs-distributor/distributor/interfaces"
	"logs-distributor/models"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func createTestLogger() *zap.Logger {
	config := zap.NewDevelopmentConfig()
	config.Level = zap.NewAtomicLevelAt(zap.FatalLevel) // Suppress logs during tests
	logger, _ := config.Build()
	return logger
}

func createTestPacket() models.LogPacket {
	messages := []models.LogMessage{
		models.NewLogMessage("INFO", "test message", "test-service", nil),
	}
	return models.NewLogPacket(messages)
}

// createTestDistributor creates a distributor with real implementations for testing
func createTestDistributor(logger *zap.Logger) interfaces.Distributor {
	// Create test analyzers
	analyzers := map[string]*models.Analyzer{
		"test-analyzer": {
			ID:               "test-analyzer",
			Name:             "Test Analyzer",
			Weight:           1.0,
			ProcessingTimeMs: 1, // Fast for tests
			IsHealthy:        true,
			LastHealthCheck:  time.Now(),
		},
	}

	// Create channels and context
	retryChannel := make(chan models.LogPacket, 10)
	ctx, _ := context.WithCancel(context.Background())

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

func TestDistributor_Lifecycle(t *testing.T) {
	logger := createTestLogger()
	defer logger.Sync()

	d := createTestDistributor(logger)

	// Test initial state
	assert.NotNil(t, d)

	// Test Start
	err := d.Start()
	require.NoError(t, err, "Distributor should start successfully")

	// Test double start should fail
	err = d.Start()
	assert.Error(t, err, "Double start should fail")

	// Test Stop
	err = d.Stop()
	require.NoError(t, err, "Distributor should stop successfully")

	// Test double stop should fail
	err = d.Stop()
	assert.Error(t, err, "Double stop should fail")
}

func TestDistributor_PacketValidation(t *testing.T) {
	logger := createTestLogger()
	defer logger.Sync()

	d := createTestDistributor(logger)
	require.NoError(t, d.Start())
	defer d.Stop()

	tests := []struct {
		name    string
		packet  models.LogPacket
		wantErr bool
	}{
		{
			name:    "empty packet",
			packet:  models.LogPacket{ID: "test", Messages: []models.LogMessage{}},
			wantErr: true,
		},
		{
			name:    "valid packet",
			packet:  createTestPacket(),
			wantErr: false,
		},
		{
			name: "oversized message",
			packet: models.LogPacket{
				ID: "test",
				Messages: []models.LogMessage{
					{
						ID:        "test",
						Timestamp: time.Now(),
						Level:     "INFO",
						Message:   string(make([]byte, config.MaxLogMessageLength+1)),
						Source:    "test",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := d.SubmitPacket(tt.packet)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDistributor_BasicStats(t *testing.T) {
	logger := createTestLogger()
	defer logger.Sync()

	d := createTestDistributor(logger)
	require.NoError(t, d.Start())
	defer d.Stop()

	stats := d.GetStats()
	assert.NotNil(t, stats)
	assert.Greater(t, stats.ActiveAnalyzers, 0, "Should have active analyzers")
	assert.GreaterOrEqual(t, stats.TotalPacketsReceived, int64(0))
}
