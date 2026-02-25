package router

import (
	"net/http"
	"strings"

	"github.com/inaiurai/backend/internal/auth"
	"github.com/inaiurai/backend/internal/dashboard"
	"github.com/inaiurai/backend/internal/jobs"
	"github.com/inaiurai/backend/internal/registry"
)

// New returns an http.Handler that serves API under /api/v1.
func New(authHandler *auth.Handler, registryHandler *registry.Handler, jobsHandler *jobs.Handler, dashHandler *dashboard.Handler) http.Handler {
	mux := http.NewServeMux()
	base := "/api/v1"
	mux.HandleFunc(base+"/auth/register", authHandler.Register)
	mux.HandleFunc(base+"/auth/login", authHandler.Login)
	mux.HandleFunc(base+"/agents", agentsHandler(registryHandler))
	mux.HandleFunc(base+"/jobs", jobsHandlerFunc(jobsHandler))

	mux.HandleFunc(base+"/account/me", methodGET(dashHandler.GetMe))
	mux.HandleFunc(base+"/account/settings", methodPATCH(dashHandler.UpdateSettings))
	mux.HandleFunc(base+"/credit-ledger", methodGET(dashHandler.ListCreditLedger))
	mux.HandleFunc(base+"/tasks", methodGET(dashHandler.ListTasks))
	mux.HandleFunc(base+"/agents/kill-all", methodPOST(dashHandler.KillAllAgents))
	mux.HandleFunc(base+"/agents/resume-all", methodPOST(dashHandler.ResumeAllAgents))

	mux.HandleFunc(base+"/api-keys", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			dashHandler.ListAPIKeys(w, r)
		case http.MethodPost:
			dashHandler.CreateAPIKey(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc(base+"/api-keys/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && strings.Count(r.URL.Path, "/") >= 4 {
			dashHandler.DeleteAPIKey(w, r)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})

	return mux
}

func methodGET(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h(w, r)
	}
}

func methodPOST(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h(w, r)
	}
}

func methodPATCH(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h(w, r)
	}
}

func agentsHandler(h *registry.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			h.CreateAgent(w, r)
		case http.MethodGet:
			h.ListAgents(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func jobsHandlerFunc(h *jobs.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			h.CreateJob(w, r)
		case http.MethodGet:
			h.ListJobs(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
