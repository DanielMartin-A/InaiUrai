package handlers

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/DanielMartin-A/InaiUrai/backend/internal/services"
)

type WhatsAppHandler struct{ svc *services.WhatsAppService }

func NewWhatsAppHandler(svc *services.WhatsAppService) *WhatsAppHandler {
	return &WhatsAppHandler{svc: svc}
}

func (h *WhatsAppHandler) HandleVerify(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("hub.mode")
	token := r.URL.Query().Get("hub.verify_token")
	challenge := r.URL.Query().Get("hub.challenge")
	if result, ok := h.svc.VerifyWebhook(mode, token, challenge); ok {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(result))
		return
	}
	http.Error(w, "Forbidden", http.StatusForbidden)
}

func (h *WhatsAppHandler) HandleMessage(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 65536))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	sig := r.Header.Get("X-Hub-Signature-256")
	if !h.svc.ValidateSignature(body, sig) {
		slog.Warn("whatsapp signature failed")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var payload services.WAPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	h.svc.ProcessWebhook(r.Context(), payload)
	w.WriteHeader(http.StatusOK)
}
