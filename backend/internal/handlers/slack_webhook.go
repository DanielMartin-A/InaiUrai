package handlers

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/DanielMartin-A/InaiUrai/backend/internal/services"
)

type SlackHandler struct{ svc *services.SlackService }

func NewSlackHandler(svc *services.SlackService) *SlackHandler {
	return &SlackHandler{svc: svc}
}

func (h *SlackHandler) HandleEvent(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(io.LimitReader(r.Body, 65536))
	var payload struct {
		Type      string `json:"type"`
		Challenge string `json:"challenge"`
		Event     struct {
			Type    string `json:"type"`
			User    string `json:"user"`
			Text    string `json:"text"`
			Channel string `json:"channel"`
		} `json:"event"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if payload.Type == "url_verification" {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(payload.Challenge))
		return
	}
	if payload.Event.Type == "message" && payload.Event.User != "" {
		go h.svc.ProcessMessage(context.Background(), payload.Event.User, payload.Event.Text, payload.Event.Channel)
	}
	w.WriteHeader(http.StatusOK)
}

func (h *SlackHandler) HandleInteraction(w http.ResponseWriter, r *http.Request) {
	slog.Info("slack interaction received")
	w.WriteHeader(http.StatusOK)
}
