package interfaces

import (
	"context"
	"logs-distributor/models"
	"sync"
)

// RetryHandler defines the interface for retry logic
type RetryHandler interface {
	TrackPacket(packet models.LogPacket)
	UntrackPacket(packetID string)
	HandleFailedPacket(result models.AnalysisResult)
	ProcessRetries(ctx context.Context, wg *sync.WaitGroup, packetChannel chan models.LogPacket)
	GetFailedPacketsCount(analyzers map[string]*models.Analyzer) int
	GetTrackedPackets() []models.LogPacket
}
