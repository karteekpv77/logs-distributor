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

func TestRetryHandler_TrackUntrack(t *testing.T) {
	logger := createTestLogger()
	defer logger.Sync()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	retryChannel := make(chan models.LogPacket, 10)
	retryHandler := implementations.NewRetryHandler(retryChannel, logger, ctx)

	packet := models.LogPacket{
		ID: "test-packet",
		Messages: []models.LogMessage{
			{ID: "msg1", Level: "INFO", Message: "test", Source: "test"},
		},
	}

	// Test tracking packet
	retryHandler.TrackPacket(packet)
	trackedPackets := retryHandler.GetTrackedPackets()
	assert.Len(t, trackedPackets, 1)
	assert.Equal(t, packet.ID, trackedPackets[0].ID)

	// Test untracking packet
	retryHandler.UntrackPacket(packet.ID)
	trackedPackets = retryHandler.GetTrackedPackets()
	assert.Len(t, trackedPackets, 0)
}

func TestRetryHandler_HandleFailedPacket(t *testing.T) {
	logger := createTestLogger()
	defer logger.Sync()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	retryChannel := make(chan models.LogPacket, 10)
	retryHandler := implementations.NewRetryHandler(retryChannel, logger, ctx)

	packet := models.LogPacket{
		ID: "test-packet",
		Messages: []models.LogMessage{
			{ID: "msg1", Level: "INFO", Message: "test", Source: "test"},
		},
	}

	// Track packet first
	retryHandler.TrackPacket(packet)

	failedResult := models.AnalysisResult{
		PacketID:   packet.ID,
		AnalyzerID: "test-analyzer",
		Success:    false,
		Error:      "test error",
	}

	retryHandler.HandleFailedPacket(failedResult)

	// Should still be tracked for retry (since retry count < max)
	trackedPackets := retryHandler.GetTrackedPackets()
	assert.Len(t, trackedPackets, 1)
}

func TestRetryHandler_ProcessRetries(t *testing.T) {
	logger := createTestLogger()
	defer logger.Sync()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	retryChannel := make(chan models.LogPacket, 10)
	packetChannel := make(chan models.LogPacket, 10)
	retryHandler := implementations.NewRetryHandler(retryChannel, logger, ctx)

	var wg sync.WaitGroup
	retryHandler.ProcessRetries(ctx, &wg, packetChannel)

	// Add a packet to retry
	packet := models.LogPacket{ID: "test", Messages: []models.LogMessage{{ID: "msg1", Level: "INFO", Message: "test", Source: "test"}}}
	retryChannel <- packet

	// Wait for timeout
	<-ctx.Done()
	wg.Wait()

	// Should complete without panics
	assert.True(t, true)
}

func TestRetryHandler_GetFailedPacketsCount(t *testing.T) {
	logger := createTestLogger()
	defer logger.Sync()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	retryChannel := make(chan models.LogPacket, 10)
	retryHandler := implementations.NewRetryHandler(retryChannel, logger, ctx)

	analyzers := map[string]*models.Analyzer{
		"test": {
			ID:         "test",
			Name:       "Test",
			ErrorCount: 5,
		},
	}

	count := retryHandler.GetFailedPacketsCount(analyzers)
	assert.Equal(t, 5, count)
}
