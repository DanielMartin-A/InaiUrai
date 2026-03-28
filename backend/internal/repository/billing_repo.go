package repository

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

type BillingRepo struct{ db *sql.DB }

func NewBillingRepo(db *sql.DB) *BillingRepo { return &BillingRepo{db: db} }

func (r *BillingRepo) LogEvent(ctx context.Context, orgID uuid.UUID, eventType, stripeEventID string, amountCents int) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO billing_events (customer_id, event_type, amount_cents, stripe_event_id)
		 SELECT m.id, $2, $3, $4 FROM members m WHERE m.org_id = $1 AND m.is_admin = TRUE LIMIT 1
		 ON CONFLICT (stripe_event_id) DO NOTHING`,
		orgID, eventType, amountCents, stripeEventID)
	return err
}
