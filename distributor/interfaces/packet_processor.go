package interfaces

import (
	"context"
	"logs-distributor/models"
	"sync"
)

// PacketProcessor defines the interface for packet analysis
type PacketProcessor interface {
	ProcessPacket(analyzer *models.Analyzer, packet models.LogPacket) models.AnalysisResult
	RunAnalyzer(ctx context.Context, wg *sync.WaitGroup, analyzer *models.Analyzer)
}
