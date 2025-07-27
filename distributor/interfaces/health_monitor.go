package interfaces

import (
	"context"
	"sync"
)

// HealthMonitor defines the interface for analyzer health monitoring
type HealthMonitor interface {
	Start(ctx context.Context, wg *sync.WaitGroup, onHealthChange func())
}
