package services

import (
	"github.com/DanielMartin-A/InaiUrai/backend/internal/repository"
)

type EngagementService struct {
	engagementRepo *repository.EngagementRepo
	taskRepo       *repository.TaskRepo
	orgRepo        *repository.OrgRepo
	engine         *EngineClient
	contextRepo    *repository.ContextRepo
}

func NewEngagementService(er *repository.EngagementRepo, tr *repository.TaskRepo, or *repository.OrgRepo, ec *EngineClient, cr *repository.ContextRepo) *EngagementService {
	return &EngagementService{engagementRepo: er, taskRepo: tr, orgRepo: or, engine: ec, contextRepo: cr}
}
