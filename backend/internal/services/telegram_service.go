package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/DanielMartin-A/InaiUrai/backend/internal/models"
	"github.com/DanielMartin-A/InaiUrai/backend/internal/repository"
)

type TelegramService struct {
	token      string
	memberRepo *repository.MemberRepo
	orgRepo    *repository.OrgRepo
	taskMgr    *TaskManager
}

func NewTelegramService(token string, mr *repository.MemberRepo, or *repository.OrgRepo, tm *TaskManager) *TelegramService {
	return &TelegramService{token: token, memberRepo: mr, orgRepo: or, taskMgr: tm}
}

type TelegramUpdate struct {
	Message struct {
		MessageID int `json:"message_id"`
		From      struct {
			ID        int64  `json:"id"`
			FirstName string `json:"first_name"`
			Username  string `json:"username"`
		} `json:"from"`
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		Text string `json:"text"`
	} `json:"message"`
}

func (s *TelegramService) ProcessUpdate(ctx context.Context, update TelegramUpdate) {
	text := strings.TrimSpace(update.Message.Text)
	chatID := update.Message.Chat.ID
	telegramUserID := update.Message.From.ID

	if text == "" || text == "/start" {
		s.reply(chatID, "Welcome to InaiUrai! \xf0\x9f\x9a\x80\n\nDescribe what you need \xe2\x80\x94 I'll route it to the right AI executive.\n\nExamples:\n\xe2\x80\xa2 Research top CRM tools for agencies\n\xe2\x80\xa2 Draft a cold email to a potential investor\n\xe2\x80\xa2 Analyze our competitor's pricing strategy")
		return
	}

	member, err := s.memberRepo.GetByTelegramID(ctx, telegramUserID)
	if err != nil {
		slog.Error("member lookup failed", "error", err, "telegram_id", telegramUserID)
		s.reply(chatID, "Something went wrong. Please try again.")
		return
	}

	if member == nil {
		name := update.Message.From.FirstName
		if name == "" {
			name = update.Message.From.Username
		}
		member, _, err = s.memberRepo.CreateWithOrg(ctx, name, telegramUserID)
		if err != nil {
			slog.Error("auto-registration failed", "error", err)
			s.reply(chatID, "Registration failed. Please try again.")
			return
		}
		s.reply(chatID, fmt.Sprintf("Welcome %s! You have 3 free tasks. Let's go! \xf0\x9f\x8e\xaf", name))
	}

	org, err := s.orgRepo.GetByID(ctx, member.OrgID)
	if err != nil || org == nil {
		s.reply(chatID, "Organization not found. Please contact support.")
		return
	}

	s.reply(chatID, "\xf0\x9f\x94\x84 Working on it...")

	result, err := s.taskMgr.ExecuteSoloTask(ctx, org, member, text)
	if err != nil {
		slog.Error("task dispatch failed", "error", err, "org_id", org.ID)
		s.reply(chatID, "Something went wrong. Please try again.")
		return
	}

	if result != "" {
		s.reply(chatID, result)
	}
}

func (s *TelegramService) reply(chatID int64, text string) {
	body, _ := json.Marshal(map[string]interface{}{
		"chat_id": chatID, "text": text, "parse_mode": "Markdown",
	})
	resp, err := http.Post(
		fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.token),
		"application/json", bytes.NewReader(body))
	if err != nil {
		slog.Warn("telegram reply failed", "error", err)
		return
	}
	resp.Body.Close()
}

func splitMessage(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}
	var chunks []string
	for len(text) > 0 {
		end := maxLen
		if end > len(text) {
			end = len(text)
		}
		if end < len(text) {
			if idx := strings.LastIndex(text[:end], "\n"); idx > end/2 {
				end = idx + 1
			}
		}
		chunks = append(chunks, text[:end])
		text = text[end:]
	}
	return chunks
}

var _ = models.Member{}
