package models

import (
	"time"

	"github.com/google/uuid"
)

type Organization struct {
	ID                   uuid.UUID `json:"id"`
	Name                 string    `json:"name"`
	Industry             string    `json:"industry,omitempty"`
	Website              string    `json:"website,omitempty"`
	SubscriptionPlan     string    `json:"subscription_plan"`
	TasksUsedThisMonth   int       `json:"tasks_used_this_month"`
	TasksLimit           int       `json:"tasks_limit"`
	MembersLimit         int       `json:"members_limit"`
	RolesLimit           int       `json:"roles_limit"`
	FreeTasksRemaining   int       `json:"free_tasks_remaining"`
	StripeCustomerID     string    `json:"stripe_customer_id,omitempty"`
	StripeSubscriptionID string    `json:"stripe_subscription_id,omitempty"`
	Onboarded            bool      `json:"onboarded"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

func (o *Organization) CanCreateTask() bool {
	if o.SubscriptionPlan == "free_trial" {
		return o.FreeTasksRemaining > 0
	}
	return o.TasksUsedThisMonth < o.TasksLimit || o.TasksLimit == 0
}

func (o *Organization) TierSlug() string {
	return o.SubscriptionPlan
}
