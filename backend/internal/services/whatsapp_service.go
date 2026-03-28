package services

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/DanielMartin-A/InaiUrai/backend/internal/repository"
)

type WhatsAppService struct {
	phoneNumberID string
	accessToken   string
	appSecret     string
	verifyToken   string
	apiURL        string
	memberRepo    *repository.MemberRepo
	orgRepo       *repository.OrgRepo
	taskMgr       *TaskManager
}

func NewWhatsAppService(phoneNumberID, accessToken, appSecret, verifyToken string, mr *repository.MemberRepo, or *repository.OrgRepo, tm *TaskManager) *WhatsAppService {
	return &WhatsAppService{
		phoneNumberID: phoneNumberID, accessToken: accessToken,
		appSecret: appSecret, verifyToken: verifyToken,
		apiURL: fmt.Sprintf("https://graph.facebook.com/v19.0/%s/messages", phoneNumberID),
		memberRepo: mr, orgRepo: or, taskMgr: tm,
	}
}

type WAPayload struct {
	Object string    `json:"object"`
	Entry  []WAEntry `json:"entry"`
}

type WAEntry struct {
	Changes []WAChange `json:"changes"`
}

type WAChange struct {
	Value WAValue `json:"value"`
}

type WAValue struct {
	Contacts []WAContact `json:"contacts,omitempty"`
	Messages []WAMessage `json:"messages,omitempty"`
}

type WAContact struct {
	WaID    string `json:"wa_id"`
	Profile struct {
		Name string `json:"name"`
	} `json:"profile"`
}

type WAMessage struct {
	From string `json:"from"`
	Type string `json:"type"`
	Text *struct {
		Body string `json:"body"`
	} `json:"text,omitempty"`
}

func (s *WhatsAppService) VerifyWebhook(mode, token, challenge string) (string, bool) {
	if mode == "subscribe" && token == s.verifyToken {
		return challenge, true
	}
	return "", false
}

func (s *WhatsAppService) ValidateSignature(body []byte, signature string) bool {
	if s.appSecret == "" {
		return true
	}
	mac := hmac.New(sha256.New, []byte(s.appSecret))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

func (s *WhatsAppService) ProcessWebhook(ctx context.Context, payload WAPayload) {
	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			for _, msg := range change.Value.Messages {
				if msg.Type != "text" || msg.Text == nil {
					continue
				}
				waID := msg.From
				name := ""
				for _, c := range change.Value.Contacts {
					if c.WaID == waID {
						name = c.Profile.Name
						break
					}
				}
				go s.handleMessage(waID, name, msg.Text.Body)
			}
		}
	}
}

func (s *WhatsAppService) handleMessage(waID, name, text string) {
	ctx := context.Background()
	member, err := s.memberRepo.GetByWhatsAppID(ctx, waID)
	if err != nil {
		slog.Error("whatsapp member lookup failed", "error", err)
		return
	}
	if member == nil {
		if name == "" {
			name = "WhatsApp User"
		}
		member, _, err = s.memberRepo.CreateWithOrgWhatsApp(ctx, name, waID)
		if err != nil {
			s.SendText(waID, "Registration failed. Please try again.")
			return
		}
		s.SendText(waID, fmt.Sprintf("Welcome %s! You have 3 free tasks. \xf0\x9f\x8e\xaf", name))
	}
	org, _ := s.orgRepo.GetByID(ctx, member.OrgID)
	if org == nil {
		s.SendText(waID, "Something went wrong.")
		return
	}
	s.SendText(waID, "\xf0\x9f\x94\x84 Working on it...")
	result, err := s.taskMgr.ExecuteSoloTask(ctx, org, member, text)
	if err != nil {
		slog.Error("whatsapp task dispatch failed", "error", err)
		s.SendText(waID, "Something went wrong. Please try again.")
		return
	}
	if result != "" {
		s.SendText(waID, result)
	}
}

func (s *WhatsAppService) SendText(to, text string) error {
	if s.accessToken == "" || s.phoneNumberID == "" {
		return nil
	}
	if len(text) > 4096 {
		text = text[:4090] + "\n\xe2\x80\xa6"
	}
	body, _ := json.Marshal(map[string]interface{}{
		"messaging_product": "whatsapp", "to": to,
		"type": "text", "text": map[string]string{"body": text},
	})
	req, _ := http.NewRequest("POST", s.apiURL, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+s.accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Warn("whatsapp send failed", "error", err)
		return err
	}
	resp.Body.Close()
	return nil
}
