package implementations

import (
	"context"
	"fmt"
	"logs-distributor/config"
	"logs-distributor/distributor/interfaces"
	"logs-distributor/models"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// DistributorConfig holds the configuration for creating a Distributor
type DistributorConfig struct {
	LoadBalancer    interfaces.LoadBalancer
	HealthMonitor   interfaces.HealthMonitor
	PersistenceMgr  interfaces.PersistenceManager
	RetryHandler    interfaces.RetryHandler
	PacketProcessor interfaces.PacketProcessor
	PacketValidator interfaces.PacketValidator
}

// Distributor implements the Distributor interface
type Distributor struct {
	analyzers map[string]*models.Analyzer
	logger    *zap.Logger
	stats     *models.DistributorStats
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.RWMutex
	startTime time.Time
	isRunning bool

	// Injected components - now using interfaces
	loadBalancer    interfaces.LoadBalancer
	health          interfaces.HealthMonitor
	persistence     interfaces.PersistenceManager
	retryHandler    interfaces.RetryHandler
	packetProcessor interfaces.PacketProcessor
	validator       interfaces.PacketValidator

	// Channels
	packetChannel chan models.LogPacket
	resultChannel chan models.AnalysisResult
	retryChannel  chan models.LogPacket

	// Atomic counters
	totalPacketsReceived int64
	totalMessagesRouted  int64
}

// Ensure Distributor implements Distributor interface
var _ interfaces.Distributor = (*Distributor)(nil)

// NewDistributor creates a new distributor instance with dependency injection
func NewDistributor(logger *zap.Logger, cfg *DistributorConfig) interfaces.Distributor {
	ctx, cancel := context.WithCancel(context.Background())

	d := &Distributor{
		analyzers:     make(map[string]*models.Analyzer),
		logger:        logger,
		ctx:           ctx,
		cancel:        cancel,
		startTime:     time.Now(),
		packetChannel: make(chan models.LogPacket, config.PacketChannelBuffer),
		resultChannel: make(chan models.AnalysisResult, config.ResultChannelBuffer),
		retryChannel:  make(chan models.LogPacket, config.RetryChannelBuffer),
		stats: &models.DistributorStats{
			AnalyzerStats: make(map[string]*models.Analyzer),
		},
		// Injected dependencies
		loadBalancer:    cfg.LoadBalancer,
		health:          cfg.HealthMonitor,
		persistence:     cfg.PersistenceMgr,
		retryHandler:    cfg.RetryHandler,
		packetProcessor: cfg.PacketProcessor,
		validator:       cfg.PacketValidator,
	}

	// Initialize analyzers
	if err := d.initializeAnalyzers(); err != nil {
		logger.Fatal("CRITICAL: Failed to initialize analyzers - cannot start service", zap.Error(err))
	}

	return d
}

// initializeAnalyzers sets up default analyzers
func (d *Distributor) initializeAnalyzers() error {
	defaultConfigs := config.GetDefaultAnalyzers()

	for _, cfg := range defaultConfigs {
		if err := d.validateAnalyzerConfig(cfg); err != nil {
			return fmt.Errorf("invalid analyzer config %s: %w", cfg.ID, err)
		}

		analyzer := &models.Analyzer{
			ID:               cfg.ID,
			Name:             cfg.Name,
			Weight:           cfg.Weight,
			ProcessingTimeMs: cfg.ProcessingTimeMs,
			IsHealthy:        true,
			LastHealthCheck:  time.Now(),
		}

		d.analyzers[analyzer.ID] = analyzer
		d.stats.AnalyzerStats[analyzer.ID] = analyzer
	}

	return nil
}

// validateAnalyzerConfig validates analyzer configuration
func (d *Distributor) validateAnalyzerConfig(cfg config.AnalyzerConfig) error {
	if cfg.Weight < config.MinWeight || cfg.Weight > config.MaxWeight {
		return fmt.Errorf("weight %.2f must be between %.2f and %.2f", cfg.Weight, config.MinWeight, config.MaxWeight)
	}
	if len(cfg.Name) > config.MaxAnalyzerNameLength {
		return fmt.Errorf("name length %d exceeds maximum %d", len(cfg.Name), config.MaxAnalyzerNameLength)
	}
	if cfg.ProcessingTimeMs < 0 {
		return fmt.Errorf("processing time cannot be negative")
	}
	return nil
}

// Start begins the distributor service
func (d *Distributor) Start() error {
	d.mu.Lock()
	if d.isRunning {
		d.mu.Unlock()
		return fmt.Errorf("distributor is already running")
	}
	d.isRunning = true
	d.mu.Unlock()

	// Recover state from previous run
	if state, err := d.persistence.RecoverState(); err != nil {
		d.logger.Error("Failed to recover previous state", zap.Error(err))
	} else {
		d.recoverFromState(state)
	}

	for i := 0; i < config.PacketWorkers; i++ {
		go d.processPackets()
		go d.processResults()
	}

	go d.retryHandler.ProcessRetries(d.ctx, &d.wg, d.packetChannel)
	go d.persistence.StartCheckpointing(d.ctx, &d.wg, d.getState)

	d.health.Start(d.ctx, &d.wg, func() { d.loadBalancer.UpdateWeights() })

	for _, analyzer := range d.getAnalyzers() {
		go d.packetProcessor.RunAnalyzer(d.ctx, &d.wg, analyzer)
	}

	return nil
}

// Stop gracefully shuts down the distributor
func (d *Distributor) Stop() error {
	d.mu.Lock()
	if !d.isRunning {
		d.mu.Unlock()
		return fmt.Errorf("distributor is not running")
	}
	d.isRunning = false
	d.mu.Unlock()

	// Save current state before shutdown
	if err := d.persistence.SaveState(d.getState()); err != nil {
		d.logger.Error("Failed to save state during shutdown", zap.Error(err))
	}

	// Cancel context and wait for goroutines to finish
	d.cancel()
	d.wg.Wait()

	// Close channels to prevent resource leaks and signal shutdown completion
	close(d.packetChannel)
	close(d.resultChannel)
	close(d.retryChannel)

	return nil
}

// SubmitPacket submits a log packet for processing
func (d *Distributor) SubmitPacket(packet models.LogPacket) error {
	if err := d.validator.ValidatePacket(packet); err != nil {
		return fmt.Errorf("packet validation failed: %w", err)
	}

	// Track packet for retry
	d.retryHandler.TrackPacket(packet)
	atomic.AddInt64(&d.totalPacketsReceived, 1)

	select {
	case d.packetChannel <- packet:
		return nil
	case <-time.After(config.SubmissionTimeout):
		d.retryHandler.UntrackPacket(packet.ID)
		return fmt.Errorf("submission timeout: queue full")
	case <-d.ctx.Done():
		return fmt.Errorf("distributor shutting down")
	}
}

// processPackets handles incoming log packets (runs as worker pool for high throughput)
func (d *Distributor) processPackets() {
	d.wg.Add(1)
	defer d.wg.Done()

	for {
		select {
		case packet := <-d.packetChannel:
			// Add small delay to simulate worker processing time
			time.Sleep(100 * time.Millisecond)
			d.distributePacket(packet)
		case <-d.ctx.Done():
			return
		}
	}
}

// distributePacket distributes a packet using the load balancer
func (d *Distributor) distributePacket(packet models.LogPacket) {
	selectedAnalyzer := d.loadBalancer.SelectAnalyzer()
	if selectedAnalyzer == nil {
		d.logger.Error("No healthy analyzers available, requeueing packet", zap.String("packet_id", packet.ID))
		d.requeuePacketWithDelay(packet, 5*time.Second)
		return
	}

	// Send to analyzer for processing
	go d.sendToAnalyzer(selectedAnalyzer, packet)
	atomic.AddInt64(&d.totalMessagesRouted, int64(len(packet.Messages)))
}

// sendToAnalyzer sends a packet to a specific analyzer
func (d *Distributor) sendToAnalyzer(analyzer *models.Analyzer, packet models.LogPacket) {
	result := d.packetProcessor.ProcessPacket(analyzer, packet)

	select {
	case d.resultChannel <- result:
		// Success - result submitted
	case <-d.ctx.Done():
		// Shutting down - create failure result to ensure packet is untracked
		failureResult := models.AnalysisResult{
			PacketID:    packet.ID,
			AnalyzerID:  analyzer.ID,
			Success:     false,
			ProcessedAt: time.Now(),
			Error:       "processing interrupted by shutdown",
		}
		d.retryHandler.HandleFailedPacket(failureResult)
		return
	case <-time.After(config.ResultTimeout):
		// Result channel full - create failure result to trigger retry logic

		failureResult := models.AnalysisResult{
			PacketID:    packet.ID,
			AnalyzerID:  analyzer.ID,
			Success:     false,
			ProcessedAt: time.Now(),
			Error:       "result channel timeout - system overloaded",
		}
		d.retryHandler.HandleFailedPacket(failureResult)
	}
}

// processResults handles analysis results
func (d *Distributor) processResults() {
	d.wg.Add(1)
	defer d.wg.Done()

	for {
		select {
		case result := <-d.resultChannel:
			// Add delay to simulate result processing (database writes, etc.)
			time.Sleep(100 * time.Millisecond)

			if result.Success {
				d.retryHandler.UntrackPacket(result.PacketID)
			} else {
				d.retryHandler.HandleFailedPacket(result)
			}
		case <-d.ctx.Done():
			return
		}
	}
}

// requeuePacketWithDelay requeues a packet after a delay
func (d *Distributor) requeuePacketWithDelay(packet models.LogPacket, delay time.Duration) {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		select {
		case d.packetChannel <- packet:
		case <-d.ctx.Done():
		default:
			d.logger.Error("Failed to requeue packet: queue full", zap.String("packet_id", packet.ID))
		}
	case <-d.ctx.Done():
	}
}

