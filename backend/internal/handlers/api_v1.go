package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/DanielMartin-A/InaiUrai/backend/internal/middleware"
	"github.com/DanielMartin-A/InaiUrai/backend/internal/models"
	"github.com/DanielMartin-A/InaiUrai/backend/internal/repository"
	"github.com/DanielMartin-A/InaiUrai/backend/internal/services"
	"github.com/google/uuid"
)

type APIv1Handler struct {
	orgRepo        *repository.OrgRepo
	memberRepo     *repository.MemberRepo
	engagementRepo *repository.EngagementRepo
	taskRepo       *repository.TaskRepo
	engine         *services.EngineClient
	engagementSvc  *services.EngagementService
	taskMgr        *services.TaskManager
}

func NewAPIv1Handler(or *repository.OrgRepo, mr *repository.MemberRepo, er *repository.EngagementRepo, tr *repository.TaskRepo, ec *services.EngineClient, es *services.EngagementService, tm *services.TaskManager) *APIv1Handler {
	return &APIv1Handler{orgRepo: or, memberRepo: mr, engagementRepo: er, taskRepo: tr, engine: ec, engagementSvc: es, taskMgr: tm}
}

func (h *APIv1Handler) CreateEngagement(w http.ResponseWriter, r *http.Request) {
	member := r.Context().Value(middleware.MemberContextKey).(*models.Member)
	var req struct {
		Objective string `json:"objective"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", 400)
		return
	}

	org, _ := h.orgRepo.GetByID(r.Context(), member.OrgID)
	if org == nil {
		http.Error(w, "org not found", 404)
		return
	}

	result, err := h.engine.Orchestrate(r.Context(), req.Objective, "", org.ID.String(), member.ID.String())
	if err != nil {
		slog.Error("orchestrate failed", "error", err)
		http.Error(w, "orchestration failed", 500)
		return
	}

	rolesJSON, _ := json.Marshal(result["team"])
	planJSON, _ := json.Marshal(result["execution_plan"])
	hbJSON, _ := json.Marshal(result["heartbeat_config"])

	eng := &models.Engagement{
		OrgID: org.ID, CreatedBy: &member.ID, Objective: req.Objective,
		EngagementType: result["engagement_type"].(string), Status: "planning",
		Roles: rolesJSON, ExecutionPlan: planJSON, HeartbeatConfig: hbJSON,
	}
	if err := h.engagementRepo.Create(r.Context(), eng); err != nil {
		http.Error(w, "failed to create engagement", 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"engagement": eng, "plan": result,
	})
}

func (h *APIv1Handler) ListEngagements(w http.ResponseWriter, r *http.Request) {
	member := r.Context().Value(middleware.MemberContextKey).(*models.Member)
	status := r.URL.Query().Get("status")
	engagements, err := h.engagementRepo.ListByOrg(r.Context(), member.OrgID, status)
	if err != nil {
		http.Error(w, "internal error", 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"engagements": engagements})
}

func (h *APIv1Handler) GetEngagement(w http.ResponseWriter, r *http.Request) {
	member := r.Context().Value(middleware.MemberContextKey).(*models.Member)
	id, _ := uuid.Parse(r.PathValue("id"))
	eng, err := h.engagementRepo.GetByID(r.Context(), id)
	if err != nil || eng == nil {
		http.Error(w, "not found", 404)
		return
	}
	if eng.OrgID != member.OrgID {
		http.Error(w, "not found", 404)
		return
	}
	json.NewEncoder(w).Encode(eng)
}

func (h *APIv1Handler) ListTasks(w http.ResponseWriter, r *http.Request) {
	member := r.Context().Value(middleware.MemberContextKey).(*models.Member)
	id, _ := uuid.Parse(r.PathValue("id"))
	eng, err := h.engagementRepo.GetByID(r.Context(), id)
	if err != nil || eng == nil || eng.OrgID != member.OrgID {
		http.Error(w, "not found", 404)
		return
	}
	tasks, err := h.taskRepo.ListByEngagement(r.Context(), id)
	if err != nil {
		http.Error(w, "internal error", 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"tasks": tasks})
}

func (h *APIv1Handler) GetTrace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	json.NewEncoder(w).Encode(map[string]string{"message": "Use engine /v1/trace/" + id + " directly or implement proxy"})
}

func (h *APIv1Handler) CreateTask(w http.ResponseWriter, r *http.Request) {
	member := r.Context().Value(middleware.MemberContextKey).(*models.Member)
	var req struct {
		InputText    string `json:"input_text"`
		EngagementID string `json:"engagement_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", 400)
		return
	}

	org, _ := h.orgRepo.GetByID(r.Context(), member.OrgID)
	if org == nil {
		http.Error(w, "org not found", 404)
		return
	}

	result, err := h.taskMgr.ExecuteSoloTask(r.Context(), org, member, req.InputText)
	if err != nil {
		http.Error(w, "task failed", 500)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"output": result})
}

func (h *APIv1Handler) GetOrg(w http.ResponseWriter, r *http.Request) {
	member := r.Context().Value(middleware.MemberContextKey).(*models.Member)
	org, _ := h.orgRepo.GetByID(r.Context(), member.OrgID)
	if org == nil {
		http.Error(w, "not found", 404)
		return
	}
	json.NewEncoder(w).Encode(org)
}

func (h *APIv1Handler) ListRoles(w http.ResponseWriter, r *http.Request) {
	roles := []map[string]string{
		{"slug": "chief-of-staff", "title": "Chief of Staff", "division": "Operations & Execution"},
		{"slug": "cmo", "title": "Chief Marketing Officer", "division": "Growth & Revenue"},
		{"slug": "cro", "title": "Chief Revenue Officer", "division": "Growth & Revenue"},
		{"slug": "cbo", "title": "Chief Brand Officer", "division": "Growth & Revenue"},
		{"slug": "cfo", "title": "Chief Financial Officer", "division": "Finance & Intelligence"},
		{"slug": "cio", "title": "Chief Intelligence Officer", "division": "Finance & Intelligence"},
		{"slug": "researcher", "title": "Chief Research Officer", "division": "Finance & Intelligence"},
		{"slug": "coo", "title": "Chief Operating Officer", "division": "Operations & Execution"},
		{"slug": "cpo", "title": "Chief People Officer", "division": "Operations & Execution"},
		{"slug": "cco", "title": "Chief Communications Officer", "division": "Communications & Content"},
		{"slug": "content-chief", "title": "Chief Content Officer", "division": "Communications & Content"},
		{"slug": "creative-chief", "title": "Chief Creative Officer", "division": "Communications & Content"},
		{"slug": "general-counsel", "title": "General Counsel", "division": "Legal & Technical"},
		{"slug": "cto", "title": "Chief Technology Officer", "division": "Legal & Technical"},
		{"slug": "cdo", "title": "Chief Data Officer", "division": "Legal & Technical"},
		{"slug": "product-chief", "title": "Chief Product Officer", "division": "Legal & Technical"},
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"roles": roles})
}
