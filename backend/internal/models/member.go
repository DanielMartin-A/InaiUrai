package models

import (
	"time"

	"github.com/google/uuid"
)

type Member struct {
	ID             uuid.UUID `json:"id"`
	OrgID          uuid.UUID `json:"org_id"`
	Name           string    `json:"name,omitempty"`
	Email          string    `json:"email,omitempty"`
	TelegramUserID int64     `json:"telegram_user_id,omitempty"`
	SlackUserID    string    `json:"slack_user_id,omitempty"`
	WhatsAppID     string    `json:"whatsapp_id,omitempty"`
	RoleInCompany  string    `json:"role_in_company,omitempty"`
	ActiveChannel  string    `json:"active_channel"`
	IsAdmin        bool      `json:"is_admin"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