// GetStats returns current distributor statistics
func (d *Distributor) GetStats() *models.DistributorStats {
	d.mu.RLock()
	defer d.mu.RUnlock()

	statsCopy := *d.stats
	statsCopy.TotalPacketsReceived = d.getTotalPacketsReceived()
	statsCopy.TotalMessagesRouted = atomic.LoadInt64(&d.totalMessagesRouted)
	statsCopy.Uptime = time.Since(d.startTime)
	statsCopy.PacketChannelUtil = float64(len(d.packetChannel)) / float64(config.PacketChannelBuffer) * 100
	statsCopy.ResultChannelUtil = float64(len(d.resultChannel)) / float64(config.ResultChannelBuffer) * 100
	statsCopy.RetryChannelUtil = float64(len(d.retryChannel)) / float64(config.RetryChannelBuffer) * 100

	// Count active analyzers
	activeCount := 0
	for _, analyzer := range d.getAnalyzers() {
		if analyzer.IsHealthy {
			activeCount++
		}
	}
	statsCopy.ActiveAnalyzers = activeCount

	return &statsCopy
}

// getFailedPacketsCount returns the count of failed packets
func (d *Distributor) getFailedPacketsCount() int {
	return d.retryHandler.GetFailedPacketsCount(d.getAnalyzers())
}

