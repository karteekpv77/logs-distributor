package models

import (
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// LogMessage represents a single log entry
type LogMessage struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Source    string                 `json:"source"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// LogPacket represents a collection of log messages sent to the distributor
type LogPacket struct {
	ID         string       `json:"id"`
	Messages   []LogMessage `json:"messages"`
	RetryCount int          `json:"retry_count,omitempty"` // Number of retry attempts
}

// Analyzer represents an analyzer service configuration
type Analyzer struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Weight           float64   `json:"weight"`
	ProcessingTimeMs int       `json:"processing_time_ms"` // simulated processing time
	IsHealthy        bool      `json:"is_healthy"`
	LastHealthCheck  time.Time `json:"last_health_check"`
	ProcessedCount   int64     `json:"processed_count"` // Use atomic operations for these
	ErrorCount       int64     `json:"error_count"`     // Use atomic operations for these
}

// DistributorStats represents current distributor statistics
type DistributorStats struct {
	TotalPacketsReceived int64                `json:"total_packets_received"`
	TotalMessagesRouted  int64                `json:"total_messages_routed"`
	ActiveAnalyzers      int                  `json:"active_analyzers"`
	PacketChannelUtil    float64              `json:"packet_channel_util_percent"`
	ResultChannelUtil    float64              `json:"result_channel_util_percent"`
	RetryChannelUtil     float64              `json:"retry_channel_util_percent"`
	AnalyzerStats        map[string]*Analyzer `json:"analyzer_stats"`
	Uptime               time.Duration        `json:"uptime"`
	LastFailure          *time.Time           `json:"last_failure,omitempty"`
}

// DistributorState represents the state that needs to be persisted for recovery
type DistributorState struct {
	Analyzers      map[string]*Analyzer `json:"analyzers"`
	PendingPackets []LogPacket          `json:"pending_packets"`
	LastCheckpoint time.Time            `json:"last_checkpoint"`
	TotalProcessed int64                `json:"total_processed"`
}

// AnalysisResult represents the result from an analyzer
type AnalysisResult struct {
	PacketID    string                 `json:"packet_id"`
	AnalyzerID  string                 `json:"analyzer_id"`
	Success     bool                   `json:"success"`
	ProcessedAt time.Time              `json:"processed_at"`
	Results     map[string]interface{} `json:"results,omitempty"`
	Error       string                 `json:"error,omitempty"`
}

// NewLogMessage creates a new log message with generated ID and timestamp
func NewLogMessage(level, message, source string, metadata map[string]interface{}) LogMessage {
	return LogMessage{
		ID:        uuid.New().String(),
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Source:    source,
		Metadata:  metadata,
	}
}

// NewLogPacket creates a new log packet with generated ID
func NewLogPacket(messages []LogMessage) LogPacket {
	return LogPacket{
		ID:       uuid.New().String(),
		Messages: messages,
	}
}

// GetProcessedCount returns the processed count safely
func (a *Analyzer) GetProcessedCount() int64 {
	return atomic.LoadInt64(&a.ProcessedCount)
}

// GetErrorCount returns the error count safely
func (a *Analyzer) GetErrorCount() int64 {
	return atomic.LoadInt64(&a.ErrorCount)
}
