package tests

import (
	"context"
	"logs-distributor/distributor/implementations"
	"logs-distributor/models"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPersistenceManager_SaveRecover(t *testing.T) {
	logger := createTestLogger()
	defer logger.Sync()

	persistenceManager := implementations.NewPersistenceManager(logger)

	// Clean up test file
	defer os.Remove("distributor_state.json.gz")

	testState := &models.DistributorState{
		Analyzers: map[string]*models.Analyzer{
			"test": {
				ID:   "test",
				Name: "Test Analyzer",
			},
		},
		PendingPackets: []models.LogPacket{},
		LastCheckpoint: time.Now(),
		TotalProcessed: 42,
	}

	// Test saving state
	err := persistenceManager.SaveState(testState)
	assert.NoError(t, err)

	// Test recovering state
	recoveredState, err := persistenceManager.RecoverState()
	assert.NoError(t, err)
	assert.NotNil(t, recoveredState)
	assert.Equal(t, testState.TotalProcessed, recoveredState.TotalProcessed)
	assert.Len(t, recoveredState.Analyzers, 1)
}

func TestPersistenceManager_RecoverNonexistentFile(t *testing.T) {
	logger := createTestLogger()
	defer logger.Sync()

	// Make sure file doesn't exist
	os.Remove("distributor_state.json.gz")

	persistenceManager := implementations.NewPersistenceManager(logger)

	_, err := persistenceManager.RecoverState()
	assert.Error(t, err)
}

func TestPersistenceManager_StartCheckpointing(t *testing.T) {
	logger := createTestLogger()
	defer logger.Sync()

	persistenceManager := implementations.NewPersistenceManager(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	getStateFunc := func() *models.DistributorState {
		return &models.DistributorState{
			TotalProcessed: 1,
		}
	}

	var wg sync.WaitGroup
	persistenceManager.StartCheckpointing(ctx, &wg, getStateFunc)

	// Wait for timeout
	<-ctx.Done()
	wg.Wait()

	// Should complete without panics
	assert.True(t, true)
}

func TestPersistenceManager_SaveWithPendingPackets(t *testing.T) {
	logger := createTestLogger()
	defer logger.Sync()

	persistenceManager := implementations.NewPersistenceManager(logger)

	// Clean up test file
	defer os.Remove("distributor_state.json.gz")

	testState := &models.DistributorState{
		Analyzers: map[string]*models.Analyzer{},
		PendingPackets: []models.LogPacket{
			{
				ID: "packet-1",
				Messages: []models.LogMessage{
					{ID: "msg1", Level: "INFO", Message: "test", Source: "test"},
				},
			},
		},
		LastCheckpoint: time.Now(),
		TotalProcessed: 10,
	}

	err := persistenceManager.SaveState(testState)
	assert.NoError(t, err)

	recoveredState, err := persistenceManager.RecoverState()
	assert.NoError(t, err)
	assert.Len(t, recoveredState.PendingPackets, 1)
	assert.Equal(t, "packet-1", recoveredState.PendingPackets[0].ID)
}
