package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/inaiurai/backend/internal/middleware"
	"github.com/inaiurai/backend/internal/models"
	"github.com/inaiurai/backend/internal/services"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

// --- noopTx satisfies pgx.Tx for test use; only Commit/Rollback are called. ---

type noopTx struct{}

func (noopTx) Begin(context.Context) (pgx.Tx, error)       { return noopTx{}, nil }
func (noopTx) Commit(context.Context) error                 { return nil }
func (noopTx) Rollback(context.Context) error               { return nil }
func (noopTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag(""), nil
}
func (noopTx) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, nil }
func (noopTx) QueryRow(context.Context, string, ...any) pgx.Row        { return nil }
func (noopTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (noopTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return nil }
func (noopTx) LargeObjects() pgx.LargeObjects                         { return pgx.LargeObjects{} }
func (noopTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (noopTx) Conn() *pgx.Conn { return nil }

// --- TxBeginner mock ---

type mockPool struct{}

func (mockPool) Begin(context.Context) (pgx.Tx, error) { return noopTx{}, nil }

// --- TaskRepo mock ---

type mockTaskRepo struct {
	tasks map[uuid.UUID]*models.Task
}

func newMockTaskRepo() *mockTaskRepo { return &mockTaskRepo{tasks: make(map[uuid.UUID]*models.Task)} }

func (m *mockTaskRepo) Create(_ context.Context, t *models.Task) error {
	m.tasks[t.ID] = t
	return nil
}
func (m *mockTaskRepo) GetByID(_ context.Context, id uuid.UUID) (*models.Task, error) {
	t, ok := m.tasks[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return t, nil
}
func (m *mockTaskRepo) Update(_ context.Context, t *models.Task) error {
	m.tasks[t.ID] = t
	return nil
}

// --- AgentRepo mock ---

type mockAgentRepo struct {
	agents map[uuid.UUID]*models.Agent
}

func newMockAgentRepo() *mockAgentRepo {
	return &mockAgentRepo{agents: make(map[uuid.UUID]*models.Agent)}
}

func (m *mockAgentRepo) GetByID(_ context.Context, id uuid.UUID) (*models.Agent, error) {
	ag, ok := m.agents[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return ag, nil
}

// --- Escrow mock: records calls ---

type mockEscrow struct {
	lockCalled   bool
	settleCalled bool
	refundCalled bool
}

func (m *mockEscrow) LockCredits(context.Context, pgx.Tx, uuid.UUID, uuid.UUID, int) error {
	m.lockCalled = true
	return nil
}
func (m *mockEscrow) SettleTask(_ context.Context, _ pgx.Tx, _, _, _ uuid.UUID, _, _ int) error {
	m.settleCalled = true
	return nil
}
func (m *mockEscrow) RefundFailed(_ context.Context, _ pgx.Tx, _, _ uuid.UUID, _ int) error {
	m.refundCalled = true
	return nil
}

// --- Dispatcher mock ---

type mockDispatcher struct {
	dispatched bool
}

func (m *mockDispatcher) DispatchTask(context.Context, *models.Task) error {
	m.dispatched = true
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func schemasDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "schemas")
}

func newTestValidator(t *testing.T) *services.Validator {
	t.Helper()
	v, err := services.NewValidator(context.Background(), schemasDir(t))
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}
	return v
}

func newTestHandler(t *testing.T) (*TaskHandler, *mockTaskRepo, *mockAgentRepo, *mockEscrow, *mockDispatcher) {
	t.Helper()
	tr := newMockTaskRepo()
	ar := newMockAgentRepo()
	esc := &mockEscrow{}
	disp := &mockDispatcher{}
	h := &TaskHandler{
		Pool:       mockPool{},
		TaskRepo:   tr,
		AgentRepo:  ar,
		Escrow:     esc,
		Dispatcher: disp,
		Validator:  newTestValidator(t),
		Logger:     slog.Default(),
	}
	return h, tr, ar, esc, disp
}

// injectCtx sets account and optionally agent into the request context.
func injectCtx(r *http.Request, acc *models.Account, ag *models.Agent) *http.Request {
	ctx := r.Context()
	ctx = middleware.WithAccount(ctx, acc)
	if ag != nil {
		ctx = middleware.WithAgent(ctx, ag)
	}
	return r.WithContext(ctx)
}

// =====================================================================
// POST /v1/tasks
// =====================================================================

func TestCreateTask_ValidPayload(t *testing.T) {
	h, _, _, esc, _ := newTestHandler(t)

	agentID := uuid.New()
	acc := &models.Account{ID: uuid.New(), CreditBalance: 1000}

	body := fmt.Sprintf(`{
		"requester_agent_id": %q,
		"capability_required": "research",
		"input_payload": {"query":"AI trends","depth":"standard","max_sources":5},
		"budget": 10
	}`, agentID)

	req := httptest.NewRequest(http.MethodPost, "/v1/tasks", strings.NewReader(body))
	req = injectCtx(req, acc, nil)
	rec := httptest.NewRecorder()

	h.CreateTask(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp createTaskResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.TaskID == "" {
		t.Error("response missing task_id")
	}
	if !esc.lockCalled {
		t.Error("expected Escrow.LockCredits to be called")
	}
}

func TestCreateTask_InvalidSchema(t *testing.T) {
	h, _, _, _, _ := newTestHandler(t)

	agentID := uuid.New()
	acc := &models.Account{ID: uuid.New(), CreditBalance: 1000}

	// Missing "depth" and "max_sources" — required by research input schema.
	body := fmt.Sprintf(`{
		"requester_agent_id": %q,
		"capability_required": "research",
		"input_payload": {"query":"AI trends"},
		"budget": 10
	}`, agentID)

	req := httptest.NewRequest(http.MethodPost, "/v1/tasks", strings.NewReader(body))
	req = injectCtx(req, acc, nil)
	rec := httptest.NewRecorder()

	h.CreateTask(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateTask_ZeroBudget(t *testing.T) {
	h, _, _, _, _ := newTestHandler(t)

	agentID := uuid.New()
	acc := &models.Account{ID: uuid.New()}

	body := fmt.Sprintf(`{
		"requester_agent_id": %q,
		"capability_required": "research",
		"input_payload": {"query":"AI trends","depth":"quick","max_sources":1},
		"budget": 0
	}`, agentID)

	req := httptest.NewRequest(http.MethodPost, "/v1/tasks", strings.NewReader(body))
	req = injectCtx(req, acc, nil)
	rec := httptest.NewRecorder()

	h.CreateTask(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

// =====================================================================
// POST /v1/tasks/{id}/result
// =====================================================================

func seedTask(tr *mockTaskRepo, ar *mockAgentRepo) (task *models.Task, workerAgent *models.Agent) {
	reqAccID := uuid.New()
	reqAgentID := uuid.New()
	wrkAccID := uuid.New()
	wrkAgentID := uuid.New()

	ar.agents[reqAgentID] = &models.Agent{ID: reqAgentID, AccountID: reqAccID}
	ar.agents[wrkAgentID] = &models.Agent{ID: wrkAgentID, AccountID: wrkAccID}

	task = &models.Task{
		ID:                 uuid.New(),
		RequesterAgentID:   reqAgentID,
		WorkerAgentID:      &wrkAgentID,
		CapabilityRequired: "research",
		Budget:             100,
		Status:             models.TaskStatusInProgress,
	}
	tr.tasks[task.ID] = task

	return task, ar.agents[wrkAgentID]
}

func TestSubmitResult_ValidSuccess(t *testing.T) {
	h, tr, ar, esc, _ := newTestHandler(t)
	task, workerAgent := seedTask(tr, ar)

	body := `{
		"output_payload": {
			"status":"success",
			"findings":"AI is growing.",
			"key_points":["LLMs"],
			"sources":[{"title":"Paper","url":"https://example.com","relevance":0.9}]
		},
		"output_status": "success",
		"actual_cost": 80
	}`

	url := fmt.Sprintf("/v1/tasks/%s/result", task.ID)
	req := httptest.NewRequest(http.MethodPost, url, strings.NewReader(body))
	acc := &models.Account{ID: workerAgent.AccountID}
	req = injectCtx(req, acc, workerAgent)
	rec := httptest.NewRecorder()

	h.SubmitResult(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !esc.settleCalled {
		t.Error("expected SettleTask to be called")
	}
}

func TestSubmitResult_ErrorResult(t *testing.T) {
	h, tr, ar, esc, _ := newTestHandler(t)
	task, workerAgent := seedTask(tr, ar)

	body := `{
		"output_payload": {
			"status":"error",
			"error":{"code":"TIMEOUT","message":"took too long"}
		},
		"output_status": "error",
		"actual_cost": 0
	}`

	url := fmt.Sprintf("/v1/tasks/%s/result", task.ID)
	req := httptest.NewRequest(http.MethodPost, url, strings.NewReader(body))
	acc := &models.Account{ID: workerAgent.AccountID}
	req = injectCtx(req, acc, workerAgent)
	rec := httptest.NewRecorder()

	h.SubmitResult(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !esc.refundCalled {
		t.Error("expected RefundFailed to be called")
	}
	if esc.settleCalled {
		t.Error("SettleTask should NOT be called on error result")
	}
}

func TestSubmitResult_WrongWorker(t *testing.T) {
	h, tr, ar, _, _ := newTestHandler(t)
	task, _ := seedTask(tr, ar)

	imposterAgent := &models.Agent{ID: uuid.New(), AccountID: uuid.New()}

	body := `{
		"output_payload": {"status":"success","findings":"x","key_points":[],"sources":[]},
		"output_status": "success",
		"actual_cost": 50
	}`

	url := fmt.Sprintf("/v1/tasks/%s/result", task.ID)
	req := httptest.NewRequest(http.MethodPost, url, strings.NewReader(body))
	acc := &models.Account{ID: imposterAgent.AccountID}
	req = injectCtx(req, acc, imposterAgent)
	rec := httptest.NewRecorder()

	h.SubmitResult(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

// =====================================================================
// GET /v1/capabilities
// =====================================================================

func TestListCapabilities(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/capabilities", nil)
	rec := httptest.NewRecorder()

	ListCapabilities(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var caps []capabilityInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &caps); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(caps) != 3 {
		t.Fatalf("expected 3 capabilities, got %d", len(caps))
	}

	names := map[string]bool{}
	for _, c := range caps {
		names[c.Name] = true
	}
	for _, want := range []string{"research", "summarize", "data_extraction"} {
		if !names[want] {
			t.Errorf("missing capability %q", want)
		}
	}
}
