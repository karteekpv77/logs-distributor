package interfaces

import (
	"context"
	"logs-distributor/models"
	"sync"
)

// PersistenceManager defines the interface for state persistence
type PersistenceManager interface {
	SaveState(state *models.DistributorState) error
	RecoverState() (*models.DistributorState, error)
	StartCheckpointing(ctx context.Context, wg *sync.WaitGroup, getStateFunc func() *models.DistributorState)
}
