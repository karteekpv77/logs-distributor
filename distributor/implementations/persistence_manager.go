package implementations

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"logs-distributor/config"
	"logs-distributor/distributor/interfaces"
	"logs-distributor/models"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
)

// PersistenceManager implements the PersistenceManager interface
type PersistenceManager struct {
	logger *zap.Logger
}

// Ensure PersistenceManager implements PersistenceManager interface
var _ interfaces.PersistenceManager = (*PersistenceManager)(nil)

func NewPersistenceManager(logger *zap.Logger) interfaces.PersistenceManager {
	return &PersistenceManager{
		logger: logger,
	}
}

// StartCheckpointing begins periodic state saving
func (s *PersistenceManager) StartCheckpointing(ctx context.Context, wg *sync.WaitGroup, getStateFunc func() *models.DistributorState) {
	go s.stateCheckpointer(ctx, wg, getStateFunc)
}

// stateCheckpointer periodically saves distributor state
func (s *PersistenceManager) stateCheckpointer(ctx context.Context, wg *sync.WaitGroup, getStateFunc func() *models.DistributorState) {
	wg.Add(1)
	defer wg.Done()

	ticker := time.NewTicker(config.CheckpointInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if state := getStateFunc(); state != nil {
				if err := s.SaveState(state); err != nil {
					s.logger.Error("Failed to checkpoint state", zap.Error(err))
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

// SaveState saves the current distributor state to disk with gzip compression
func (s *PersistenceManager) SaveState(state *models.DistributorState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Write compressed file
	compressedFile := config.StateFilePath + ".gz"
	file, err := os.Create(compressedFile)
	if err != nil {
		return fmt.Errorf("failed to create compressed state file: %w", err)
	}
	defer file.Close()

	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()

	if _, err := gzipWriter.Write(data); err != nil {
		return fmt.Errorf("failed to write compressed state: %w", err)
	}

	gzipWriter.Close()
	file.Close()

	return nil
}

// RecoverState recovers distributor state from disk
func (s *PersistenceManager) RecoverState() (*models.DistributorState, error) {
	compressedFile := config.StateFilePath + ".gz"
	file, err := os.Open(compressedFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open compressed state file: %w", err)
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	data, err := ioutil.ReadAll(gzipReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read compressed data: %w", err)
	}

	var state models.DistributorState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return &state, nil
}
