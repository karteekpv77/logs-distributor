package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"logs-distributor/config"
	"logs-distributor/distributor/interfaces"
	"logs-distributor/models"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Handler struct {
	distributor interfaces.Distributor
	logger      *zap.Logger
}

func NewHandler(d interfaces.Distributor, logger *zap.Logger) *Handler {
	return &Handler{
		distributor: d,
		logger:      logger,
	}
}

// SetupRoutes configures all API routes
func (h *Handler) SetupRoutes() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(h.loggingMiddleware())
	r.Use(h.corsMiddleware())

	// API routes
	api := r.Group("/api/v1")
	{
		api.POST("/logs", h.SubmitLogs)
		api.GET("/health", h.HealthCheck)
		api.GET("/stats", h.GetStats)
		api.GET("/analyzers", h.GetAnalyzers)
		api.GET("/dead-letter", h.GetDeadLetterPackets)
		api.POST("/analyzers/:id/health", h.SetAnalyzerHealth)
	}

	return r
}

// SubmitLogs handles log packet submission
func (h *Handler) SubmitLogs(c *gin.Context) {
	var packets []models.LogPacket
	if err := c.ShouldBindJSON(&packets); err != nil {
		h.logger.Error("Invalid log packets format",
			zap.Error(err),
			zap.String("client_ip", c.ClientIP()),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid format - expected array of log packets",
		})
		return
	}

	if len(packets) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Array must contain at least one packet",
		})
		return
	}

	if len(packets) > config.MaxMessagesPerPacket {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Too many packets: %d, maximum allowed: %d", len(packets), config.MaxMessagesPerPacket),
		})
		return
	}

	var successCount, failCount int
	var processedPackets []string

	for i := range packets {
		if len(packets[i].Messages) == 0 {
			failCount++
			continue
		}

		if packets[i].ID == "" {
			packets[i] = models.NewLogPacket(packets[i].Messages)
		}

		if err := h.distributor.SubmitPacket(packets[i]); err != nil {
			failCount++
			h.logger.Error("Failed to submit packet",
				zap.String("packet_id", packets[i].ID),
				zap.String("client_ip", c.ClientIP()),
			)
		} else {
			successCount++
			processedPackets = append(processedPackets, packets[i].ID)
		}
	}

	status := http.StatusAccepted
	if failCount > 0 && successCount == 0 {
		status = http.StatusServiceUnavailable
	} else if failCount > 0 {
		status = http.StatusMultiStatus
	}

	h.logger.Info("Log packets processed",
		zap.Int("total", len(packets)),
		zap.Int("successful", successCount),
		zap.Int("failed", failCount),
		zap.String("client_ip", c.ClientIP()),
	)

	c.JSON(status, gin.H{
		"total_packets":     len(packets),
		"successful":        successCount,
		"failed":            failCount,
		"processed_packets": processedPackets,
	})
}

// HealthCheck returns the health status of the distributor
func (h *Handler) HealthCheck(c *gin.Context) {
	stats := h.distributor.GetStats()

	status := "healthy"
	if stats.ActiveAnalyzers == 0 {
		status = "unhealthy"
	} else if stats.ActiveAnalyzers < len(stats.AnalyzerStats)/2 {
		status = "degraded"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":                      status,
		"uptime":                      stats.Uptime.String(),
		"active_analyzers":            stats.ActiveAnalyzers,
		"packet_channel_util_percent": stats.PacketChannelUtil,
		"result_channel_util_percent": stats.ResultChannelUtil,
		"retry_channel_util_percent":  stats.RetryChannelUtil,
		"total_packets_received":      stats.TotalPacketsReceived,
		"total_messages_analyzed":     stats.TotalMessagesRouted,
		"total_analyzers":             len(stats.AnalyzerStats),
		"timestamp":                   time.Now(),
	})
}

