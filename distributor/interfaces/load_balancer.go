package interfaces

import "logs-distributor/models"

// LoadBalancer defines the interface for analyzer selection and weight management
type LoadBalancer interface {
	SelectAnalyzer() *models.Analyzer
	UpdateWeights()
}
