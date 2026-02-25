package services

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/google/uuid"

	"github.com/inaiurai/backend/internal/models"
)

// Default credits per capability from CAPABILITY SPECS (.cursorrules).
var defaultPriceByCapability = map[string]int{
	"Research":        8,
	"Summarize":        3,
	"Data Extraction": 5,
}

// AgentRepo is the minimal interface required for matching.
type AgentRepo interface {
	FindAvailableWorkers(ctx context.Context, capability string) ([]*models.Agent, error)
}

// Matcher finds workers for tasks based on budget and routing preference.
type Matcher struct {
	AgentRepo AgentRepo
}

// NewMatcher returns a new Matcher.
func NewMatcher(agentRepo AgentRepo) *Matcher {
	return &Matcher{AgentRepo: agentRepo}
}

// workerCandidate holds an agent and its price for the task's capability.
type workerCandidate struct {
	agent           *models.Agent
	pricingPerTask  int
	schemaCompliance float64 // 0–1
	successRate     float64 // 0–1 (default 0.5 when unknown)
	reputation      float64 // 0–1 (default 0.5 when unknown)
	avgResponseMs   int     // 0 when unknown
}

// getPricingPerTask reads price for the given capability from capabilities_offered JSONB, or uses default.
func getPricingPerTask(capabilitiesOffered json.RawMessage, capability string) int {
	if len(capabilitiesOffered) == 0 {
		return defaultPriceByCapability[capability]
	}
	var m map[string]struct {
		Price int `json:"price"`
	}
	if err := json.Unmarshal(capabilitiesOffered, &m); err != nil {
		return defaultPriceByCapability[capability]
	}
	if c, ok := m[capability]; ok && c.Price > 0 {
		return c.Price
	}
	return defaultPriceByCapability[capability]
}

// buildCandidates filters workers by budget and builds candidates with scoring fields.
func buildCandidates(workers []*models.Agent, task *models.Task, excludeID uuid.UUID) []workerCandidate {
	var candidates []workerCandidate
	for _, ag := range workers {
		if ag.ID == excludeID {
			continue
		}
		price := getPricingPerTask(ag.CapabilitiesOffered, task.CapabilityRequired)
		if price > task.Budget {
			continue
		}
		compl := 0.0
		if ag.SchemaComplianceRate != nil {
			compl = float64(*ag.SchemaComplianceRate)
			if compl > 1 {
				compl = 1
			}
		}
		ms := 0
		if ag.AvgResponseTimeMs != nil {
			ms = *ag.AvgResponseTimeMs
		}
		wc := workerCandidate{
			agent:            ag,
			pricingPerTask:   price,
			schemaCompliance: compl,
			successRate:      0.5,
			reputation:       0.5,
			avgResponseMs:   ms,
		}
		candidates = append(candidates, wc)
	}
	return candidates
}

func routingPreference(task *models.Task) string {
	p := task.RoutingPreference
	if p == "" {
		return models.RoutingAuto
	}
	if p != models.RoutingFastest && p != models.RoutingCheapest && p != models.RoutingAuto {
		return models.RoutingAuto
	}
	return p
}

// scoreAndSort sorts candidates by the task's routing preference (best first).
func scoreAndSort(candidates []workerCandidate, task *models.Task) {
	pref := routingPreference(task)
	switch pref {
	case models.RoutingFastest:
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].avgResponseMs < candidates[j].avgResponseMs
		})
		return
	case models.RoutingCheapest:
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].pricingPerTask < candidates[j].pricingPerTask
		})
		return
	}

	// "auto": weighted score
	maxMs := 0
	for i := range candidates {
		if candidates[i].avgResponseMs > maxMs {
			maxMs = candidates[i].avgResponseMs
		}
	}
	if maxMs <= 0 {
		maxMs = 1
	}
	maxPrice := 0
	for i := range candidates {
		if candidates[i].pricingPerTask > maxPrice {
			maxPrice = candidates[i].pricingPerTask
		}
	}
	if maxPrice <= 0 {
		maxPrice = 1
	}
	scores := make([]float64, len(candidates))
	for i := range candidates {
		c := &candidates[i]
		speedNorm := 0.0
		if maxMs > 0 {
			speedNorm = 1.0 - (float64(c.avgResponseMs) / float64(maxMs))
		}
		priceNorm := 0.0
		if maxPrice > 0 {
			priceNorm = 1.0 - (float64(c.pricingPerTask) / float64(maxPrice))
		}
		scores[i] = c.schemaCompliance*0.20 + c.successRate*0.25 + c.reputation*0.25 + speedNorm*0.15 + priceNorm*0.15
	}
	sort.Slice(candidates, func(i, j int) bool {
		return scores[i] > scores[j]
	})
}

// FindBestWorker returns the single best worker for the task, or nil if none.
func (m *Matcher) FindBestWorker(ctx context.Context, task *models.Task) (*models.Agent, error) {
	workers, err := m.AgentRepo.FindAvailableWorkers(ctx, task.CapabilityRequired)
	if err != nil {
		return nil, err
	}
	candidates := buildCandidates(workers, task, uuid.Nil)
	if len(candidates) == 0 {
		return nil, nil
	}
	scoreAndSort(candidates, task)
	return candidates[0].agent, nil
}

// FindFallbacks returns up to 2 alternative workers for the task, excluding the given agent ID.
func (m *Matcher) FindFallbacks(ctx context.Context, task *models.Task, excludeAgentID uuid.UUID) ([]*models.Agent, error) {
	workers, err := m.AgentRepo.FindAvailableWorkers(ctx, task.CapabilityRequired)
	if err != nil {
		return nil, err
	}
	candidates := buildCandidates(workers, task, excludeAgentID)
	if len(candidates) == 0 {
		return nil, nil
	}
	scoreAndSort(candidates, task)
	max := 2
	if len(candidates) < max {
		max = len(candidates)
	}
	out := make([]*models.Agent, 0, max)
	for i := 0; i < max; i++ {
		out = append(out, candidates[i].agent)
	}
	return out, nil
}
