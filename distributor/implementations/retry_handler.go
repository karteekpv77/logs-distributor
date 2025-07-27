package implementations

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"logs-distributor/config"
	"logs-distributor/distributor/interfaces"
	"logs-distributor/models"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// RetryHandler implements the RetryHandler interface
type RetryHandler struct {
	retryChannel chan models.LogPacket
	logger       *zap.Logger
	mu           sync.RWMutex
	packetMap    map[string]models.LogPacket
	ctx          context.Context
}

// Ensure RetryHandler implements RetryHandler interface
var _ interfaces.RetryHandler = (*RetryHandler)(nil)

func NewRetryHandler(retryChannel chan models.LogPacket, logger *zap.Logger, ctx context.Context) interfaces.RetryHandler {
	return &RetryHandler{
		retryChannel: retryChannel,
		logger:       logger,
		packetMap:    make(map[string]models.LogPacket),
		ctx:          ctx,
	}
}

// TrackPacket stores a packet for potential retry
func (r *RetryHandler) TrackPacket(packet models.LogPacket) {
	r.mu.Lock()
	r.packetMap[packet.ID] = packet
	r.mu.Unlock()
}

// UntrackPacket removes a packet from tracking (success case)
func (r *RetryHandler) UntrackPacket(packetID string) {
	r.mu.Lock()
	delete(r.packetMap, packetID)
	r.mu.Unlock()
}

// HandleFailedPacket implements retry logic with exponential backoff
func (r *RetryHandler) HandleFailedPacket(result models.AnalysisResult) {
	r.mu.Lock()
	packet, exists := r.packetMap[result.PacketID]
	if !exists {
		r.mu.Unlock()
		r.logger.Error("Failed packet not found in map", zap.String("packet_id", result.PacketID))
		return
	}

	if packet.RetryCount < config.MaxRetries {
		packet.RetryCount++
		backoffDuration := time.Duration(packet.RetryCount*config.RetryBackoffFactor) * config.BaseRetryDelay

		// Update packet in map with new retry count
		r.packetMap[result.PacketID] = packet
		r.mu.Unlock()

		// Schedule retry with proper timer cleanup
		go r.scheduleRetryWithCleanup(r.ctx, packet, backoffDuration)
	} else {
		// Remove from packet map before logging (prevent further concurrent access)
		delete(r.packetMap, result.PacketID)
		r.mu.Unlock()

		r.logger.Error("Packet failed permanently after max retries",
			zap.String("packet_id", result.PacketID),
			zap.Int("retry_count", packet.RetryCount),
			zap.String("final_error", result.Error),
		)

		r.saveToDeadLetterFile(packet, result.Error)
	}
}

// scheduleRetryWithCleanup schedules retry with proper resource cleanup
func (r *RetryHandler) scheduleRetryWithCleanup(ctx context.Context, packet models.LogPacket, delay time.Duration) {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		select {
		case r.retryChannel <- packet:
		case <-ctx.Done():
			// Context cancelled during retry submission - drop packet
		default:
			r.logger.Error("Failed to schedule retry: channel full", zap.String("packet_id", packet.ID))
		}
	case <-ctx.Done():
		// Context cancelled during delay - abort retry
	}
}

// ProcessRetries handles retry packets
func (r *RetryHandler) ProcessRetries(ctx context.Context, wg *sync.WaitGroup, packetChannel chan models.LogPacket) {
	wg.Add(1)
	defer wg.Done()

	for {
		select {
		case packet := <-r.retryChannel:
			// Update packet map with retry count
			r.mu.Lock()
			r.packetMap[packet.ID] = packet
			r.mu.Unlock()

			// Resubmit for processing
			select {
			case packetChannel <- packet:
			case <-time.After(config.SubmissionTimeout):
				r.logger.Error("Failed to submit retry packet", zap.String("packet_id", packet.ID))
			case <-ctx.Done():
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// saveToDeadLetterFile saves permanently failed packets with rotation
func (r *RetryHandler) saveToDeadLetterFile(packet models.LogPacket, finalError string) {
	deadLetterEntry := struct {
		Packet     models.LogPacket `json:"packet"`
		FinalError string           `json:"final_error"`
		FailedAt   time.Time        `json:"failed_at"`
	}{
		Packet:     packet,
		FinalError: finalError,
		FailedAt:   time.Now(),
	}

	// Read existing dead letter entries
	var deadLetterEntries []interface{}
	if data, err := ioutil.ReadFile(config.DeadLetterFile); err == nil {
		if err := json.Unmarshal(data, &deadLetterEntries); err != nil {
			r.logger.Error("Failed to parse existing dead letter file", zap.Error(err))
		}
	}

	// Limit dead letter file size (rotate if too large)
	const maxDeadLetterEntries = 10000
	if len(deadLetterEntries) >= maxDeadLetterEntries {
		// Keep only the most recent entries
		deadLetterEntries = deadLetterEntries[len(deadLetterEntries)-maxDeadLetterEntries/2:]
		r.logger.Info("Rotated dead letter file", zap.Int("new_size", len(deadLetterEntries)))
	}

	deadLetterEntries = append(deadLetterEntries, deadLetterEntry)

	// Write back to file with compact JSON
	if data, err := json.Marshal(deadLetterEntries); err == nil {
		if err := ioutil.WriteFile(config.DeadLetterFile, data, 0644); err != nil {
			r.logger.Error("Failed to write dead letter file", zap.Error(err))
		} else {
			r.logger.Info("Packet saved to dead letter file",
				zap.String("packet_id", packet.ID),
				zap.String("file", config.DeadLetterFile),
			)
		}
	}
}

// GetFailedPacketsCount returns the count of failed packets
func (r *RetryHandler) GetFailedPacketsCount(analyzers map[string]*models.Analyzer) int {
	totalErrors := int64(0)
	for _, analyzer := range analyzers {
		totalErrors += atomic.LoadInt64(&analyzer.ErrorCount)
	}
	return int(totalErrors)
}

// GetTrackedPackets returns a copy of currently tracked packets for persistence
func (r *RetryHandler) GetTrackedPackets() []models.LogPacket {
	r.mu.RLock()
	defer r.mu.RUnlock()

	packets := make([]models.LogPacket, 0, len(r.packetMap))
	for _, packet := range r.packetMap {
		packets = append(packets, packet)
	}
	return packets
}
