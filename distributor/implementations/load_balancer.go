package implementations

import (
	"logs-distributor/distributor/interfaces"
	"logs-distributor/models"
	"sync"

	"go.uber.org/zap"
)

// WeightedLoadBalancer implements the LoadBalancer interface
type WeightedLoadBalancer struct {
	analyzers       map[string]*models.Analyzer
	logger          *zap.Logger
	mu              sync.RWMutex
	currentWeights  map[string]float64
	originalWeights map[string]float64
}

// Ensure WeightedLoadBalancer implements LoadBalancer interface
var _ interfaces.LoadBalancer = (*WeightedLoadBalancer)(nil)

func NewLoadBalancer(analyzers map[string]*models.Analyzer, logger *zap.Logger) interfaces.LoadBalancer {
	lb := &WeightedLoadBalancer{
		analyzers:       analyzers,
		logger:          logger,
		currentWeights:  make(map[string]float64),
		originalWeights: make(map[string]float64),
	}

	// Initialize weights
	for _, analyzer := range analyzers {
		lb.currentWeights[analyzer.ID] = analyzer.Weight
		lb.originalWeights[analyzer.ID] = analyzer.Weight
	}

	return lb
}

// SelectAnalyzer selects an analyzer using weighted round-robin load balancing
func (lb *WeightedLoadBalancer) SelectAnalyzer() *models.Analyzer {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if len(lb.analyzers) == 0 {
		return nil
	}

	// Find healthy analyzers and calculate total weight in single pass
	var healthyAnalyzers []*models.Analyzer
	var totalWeight float64

	for _, analyzer := range lb.analyzers {
		if analyzer.IsHealthy {
			healthyAnalyzers = append(healthyAnalyzers, analyzer)
			totalWeight += lb.originalWeights[analyzer.ID]
		}
	}

	if len(healthyAnalyzers) == 0 || totalWeight == 0 {
		return nil
	}

	// Update current weights for all healthy analyzers
	for _, analyzer := range healthyAnalyzers {
		lb.currentWeights[analyzer.ID] += lb.originalWeights[analyzer.ID]
	}

	// Find analyzer with highest current weight
	var selectedAnalyzer *models.Analyzer
	var maxWeight float64 = -1

	for _, analyzer := range healthyAnalyzers {
		currentWeight := lb.currentWeights[analyzer.ID]
		if currentWeight > maxWeight {
			maxWeight = currentWeight
			selectedAnalyzer = analyzer
		}
	}

	// Decrease selected analyzer's current weight by total weight
	if selectedAnalyzer != nil {
		lb.currentWeights[selectedAnalyzer.ID] -= totalWeight
	}

	return selectedAnalyzer
}

// UpdateWeights resets weights when health changes
func (lb *WeightedLoadBalancer) UpdateWeights() {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	for _, analyzer := range lb.analyzers {
		if analyzer.IsHealthy {
			// Reset weight for recovered analyzers
			lb.currentWeights[analyzer.ID] = lb.originalWeights[analyzer.ID]
		}
	}
}
