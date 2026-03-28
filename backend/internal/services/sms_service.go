package services

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type SMSService struct {
	accountSID string
	authToken  string
	fromNumber string
	client     *http.Client
}

func NewSMSService(accountSID, authToken, fromNumber string) *SMSService {
	return &SMSService{accountSID: accountSID, authToken: authToken, fromNumber: fromNumber, client: &http.Client{Timeout: 15 * time.Second}}
}

func (s *SMSService) IsConfigured() bool {
	return s.accountSID != "" && s.authToken != "" && s.fromNumber != ""
}

func (s *SMSService) Send(ctx context.Context, to, body string) error {
	if !s.IsConfigured() {
		return nil
	}
	if len(body) > 1500 {
		body = body[:1450] + "\n\n\xf0\x9f\x93\x8e Full result in your dashboard"
	}
	data := url.Values{"To": {to}, "From": {s.fromNumber}, "Body": {body}}
	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", s.accountSID)
	req, _ := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(data.Encode()))
	req.SetBasicAuth(s.accountSID, s.authToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.client.Do(req)
	if err != nil {
		slog.Warn("twilio send failed", "error", err)
		return err
	}
	resp.Body.Close()
	return nil
}

func (s *SMSService) SendTaskNotification(ctx context.Context, phone, roleSlug, preview string) error {
	body := fmt.Sprintf("InaiUrai \xe2\x80\x94 Your %s completed a task:\n\n%s", strings.ReplaceAll(roleSlug, "-", " "), preview)
	return s.Send(ctx, phone, body)
}
