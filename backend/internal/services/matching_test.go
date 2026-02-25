package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/inaiurai/backend/internal/models"
)

// ---------------------------------------------------------------------------
// Mock AgentRepo
// ---------------------------------------------------------------------------

// mockAgentRepo holds a static list of agents and reproduces the production
// filtering contract: exclude system accounts, require online + worker role.
type mockAgentRepo struct {
	agents []*models.Agent
	// systemAccountIDs contains account IDs that should be treated as system
	// accounts (mirroring the SQL WHERE ac.is_system_account = FALSE clause).
	systemAccountIDs map[uuid.UUID]bool
}

func (m *mockAgentRepo) FindAvailableWorkers(_ context.Context, capability string) ([]*models.Agent, error) {
	var out []*models.Agent
	for _, ag := range m.agents {
		if m.systemAccountIDs[ag.AccountID] {
			continue
		}
		if ag.Availability != models.AgentAvailabilityOnline {
			continue
		}
		if ag.Role != models.AgentRoleWorker && ag.Role != models.AgentRoleBoth {
			continue
		}
		out = append(out, ag)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func float32Ptr(f float32) *float32 { return &f }

func capJSON(capability string, price int) json.RawMessage {
	m := map[string]any{capability: map[string]any{"price": price}}
	b, _ := json.Marshal(m)
	return b
}

func makeAgent(accountID uuid.UUID, price, avgMs int) *models.Agent {
	return &models.Agent{
		ID:                  uuid.New(),
		AccountID:           accountID,
		Role:                models.AgentRoleWorker,
		Availability:        models.AgentAvailabilityOnline,
		CapabilitiesOffered: capJSON("research", price),
		AvgResponseTimeMs:   intPtr(avgMs),
		SchemaComplianceRate: float32Ptr(0.9),
	}
}

// ---------------------------------------------------------------------------
// 1. TestBudgetFilter
// ---------------------------------------------------------------------------

func TestBudgetFilter(t *testing.T) {
	cheap := makeAgent(uuid.New(), 5, 100)
	exact := makeAgent(uuid.New(), 10, 200)
	expensive := makeAgent(uuid.New(), 15, 50)

	repo := &mockAgentRepo{
		agents:           []*models.Agent{cheap, exact, expensive},
		systemAccountIDs: map[uuid.UUID]bool{},
	}
	matcher := NewMatcher(repo)

	task := &models.Task{
		ID:                 uuid.New(),
		CapabilityRequired: "research",
		Budget:             10,
		RoutingPreference:  models.RoutingAuto,
	}

	best, err := matcher.FindBestWorker(context.Background(), task)
	if err != nil {
		t.Fatalf("FindBestWorker: %v", err)
	}
	if best == nil {
		t.Fatal("expected a worker, got nil")
	}

	// The expensive agent (price 15) must never be selected with budget 10.
	if best.ID == expensive.ID {
		t.Fatal("expensive agent should be excluded by budget filter")
	}

	// Also verify via FindFallbacks that expensive never appears.
	fallbacks, err := matcher.FindFallbacks(context.Background(), task, uuid.Nil)
	if err != nil {
		t.Fatalf("FindFallbacks: %v", err)
	}
	for _, fb := range fallbacks {
		if fb.ID == expensive.ID {
			t.Fatal("expensive agent should not appear in fallbacks")
		}
	}

	// With a budget of 0, no workers should match.
	task.Budget = 0
	none, err := matcher.FindBestWorker(context.Background(), task)
	if err != nil {
		t.Fatalf("FindBestWorker zero budget: %v", err)
	}
	if none != nil {
		t.Fatal("expected nil worker with zero budget")
	}
}

// ---------------------------------------------------------------------------
// 2. TestRoutingFastest
// ---------------------------------------------------------------------------

func TestRoutingFastest(t *testing.T) {
	slow := makeAgent(uuid.New(), 5, 500)
	medium := makeAgent(uuid.New(), 5, 200)
	fast := makeAgent(uuid.New(), 5, 50)

	repo := &mockAgentRepo{
		agents:           []*models.Agent{slow, medium, fast},
		systemAccountIDs: map[uuid.UUID]bool{},
	}
	matcher := NewMatcher(repo)

	task := &models.Task{
		ID:                 uuid.New(),
		CapabilityRequired: "research",
		Budget:             100,
		RoutingPreference:  models.RoutingFastest,
	}

	best, err := matcher.FindBestWorker(context.Background(), task)
	if err != nil {
		t.Fatalf("FindBestWorker: %v", err)
	}
	if best == nil {
		t.Fatal("expected a worker, got nil")
	}
	if best.ID != fast.ID {
		t.Errorf("expected fastest agent (%s, 50ms), got agent %s (%dms)",
			fast.ID, best.ID, *best.AvgResponseTimeMs)
	}
}

// ---------------------------------------------------------------------------
// 3. TestExcludesSystemAccounts
//    The production AgentRepo.FindAvailableWorkers filters out agents whose
//    account has is_system_account = TRUE. Our mock reproduces this contract.
//    We verify that even when system-account agents are present in the
//    underlying data, they never surface through FindBestWorker.
// ---------------------------------------------------------------------------

func TestExcludesSystemAccounts(t *testing.T) {
	platformAgent := makeAgent(models.SystemPlatformAccountID, 5, 10)
	adminAgent := makeAgent(models.AdminAccountID, 5, 20)
	normalAgent := makeAgent(uuid.New(), 5, 100)

	repo := &mockAgentRepo{
		agents: []*models.Agent{platformAgent, adminAgent, normalAgent},
		systemAccountIDs: map[uuid.UUID]bool{
			models.SystemPlatformAccountID: true,
			models.AdminAccountID:          true,
		},
	}
	matcher := NewMatcher(repo)

	task := &models.Task{
		ID:                 uuid.New(),
		CapabilityRequired: "research",
		Budget:             100,
		RoutingPreference:  models.RoutingFastest,
	}

	best, err := matcher.FindBestWorker(context.Background(), task)
	if err != nil {
		t.Fatalf("FindBestWorker: %v", err)
	}
	if best == nil {
		t.Fatal("expected at least the normal worker")
	}
	if best.AccountID == models.SystemPlatformAccountID {
		t.Fatal("SystemPlatformAccountID must never be returned as a worker")
	}
	if best.AccountID == models.AdminAccountID {
		t.Fatal("AdminAccountID must never be returned as a worker")
	}
	if best.ID != normalAgent.ID {
		t.Errorf("expected normal agent %s, got %s", normalAgent.ID, best.ID)
	}

	// With only system-account agents, result should be nil.
	repoSysOnly := &mockAgentRepo{
		agents: []*models.Agent{platformAgent, adminAgent},
		systemAccountIDs: map[uuid.UUID]bool{
			models.SystemPlatformAccountID: true,
			models.AdminAccountID:          true,
		},
	}
	matcherSysOnly := NewMatcher(repoSysOnly)
	none, err := matcherSysOnly.FindBestWorker(context.Background(), task)
	if err != nil {
		t.Fatalf("FindBestWorker system-only: %v", err)
	}
	if none != nil {
		t.Fatal("expected nil when only system-account agents exist")
	}
}
