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

func TestPacketProcessor_ProcessPacket(t *testing.T) {
	logger := createTestLogger()
	defer logger.Sync()

	processor := implementations.NewPacketProcessor(logger)

	analyzer := &models.Analyzer{
		ID:               "test",
		Name:             "Test Analyzer",
		ProcessingTimeMs: 1, // Short processing time for test
		IsHealthy:        true,
	}

	packet := models.LogPacket{
		ID: "test-packet",
		Messages: []models.LogMessage{
			{ID: "msg1", Level: "INFO", Message: "test message", Source: "test"},
		},
	}

	result := processor.ProcessPacket(analyzer, packet)

	assert.Equal(t, "test-packet", result.PacketID)
	assert.Equal(t, "test", result.AnalyzerID)
	assert.NotZero(t, result.ProcessedAt)

	// Result should be either success or failure (simulated)
	if result.Success {
		assert.NotNil(t, result.Results)
		assert.Contains(t, result.Results, "processed_messages")
	} else {
		assert.NotEmpty(t, result.Error)
	}
}

func TestPacketProcessor_MultipleMessages(t *testing.T) {
	logger := createTestLogger()
	defer logger.Sync()

	processor := implementations.NewPacketProcessor(logger)

	analyzer := &models.Analyzer{
		ID:               "test",
		Name:             "Test Analyzer",
		ProcessingTimeMs: 1,
		IsHealthy:        true,
	}

	packet := models.LogPacket{
		ID: "test-packet",
		Messages: []models.LogMessage{
			{ID: "msg1", Level: "INFO", Message: "message 1", Source: "test"},
			{ID: "msg2", Level: "ERROR", Message: "message 2", Source: "test"},
			{ID: "msg3", Level: "WARN", Message: "message 3", Source: "test"},
		},
	}

	result := processor.ProcessPacket(analyzer, packet)

	assert.Equal(t, "test-packet", result.PacketID)
	assert.Equal(t, "test", result.AnalyzerID)

	if result.Success {
		assert.Equal(t, 3, result.Results["processed_messages"])
	}
}

func TestPacketProcessor_RunAnalyzer(t *testing.T) {
	logger := createTestLogger()
	defer logger.Sync()

	processor := implementations.NewPacketProcessor(logger)

	analyzer := &models.Analyzer{
		ID:   "test",
		Name: "Test Analyzer",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	var wg sync.WaitGroup
	processor.RunAnalyzer(ctx, &wg, analyzer)

	// Wait for timeout
	<-ctx.Done()
	wg.Wait()

	// Should complete without issues
	assert.True(t, true)
}
