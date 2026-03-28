package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"

	"github.com/DanielMartin-A/InaiUrai/backend/internal/repository"
)

type contextKey string

const MemberContextKey contextKey = "member"

func WebhookHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", h[:16])
}

func RequireInternalKey(key string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provided := r.Header.Get("X-Internal-Key")
		if key == "" || !hmac.Equal([]byte(provided), []byte(key)) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func RequireBearerToken(orgRepo *repository.OrgRepo, memberRepo *repository.MemberRepo, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		token := strings.TrimPrefix(auth, "Bearer ")

		member, err := memberRepo.GetByAPIToken(r.Context(), token)
		if err != nil || member == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), MemberContextKey, member)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
