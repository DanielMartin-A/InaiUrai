package auth

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

// Request/response structs match openapi.json (snake_case JSON).

type RegisterRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AccountResponse struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	DisplayName  string `json:"display_name"`
	Role         string `json:"role"`
	BalanceCents int64  `json:"balance_cents"`
	HoldCents    int64  `json:"hold_cents"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

type Handler struct {
	svc Service
	log *slog.Logger
}

func NewHandler(svc Service, log *slog.Logger) *Handler {
	if log == nil {
		log = slog.Default()
	}
	return &Handler{svc: svc, log: log}
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Email == "" || req.Password == "" || req.DisplayName == "" || req.Role == "" {
		http.Error(w, "missing required fields", http.StatusBadRequest)
		return
	}
	acc, err := h.svc.Register(r.Context(), req.Email, req.Password, req.DisplayName, req.Role)
	if err != nil {
		if errors.Is(err, ErrDuplicateEmail) {
			http.Error(w, "email already registered", http.StatusConflict)
			return
		}
		if err.Error() == "invalid role" {
			http.Error(w, "invalid role", http.StatusBadRequest)
			return
		}
		h.log.Error("register failed", "error", err)
		http.Error(w, "registration failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(accountToResponse(acc))
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Email == "" || req.Password == "" {
		http.Error(w, "missing email or password", http.StatusBadRequest)
		return
	}
	token, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if err.Error() == "invalid credentials" {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		h.log.Error("login failed", "error", err)
		http.Error(w, "login failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(LoginResponse{Token: token})
}

func accountToResponse(a *Account) AccountResponse {
	return AccountResponse{
		ID:           a.ID.String(),
		Email:        a.Email,
		DisplayName:  a.DisplayName,
		Role:         a.Role,
		BalanceCents: a.BalanceCents,
		HoldCents:    a.HoldCents,
	}
}
