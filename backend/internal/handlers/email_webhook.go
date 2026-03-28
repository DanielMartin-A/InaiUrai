package handlers

import (
	"log/slog"
	"net/http"

	"github.com/DanielMartin-A/InaiUrai/backend/internal/services"
)

type EmailHandler struct{ svc *services.EmailService }

func NewEmailHandler(svc *services.EmailService) *EmailHandler {
	return &EmailHandler{svc: svc}
}

func (h *EmailHandler) HandleInbound(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		if err2 := r.ParseForm(); err2 != nil {
			slog.Warn("email: parse failed", "error", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}
	email := services.InboundEmail{
		From: r.FormValue("from"), To: r.FormValue("to"),
		Subject: r.FormValue("subject"), Text: r.FormValue("text"), HTML: r.FormValue("html"),
	}
	if email.From == "" || (email.Text == "" && email.HTML == "") {
		w.WriteHeader(http.StatusOK)
		return
	}
	slog.Info("email inbound", "from", email.From, "subject", email.Subject)
	h.svc.ProcessInbound(r.Context(), email)
	w.WriteHeader(http.StatusOK)
}
