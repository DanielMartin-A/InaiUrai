package services

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/DanielMartin-A/InaiUrai/backend/internal/models"
	"github.com/DanielMartin-A/InaiUrai/backend/internal/repository"
	"github.com/google/uuid"
)

type ProgressBroadcaster interface {
	SendToMember(memberID uuid.UUID, msg []byte)
}

type TaskManager struct {
	taskRepo      *repository.TaskRepo
	orgRepo       *repository.OrgRepo
	memberRepo    *repository.MemberRepo
	engine        *EngineClient
	engagementSvc *EngagementService
	broadcaster   ProgressBroadcaster
	delivery      *DeliveryRouter
}

func NewTaskManager(tr *repository.TaskRepo, or *repository.OrgRepo, mr *repository.MemberRepo, ec *EngineClient, es *EngagementService) *TaskManager {
	return &TaskManager{taskRepo: tr, orgRepo: or, memberRepo: mr, engine: ec, engagementSvc: es}
}

func (tm *TaskManager) SetBroadcaster(b ProgressBroadcaster) { tm.broadcaster = b }
func (tm *TaskManager) SetDeliveryRouter(d *DeliveryRouter)   { tm.delivery = d }

func (tm *TaskManager) ExecuteSoloTask(ctx context.Context, org *models.Organization, member *models.Member, inputText string) (string, error) {
	if !org.CanCreateTask() {
		return "You've reached your task limit for this billing period. Upgrade your plan or wait for renewal.", nil
	}

	soul, _ := tm.engagementSvc.contextRepo.GetOrgSoul(ctx, org.ID)

	roleSlug, reasoning, err := tm.engine.Route(ctx, inputText, soul)
	if err != nil {
		slog.Warn("route failed, using chief-of-staff", "error", err, "org_id", org.ID)
		roleSlug = "chief-of-staff"
	}
	slog.Info("routed solo task", "role", roleSlug, "reasoning", reasoning, "org_id", org.ID)

	engRoles, _ := json.Marshal([]models.EngagementRole{{RoleSlug: roleSlug, Purpose: "solo task"}})
	engagement := &models.Engagement{
		OrgID: org.ID, CreatedBy: &member.ID, Objective: inputText,
		EngagementType: "task", Status: "active", Roles: engRoles,
	}
	if err := tm.engagementSvc.engagementRepo.Create(ctx, engagement); err != nil {
		slog.Error("failed to create engagement", "error", err)
		return "Sorry, something went wrong. Please try again.", nil
	}

	task := &models.Task{
		OrgID: &org.ID, MemberID: &member.ID, EngagementID: &engagement.ID,
		RoleSlug: roleSlug, InputText: inputText, Status: "created",
	}
	if err := tm.taskRepo.Create(ctx, task); err != nil {
		slog.Error("failed to create task", "error", err)
		return "Sorry, something went wrong. Please try again.", nil
	}

	memberProfile, _ := tm.engagementSvc.contextRepo.GetMemberProfile(ctx, member.ID)

	engineReq := &models.EngineRequest{
		TaskID:        task.ID.String(),
		InputText:     inputText,
		OrgContext:    map[string]interface{}{},
		OrgSoul:       soul,
		MemberProfile: memberProfile,
		Role:          roleSlug,
		Tier:          org.TierSlug(),
		EngagementID:  engagement.ID.String(),
		OrgID:         org.ID.String(),
		MemberID:      member.ID.String(),
	}

	go tm.executeAsync(org, member, task, engagement, engineReq)

	tm.broadcastProgress(member.ID, task.ID, "routing", roleSlug, "Routed to "+roleSlug)

	return "", nil
}

func (tm *TaskManager) executeAsync(org *models.Organization, member *models.Member, task *models.Task, engagement *models.Engagement, req *models.EngineRequest) {
	ctx := context.Background()

	result, err := tm.engine.RunTask(ctx, req)
	if err != nil {
		slog.Error("engine execution failed", "error", err, "task_id", task.ID)
		tm.taskRepo.Fail(ctx, task.ID, err.Error())
		tm.engagementSvc.engagementRepo.UpdateStatus(ctx, engagement.ID, "completed")
		if tm.delivery != nil {
			tm.delivery.Deliver(ctx, member, task.ID, "", "Something went wrong. Please try again.")
		} else {
			tm.broadcastProgress(member.ID, task.ID, "error", "", "Something went wrong. Please try again.")
		}
		return
	}

	entitiesJSON, _ := json.Marshal(result.ExtractedEntities)
	tm.taskRepo.Complete(ctx, task.ID, result.OutputText, result.QualityScore, entitiesJSON, result.ProcessingTimeMs)
	tm.orgRepo.IncrementTaskCount(ctx, org.ID)
	tm.engagementSvc.engagementRepo.UpdateStatus(ctx, engagement.ID, "completed")

	if tm.delivery != nil {
		tm.delivery.Deliver(ctx, member, task.ID, req.Role, result.OutputText)
	} else {
		tm.broadcastProgress(member.ID, task.ID, "done", req.Role, result.OutputText)
	}
}

func (tm *TaskManager) ExecuteHeartbeatTask(ctx context.Context, engagement *models.Engagement, roleSlug, taskDescription string) error {
	org, err := tm.orgRepo.GetByID(ctx, engagement.OrgID)
	if err != nil || org == nil {
		return err
	}

	soul, _ := tm.engagementSvc.contextRepo.GetOrgSoul(ctx, org.ID)

	task := &models.Task{
		OrgID: &org.ID, EngagementID: &engagement.ID,
		RoleSlug: roleSlug, InputText: taskDescription, Status: "created",
	}
	if err := tm.taskRepo.Create(ctx, task); err != nil {
		return err
	}

	engineReq := &models.EngineRequest{
		TaskID: task.ID.String(), InputText: taskDescription,
		OrgSoul: soul, Role: roleSlug, Tier: org.TierSlug(),
		EngagementID: engagement.ID.String(), OrgID: org.ID.String(),
		GoalAncestry: map[string]interface{}{
			"engagement_objective": engagement.Objective,
			"engagement_type":     engagement.EngagementType,
			"your_role_purpose":   "proactive heartbeat: " + taskDescription,
		},
	}

	result, err := tm.engine.RunTask(ctx, engineReq)
	if err != nil {
		tm.taskRepo.Fail(ctx, task.ID, err.Error())
		return err
	}

	entitiesJSON, _ := json.Marshal(result.ExtractedEntities)
	tm.taskRepo.Complete(ctx, task.ID, result.OutputText, result.QualityScore, entitiesJSON, result.ProcessingTimeMs)
	return nil
}

func (tm *TaskManager) broadcastProgress(memberID, taskID uuid.UUID, status, roleSlug, content string) {
	if tm.broadcaster == nil {
		return
	}
	msg, _ := json.Marshal(map[string]string{
		"type": status, "task_id": taskID.String(), "role_slug": roleSlug, "content": content,
	})
	tm.broadcaster.SendToMember(memberID, msg)
}
