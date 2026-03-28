package services

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/DanielMartin-A/InaiUrai/backend/internal/repository"
)

type SlackService struct {
	signingSecret string
	botToken      string
	memberRepo    *repository.MemberRepo
	orgRepo       *repository.OrgRepo
	taskMgr       *TaskManager
}

func NewSlackService(signingSecret, botToken string, mr *repository.MemberRepo, or *repository.OrgRepo, tm *TaskManager) *SlackService {
	return &SlackService{signingSecret: signingSecret, botToken: botToken, memberRepo: mr, orgRepo: or, taskMgr: tm}
}

func (s *SlackService) ProcessMessage(ctx context.Context, slackUserID, text, channelID string) {
	member, err := s.memberRepo.GetBySlackID(ctx, slackUserID)
	if err != nil || member == nil {
		slog.Warn("slack member not found", "slack_user_id", slackUserID)
		return
	}
	org, err := s.orgRepo.GetByID(ctx, member.OrgID)
	if err != nil || org == nil {
		return
	}

	result, err := s.taskMgr.ExecuteSoloTask(ctx, org, member, text)
	if err != nil {
		slog.Error("slack task failed", "error", err)
		s.postMessage(channelID, "Something went wrong. Please try again.")
		return
	}
	s.postMessage(channelID, result)
}

func (s *SlackService) postMessage(channelID, text string) {
	if s.botToken == "" {
		return
	}
	body, _ := json.Marshal(map[string]string{"channel": channelID, "text": text})
	req, _ := http.NewRequest("POST", "https://slack.com/api/chat.postMessage", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.botToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Warn("slack postMessage failed", "error", err, "channel", channelID)
		return
	}
	resp.Body.Close()
}
