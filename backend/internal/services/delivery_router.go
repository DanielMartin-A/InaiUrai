package services

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/DanielMartin-A/InaiUrai/backend/internal/models"
	"github.com/DanielMartin-A/InaiUrai/backend/internal/repository"
	"github.com/google/uuid"
)

type DeliveryRouter struct {
	hub        ProgressBroadcaster
	telegram   *TelegramService
	whatsapp   *WhatsAppService
	slack      *SlackService
	sms        *SMSService
	memberRepo *repository.MemberRepo
}

func NewDeliveryRouter(hub ProgressBroadcaster, tg *TelegramService, wa *WhatsAppService, sl *SlackService, sms *SMSService, mr *repository.MemberRepo) *DeliveryRouter {
	return &DeliveryRouter{hub: hub, telegram: tg, whatsapp: wa, slack: sl, sms: sms, memberRepo: mr}
}

func (d *DeliveryRouter) Deliver(ctx context.Context, member *models.Member, taskID uuid.UUID, roleSlug, content string) {
	if d.hub != nil {
		msg, _ := json.Marshal(map[string]string{
			"type": "done", "task_id": taskID.String(), "role_slug": roleSlug, "content": content,
		})
		d.hub.SendToMember(member.ID, msg)
	}

	switch member.ActiveChannel {
	case "telegram":
		if member.TelegramUserID != 0 && d.telegram != nil {
			for _, chunk := range splitMessage(content, 4000) {
				d.telegram.reply(member.TelegramUserID, chunk)
			}
		}
	case "whatsapp":
		if d.whatsapp != nil {
			waID, _ := d.memberRepo.GetWhatsAppID(ctx, member.ID)
			if waID != "" {
				d.whatsapp.SendText(waID, content)
			}
		}
	case "slack":
		if member.SlackUserID != "" && d.slack != nil {
			d.slack.postMessage(member.SlackUserID, content)
		}
	}

	slog.Info("delivered result", "member_id", member.ID, "channel", member.ActiveChannel, "task_id", taskID)
}