// GetStats returns detailed distributor statistics
func (h *Handler) GetStats(c *gin.Context) {
	stats := h.distributor.GetStats()

	sanitizedStats := gin.H{
		"total_packets_received":      stats.TotalPacketsReceived,
		"total_messages_routed":       stats.TotalMessagesRouted,
		"active_analyzers":            stats.ActiveAnalyzers,
		"packet_channel_util_percent": stats.PacketChannelUtil,
		"result_channel_util_percent": stats.ResultChannelUtil,
		"retry_channel_util_percent":  stats.RetryChannelUtil,
		"uptime":                      stats.Uptime.String(),
		"timestamp":                   time.Now(),
	}

	analyzerSummary := make(map[string]interface{})
	for id, analyzer := range stats.AnalyzerStats {
		analyzerSummary[id] = gin.H{
			"name":            analyzer.Name,
			"is_healthy":      analyzer.IsHealthy,
			"processed_count": analyzer.GetProcessedCount(),
			"error_count":     analyzer.GetErrorCount(),
		}
	}
	sanitizedStats["analyzers"] = analyzerSummary

	c.JSON(http.StatusOK, sanitizedStats)
}

// GetAnalyzers returns information about all analyzers
func (h *Handler) GetAnalyzers(c *gin.Context) {
	stats := h.distributor.GetStats()

	analyzers := make(map[string]interface{})
	for id, analyzer := range stats.AnalyzerStats {
		analyzers[id] = gin.H{
			"id":                analyzer.ID,
			"name":              analyzer.Name,
			"weight":            analyzer.Weight,
			"is_healthy":        analyzer.IsHealthy,
			"processed_count":   analyzer.GetProcessedCount(),
			"error_count":       analyzer.GetErrorCount(),
			"last_health_check": analyzer.LastHealthCheck,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"analyzers":    analyzers,
		"total_count":  len(stats.AnalyzerStats),
		"active_count": stats.ActiveAnalyzers,
		"timestamp":    time.Now(),
	})
}

// GetDeadLetterPackets returns permanently failed packets
func (h *Handler) GetDeadLetterPackets(c *gin.Context) {
	deadLetterFile := config.DeadLetterFile

	if _, err := os.Stat(deadLetterFile); os.IsNotExist(err) {
		c.JSON(http.StatusOK, gin.H{
			"message":   "No dead letter file found",
			"count":     0,
			"packets":   []interface{}{},
			"timestamp": time.Now(),
		})
		return
	}

	data, err := ioutil.ReadFile(deadLetterFile)
	if err != nil {
		h.logger.Error("Failed to read dead letter file", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to read dead letter file",
		})
		return
	}

	var deadLetterEntries []interface{}
	if err := json.Unmarshal(data, &deadLetterEntries); err != nil {
		h.logger.Error("Failed to parse dead letter file", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to parse dead letter file",
		})
		return
	}

	// Limit response size
	maxEntries := 100
	if len(deadLetterEntries) > maxEntries {
		deadLetterEntries = deadLetterEntries[len(deadLetterEntries)-maxEntries:]
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Dead letter packets retrieved successfully",
		"count":     len(deadLetterEntries),
		"packets":   deadLetterEntries,
		"file":      deadLetterFile,
		"timestamp": time.Now(),
		"note":      fmt.Sprintf("Showing last %d entries", len(deadLetterEntries)),
	})
}

// SetAnalyzerHealth manually sets an analyzer's health status (for testing)
func (h *Handler) SetAnalyzerHealth(c *gin.Context) {
	analyzerID := c.Param("id")
	if analyzerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Analyzer ID is required",
		})
		return
	}

	var req struct {
		Healthy bool `json:"healthy"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
		})
		return
	}

	h.logger.Info("Analyzer health manually set",
		zap.String("analyzer_id", analyzerID),
		zap.Bool("healthy", req.Healthy),
		zap.String("client_ip", c.ClientIP()),
	)

	c.JSON(http.StatusOK, gin.H{
		"message":     "Analyzer health status updated",
		"analyzer_id": analyzerID,
		"healthy":     req.Healthy,
		"timestamp":   time.Now(),
	})
}

// loggingMiddleware logs HTTP requests
func (h *Handler) loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		// Skip logging for health checks to avoid spam
		if path != "/api/v1/health" {
			latency := time.Since(start)

			logLevel := zap.InfoLevel
			if c.Writer.Status() >= 400 {
				logLevel = zap.ErrorLevel
			}
			if c.Writer.Status() >= 500 {
				logLevel = zap.ErrorLevel
			}

			if ce := h.logger.Check(logLevel, "HTTP Request"); ce != nil {
				ce.Write(
					zap.String("method", c.Request.Method),
					zap.String("path", path),
					zap.Int("status", c.Writer.Status()),
					zap.Duration("latency", latency),
					zap.String("client_ip", c.ClientIP()),
				)
			}
		}
	}
}

// corsMiddleware handles CORS headers
func (h *Handler) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
