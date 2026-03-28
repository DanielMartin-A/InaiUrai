package services

import (
	"context"
	"log/slog"

	"github.com/DanielMartin-A/InaiUrai/backend/internal/repository"
	"github.com/google/uuid"
)

type BillingService struct {
	billingRepo *repository.BillingRepo
	orgRepo     *repository.OrgRepo
}

func NewBillingService(br *repository.BillingRepo, or *repository.OrgRepo) *BillingService {
	return &BillingService{billingRepo: br, orgRepo: or}
}

var planConfig = map[string]struct{ tasks, members, roles int }{
	"solo":       {50, 1, 16},
	"team":       {500, 5, 16},
	"department": {0, 10, 16},
	"company":    {0, 50, 16},
}

func (s *BillingService) HandleSubscriptionEvent(ctx context.Context, eventType, stripeCustomerID, stripePriceID, stripeEventID string) error {
	org, err := s.orgRepo.GetByStripeCustomerID(ctx, stripeCustomerID)
	if err != nil || org == nil {
		slog.Warn("stripe event for unknown org", "stripe_customer_id", stripeCustomerID)
		return nil
	}

	plan := priceToPlan(stripePriceID)
	cfg, known := planConfig[plan]
	if !known {
		slog.Warn("unknown stripe price mapping", "price_id", stripePriceID, "plan", plan)
		return nil
	}

	if err := s.orgRepo.UpdatePlan(ctx, org.ID, plan, cfg.tasks, cfg.members, cfg.roles); err != nil {
		return err
	}
	s.billingRepo.LogEvent(ctx, org.ID, eventType, stripeEventID, 0)
	slog.Info("subscription updated", "org_id", org.ID, "plan", plan)
	return nil
}

func priceToPlan(priceID string) string {
	switch priceID {
	case "price_solo":
		return "solo"
	case "price_team":
		return "team"
	case "price_department":
		return "department"
	case "price_company":
		return "company"
	default:
		return ""
	}
}

var _ = uuid.UUID{}
