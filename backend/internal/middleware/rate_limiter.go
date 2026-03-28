package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type rateBucket struct {
	timestamps []time.Time
}

var (
	rateMu      sync.Mutex
	rateBuckets = make(map[string]*rateBucket)
)

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func RateLimit(maxPerMinute int, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		rateMu.Lock()
		b, ok := rateBuckets[ip]
		if !ok {
			b = &rateBucket{}
			rateBuckets[ip] = b
		}
		now := time.Now()
		cutoff := now.Add(-time.Minute)
		var valid []time.Time
		for _, t := range b.timestamps {
			if t.After(cutoff) {
				valid = append(valid, t)
			}
		}
		if len(valid) >= maxPerMinute {
			rateMu.Unlock()
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		b.timestamps = append(valid, now)
		rateMu.Unlock()
		next.ServeHTTP(w, r)
	})
}
