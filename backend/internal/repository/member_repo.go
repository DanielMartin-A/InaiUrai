package repository

import (
	"context"
	"database/sql"

	"github.com/DanielMartin-A/InaiUrai/backend/internal/models"
	"github.com/google/uuid"
)

type MemberRepo struct{ db *sql.DB }

func NewMemberRepo(db *sql.DB) *MemberRepo { return &MemberRepo{db: db} }

func (r *MemberRepo) GetByTelegramID(ctx context.Context, telegramUserID int64) (*models.Member, error) {
	m := &models.Member{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, org_id, COALESCE(name,''), COALESCE(email,''), telegram_user_id,
		 COALESCE(slack_user_id,''), active_channel, is_admin, created_at
		 FROM members WHERE telegram_user_id = $1`, telegramUserID).Scan(
		&m.ID, &m.OrgID, &m.Name, &m.Email, &m.TelegramUserID,
		&m.SlackUserID, &m.ActiveChannel, &m.IsAdmin, &m.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return m, err
}

func (r *MemberRepo) GetBySlackID(ctx context.Context, slackUserID string) (*models.Member, error) {
	m := &models.Member{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, org_id, COALESCE(name,''), active_channel, is_admin FROM members WHERE slack_user_id = $1`,
		slackUserID).Scan(&m.ID, &m.OrgID, &m.Name, &m.ActiveChannel, &m.IsAdmin)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return m, err
}

func (r *MemberRepo) GetByWhatsAppID(ctx context.Context, waID string) (*models.Member, error) {
	m := &models.Member{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, org_id, COALESCE(name,''), active_channel, is_admin FROM members WHERE whatsapp_id = $1`,
		waID).Scan(&m.ID, &m.OrgID, &m.Name, &m.ActiveChannel, &m.IsAdmin)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return m, err
}

func (r *MemberRepo) GetByEmail(ctx context.Context, email string) (*models.Member, error) {
	m := &models.Member{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, org_id, COALESCE(name,''), active_channel, is_admin FROM members WHERE LOWER(email) = LOWER($1)`,
		email).Scan(&m.ID, &m.OrgID, &m.Name, &m.ActiveChannel, &m.IsAdmin)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return m, err
}

func (r *MemberRepo) GetWhatsAppID(ctx context.Context, memberID uuid.UUID) (string, error) {
	var waID string
	err := r.db.QueryRowContext(ctx, `SELECT COALESCE(whatsapp_id,'') FROM members WHERE id = $1`, memberID).Scan(&waID)
	return waID, err
}

func (r *MemberRepo) CreateWithOrgWhatsApp(ctx context.Context, name string, waID string) (*models.Member, *models.Organization, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()

	org := &models.Organization{
		Name: name + "'s Organization", SubscriptionPlan: "free_trial",
		TasksLimit: 3, MembersLimit: 1, RolesLimit: 1, FreeTasksRemaining: 3,
	}
	err = tx.QueryRowContext(ctx,
		`INSERT INTO organizations (name, subscription_plan, tasks_limit, members_limit, roles_limit, free_tasks_remaining)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at`,
		org.Name, org.SubscriptionPlan, org.TasksLimit, org.MembersLimit, org.RolesLimit, org.FreeTasksRemaining,
	).Scan(&org.ID, &org.CreatedAt)
	if err != nil {
		return nil, nil, err
	}

	member := &models.Member{OrgID: org.ID, Name: name, ActiveChannel: "whatsapp", IsAdmin: true}
	err = tx.QueryRowContext(ctx,
		`INSERT INTO members (org_id, name, whatsapp_id, active_channel, is_admin)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at`,
		member.OrgID, member.Name, waID, member.ActiveChannel, member.IsAdmin,
	).Scan(&member.ID, &member.CreatedAt)
	if err != nil {
		return nil, nil, err
	}

	return member, org, tx.Commit()
}

func (r *MemberRepo) GetByAPIToken(ctx context.Context, token string) (*models.Member, error) {
	m := &models.Member{}
	err := r.db.QueryRowContext(ctx,
		`SELECT m.id, m.org_id, COALESCE(m.name,''), m.is_admin
		 FROM members m JOIN api_tokens t ON t.member_id = m.id
		 WHERE t.token_hash = encode(sha256($1::bytea), 'hex') AND t.revoked = FALSE AND t.expires_at > NOW()`,
		token).Scan(&m.ID, &m.OrgID, &m.Name, &m.IsAdmin)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return m, err
}

func (r *MemberRepo) CreateWithOrg(ctx context.Context, name string, telegramUserID int64) (*models.Member, *models.Organization, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()

	org := &models.Organization{
		Name: name + "'s Organization", SubscriptionPlan: "free_trial",
		TasksLimit: 3, MembersLimit: 1, RolesLimit: 1, FreeTasksRemaining: 3,
	}
	err = tx.QueryRowContext(ctx,
		`INSERT INTO organizations (name, subscription_plan, tasks_limit, members_limit, roles_limit, free_tasks_remaining)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at`,
		org.Name, org.SubscriptionPlan, org.TasksLimit, org.MembersLimit, org.RolesLimit, org.FreeTasksRemaining,
	).Scan(&org.ID, &org.CreatedAt)
	if err != nil {
		return nil, nil, err
	}

	member := &models.Member{OrgID: org.ID, Name: name, TelegramUserID: telegramUserID, ActiveChannel: "telegram", IsAdmin: true}
	err = tx.QueryRowContext(ctx,
		`INSERT INTO members (org_id, name, telegram_user_id, active_channel, is_admin)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at`,
		member.OrgID, member.Name, member.TelegramUserID, member.ActiveChannel, member.IsAdmin,
	).Scan(&member.ID, &member.CreatedAt)
	if err != nil {
		return nil, nil, err
	}

	return member, org, tx.Commit()
}
