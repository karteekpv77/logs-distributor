package implementations

import (
	"context"
	"logs-distributor/config"
	"logs-distributor/distributor/interfaces"
	"logs-distributor/models"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// PacketProcessor implements the PacketProcessor interface
type PacketProcessor struct {
	logger *zap.Logger
}

// Ensure PacketProcessor implements PacketProcessor interface
var _ interfaces.PacketProcessor = (*PacketProcessor)(nil)

func NewPacketProcessor(logger *zap.Logger) interfaces.PacketProcessor {
	return &PacketProcessor{
		logger: logger,
	}
}

// RunAnalyzer runs an embedded analyzer service
func (a *PacketProcessor) RunAnalyzer(ctx context.Context, wg *sync.WaitGroup, analyzer *models.Analyzer) {
	wg.Add(1)
	defer wg.Done()

	<-ctx.Done()
}

// ProcessPacket simulates analysis processing for embedded analyzers
func (a *PacketProcessor) ProcessPacket(analyzer *models.Analyzer, packet models.LogPacket) models.AnalysisResult {
	// Simulate processing time
	time.Sleep(time.Duration(analyzer.ProcessingTimeMs) * time.Millisecond)

	// Simulate occasional failures
	success := rand.Float64() > config.AnalyzerFailureRate

	result := models.AnalysisResult{
		PacketID:    packet.ID,
		AnalyzerID:  analyzer.ID,
		Success:     success,
		ProcessedAt: time.Now(),
	}

	if success {
		result.Results = map[string]interface{}{
			"processed_messages": len(packet.Messages),
			"analyzer_type":      analyzer.Name,
			"processing_time_ms": analyzer.ProcessingTimeMs,
		}
		atomic.AddInt64(&analyzer.ProcessedCount, 1)
	} else {
		result.Error = "simulated processing error"
		atomic.AddInt64(&analyzer.ErrorCount, 1)
	}

	return result
}
