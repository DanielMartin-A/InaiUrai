package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/DanielMartin-A/InaiUrai/backend/internal/models"
	"github.com/google/uuid"
)

type OrgRepo struct{ db *sql.DB }

func NewOrgRepo(db *sql.DB) *OrgRepo { return &OrgRepo{db: db} }
func (r *OrgRepo) DB() *sql.DB      { return r.db }

// WithOrgScope executes fn inside a transaction with RLS context set.
// NOTE: Only effective when connected as a non-superuser role (e.g., inaiurai_app).
// The default docker-compose superuser (inaiurai) bypasses RLS.
func (r *OrgRepo) WithOrgScope(ctx context.Context, orgID uuid.UUID, fn func(tx *sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "SET LOCAL app.current_org_id = '"+orgID.String()+"'"); err != nil {
		return fmt.Errorf("set org scope: %w", err)
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *OrgRepo) SetOrgContext(ctx context.Context, tx *sql.Tx, orgID uuid.UUID) error {
	_, err := tx.ExecContext(ctx, fmt.Sprintf("SET LOCAL app.current_org_id = '%s'", orgID))
	return err
}

func (r *OrgRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Organization, error) {
	org := &models.Organization{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, industry, website, subscription_plan, tasks_used_this_month, tasks_limit,
		 members_limit, roles_limit, free_tasks_remaining, COALESCE(stripe_customer_id,''),
		 COALESCE(stripe_subscription_id,''), onboarded, created_at, updated_at
		 FROM organizations WHERE id = $1`, id).Scan(
		&org.ID, &org.Name, &org.Industry, &org.Website, &org.SubscriptionPlan,
		&org.TasksUsedThisMonth, &org.TasksLimit, &org.MembersLimit, &org.RolesLimit,
		&org.FreeTasksRemaining, &org.StripeCustomerID, &org.StripeSubscriptionID,
		&org.Onboarded, &org.CreatedAt, &org.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return org, err
}

func (r *OrgRepo) IncrementTaskCount(ctx context.Context, orgID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE organizations SET tasks_used_this_month = tasks_used_this_month + 1,
		 free_tasks_remaining = GREATEST(0, free_tasks_remaining - 1) WHERE id = $1`, orgID)
	return err
}

func (r *OrgRepo) GetByStripeCustomerID(ctx context.Context, stripeID string) (*models.Organization, error) {
	org := &models.Organization{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, subscription_plan, stripe_customer_id FROM organizations WHERE stripe_customer_id = $1`,
		stripeID).Scan(&org.ID, &org.Name, &org.SubscriptionPlan, &org.StripeCustomerID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return org, err
}

func (r *OrgRepo) UpdatePlan(ctx context.Context, orgID uuid.UUID, plan string, tasksLimit, membersLimit, rolesLimit int) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE organizations SET subscription_plan=$1, tasks_limit=$2, members_limit=$3, roles_limit=$4 WHERE id=$5`,
		plan, tasksLimit, membersLimit, rolesLimit, orgID)
	return err
}

func (r *OrgRepo) Create(ctx context.Context, org *models.Organization) error {
	return r.db.QueryRowContext(ctx,
		`INSERT INTO organizations (name, subscription_plan, tasks_limit, members_limit, roles_limit, free_tasks_remaining)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at`,
		org.Name, org.SubscriptionPlan, org.TasksLimit, org.MembersLimit, org.RolesLimit, org.FreeTasksRemaining,
	).Scan(&org.ID, &org.CreatedAt)
}