// getAnalyzers returns the analyzers map
func (d *Distributor) getAnalyzers() map[string]*models.Analyzer {
	return d.analyzers
}

// getPacketChannel returns the packet channel
func (d *Distributor) getPacketChannel() chan models.LogPacket {
	return d.packetChannel
}

// getTotalPacketsReceived returns atomic counter value
func (d *Distributor) getTotalPacketsReceived() int64 {
	return atomic.LoadInt64(&d.totalPacketsReceived)
}

// setTotalPacketsReceived sets atomic counter value
func (d *Distributor) setTotalPacketsReceived(value int64) {
	atomic.StoreInt64(&d.totalPacketsReceived, value)
}

// getState creates a state snapshot for persistence
func (d *Distributor) getState() *models.DistributorState {
	d.mu.RLock()
	defer d.mu.RUnlock()

	trackedPackets := d.retryHandler.GetTrackedPackets()

	return &models.DistributorState{
		Analyzers:      d.getAnalyzers(),
		PendingPackets: trackedPackets,
		LastCheckpoint: time.Now(),
		TotalProcessed: d.getTotalPacketsReceived(),
	}
}

// recoverFromState restores state from persistence
func (d *Distributor) recoverFromState(state *models.DistributorState) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Restore analyzers
	for id, analyzer := range state.Analyzers {
		if existing, ok := d.getAnalyzers()[id]; ok {
			atomic.AddInt64(&existing.ProcessedCount, analyzer.ProcessedCount)
			atomic.AddInt64(&existing.ErrorCount, analyzer.ErrorCount)
			existing.LastHealthCheck = analyzer.LastHealthCheck
		}
	}

	// Restore tracked packets
	restoredCount := 0
	for _, packet := range state.PendingPackets {
		d.retryHandler.TrackPacket(packet)

		select {
		case d.packetChannel <- packet:
			restoredCount++
		default:
			d.logger.Info("Channel full during recovery, packet will be retried",
				zap.String("packet_id", packet.ID))
		}
	}

	d.setTotalPacketsReceived(state.TotalProcessed)

	if len(state.PendingPackets) > 0 {
		d.logger.Info("State recovery completed",
			zap.Int("total_pending", len(state.PendingPackets)),
			zap.Int("resubmitted", restoredCount),
		)
	}
}
