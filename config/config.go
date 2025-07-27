package config

import (
	"time"
)

const (
	// Server Configuration
	DefaultPort     = "8080"
	ReadTimeout     = 30 * time.Second
	WriteTimeout    = 30 * time.Second
	IdleTimeout     = 120 * time.Second
	ShutdownTimeout = 30 * time.Second

	// Distributor Configuration
	PacketChannelBuffer = 2000
	ResultChannelBuffer = 2000
	RetryChannelBuffer  = 1000
	PacketWorkers       = 100
	CheckpointInterval  = 30 * time.Second
	HealthCheckInterval = 10 * time.Second
	SubmissionTimeout   = 5 * time.Second
	ResultTimeout       = 1 * time.Second

	// Retry Configuration
	MaxRetries         = 3
	BaseRetryDelay     = 2 * time.Second
	RetryBackoffFactor = 2

	// Failure Simulation
	AnalyzerFailureRate = 0.05 // 5% failure rate
	HealthFailureRate   = 0.05 // 5% unhealthy rate

	// File Paths
	StateFilePath  = "distributor_state.json"
	DeadLetterFile = "failed_packets.json"

	// Validation
	MaxPacketSizeBytes    = 1024 * 1024 // 1MB per packet
	MaxMessagesPerPacket  = 1000
	MinWeight             = 0.0
	MaxWeight             = 1.0
	MaxAnalyzerNameLength = 100
	MaxLogMessageLength   = 10000
)

// AnalyzerConfig represents default analyzer configurations
type AnalyzerConfig struct {
	ID               string
	Name             string
	Weight           float64
	ProcessingTimeMs int
}

// GetDefaultAnalyzers returns the default analyzer configurations
func GetDefaultAnalyzers() []AnalyzerConfig {
	return []AnalyzerConfig{
		{
			ID:               "analyzer-a1",
			Name:             "Analyzer A",
			Weight:           0.4,
			ProcessingTimeMs: 100,
		},
		{
			ID:               "analyzer-a2",
			Name:             "Analyzer B",
			Weight:           0.3,
			ProcessingTimeMs: 150,
		},
		{
			ID:               "analyzer-a3",
			Name:             "Analyzer C",
			Weight:           0.2,
			ProcessingTimeMs: 80,
		},
		{
			ID:               "analyzer-a4",
			Name:             "Analyzer D",
			Weight:           0.1,
			ProcessingTimeMs: 200,
		},
	}
}
