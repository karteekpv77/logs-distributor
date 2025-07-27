package interfaces

import "logs-distributor/models"

// Distributor defines the main interface for log distribution services
type Distributor interface {
	// Start begins the distributor service
	Start() error

	// Stop gracefully shuts down the distributor
	Stop() error

	// SubmitPacket submits a log packet for processing
	SubmitPacket(packet models.LogPacket) error

	// GetStats returns current distributor statistics
	GetStats() *models.DistributorStats
}
