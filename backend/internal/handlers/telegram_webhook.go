package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/DanielMartin-A/InaiUrai/backend/internal/services"
)

type TelegramHandler struct{ svc *services.TelegramService }

func NewTelegramHandler(svc *services.TelegramService) *TelegramHandler {
	return &TelegramHandler{svc: svc}
}

func (h *TelegramHandler) Handle(w http.ResponseWriter, r *http.Request) {
	var update services.TelegramUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		slog.Warn("invalid telegram update", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	go h.svc.ProcessUpdate(context.Background(), update)
	w.WriteHeader(http.StatusOK)
}
