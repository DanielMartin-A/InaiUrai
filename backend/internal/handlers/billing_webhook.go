package handlers

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"

	stripe "github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/webhook"

	"github.com/DanielMartin-A/InaiUrai/backend/internal/services"
)

type BillingHandler struct{ svc *services.BillingService }

func NewBillingHandler(svc *services.BillingService) *BillingHandler {
	return &BillingHandler{svc: svc}
}

func (h *BillingHandler) Handle(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 65536))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	sig := r.Header.Get("Stripe-Signature")
	secret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	event, err := webhook.ConstructEvent(body, sig, secret)
	if err != nil {
		slog.Warn("stripe signature verification failed", "error", err)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	go h.processEvent(event)
	w.WriteHeader(http.StatusOK)
}

func (h *BillingHandler) processEvent(event stripe.Event) {
	ctx := context.Background()
	switch event.Type {
	case "customer.subscription.created", "customer.subscription.updated":
		var sub struct {
			Customer string `json:"customer"`
			Items    struct {
				Data []struct {
					Price struct {
						ID string `json:"id"`
					} `json:"price"`
				} `json:"data"`
			} `json:"items"`
		}
		json.Unmarshal(event.Data.Raw, &sub)
		priceID := ""
		if len(sub.Items.Data) > 0 {
			priceID = sub.Items.Data[0].Price.ID
		}
		if err := h.svc.HandleSubscriptionEvent(ctx, string(event.Type), sub.Customer, priceID, event.ID); err != nil {
			slog.Error("stripe event processing failed", "error", err, "event_id", event.ID)
		}
	case "customer.subscription.deleted":
		var sub struct {
			Customer string `json:"customer"`
		}
		json.Unmarshal(event.Data.Raw, &sub)
		if err := h.svc.HandleSubscriptionEvent(ctx, "subscription_cancelled", sub.Customer, "", event.ID); err != nil {
			slog.Error("stripe cancellation processing failed", "error", err, "event_id", event.ID)
		}
	}
}
