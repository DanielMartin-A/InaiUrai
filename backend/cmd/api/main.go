package main

import (
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
)

// Minimal API stub for local Docker: health + engine internal endpoints.
// Replace with full v5 implementation as features land.

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	internalKey := os.Getenv("INTERNAL_API_KEY")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("POST /api/context/selective", requireInternal(internalKey, handleSelectiveContext))
	mux.HandleFunc("GET /api/internal/daily-cost/{orgID}", requireInternal(internalKey, handleDailyCost))
	mux.HandleFunc("POST /api/internal/record-cost", requireInternal(internalKey, handleRecordCost))
	mux.HandleFunc("POST /api/internal/audit", requireInternal(internalKey, handleAuditBatch))
	mux.HandleFunc("GET /api/internal/audit/{taskID}", requireInternal(internalKey, handleAuditGet))

	addr := ":" + port
	log.Printf("backend listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func requireInternal(expected string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if expected == "" {
			http.Error(w, "server misconfigured", http.StatusServiceUnavailable)
			return
		}
		provided := r.Header.Get("X-Internal-Key")
		if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

func handleSelectiveContext(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"fragments": []any{},
		"note":      "stub: no DB wired",
	})
}

func handleDailyCost(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"estimated_cost_cents": 0,
		"total_tokens":         0,
		"task_count":           0,
	})
}

func handleRecordCost(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

var auditStore sync.Map // taskID -> []auditEntry (RAM only stub)

type auditEntry struct {
	StepNumber  int                    `json:"step_number"`
	ActionType  string                 `json:"action_type"`
	ToolName    *string                `json:"tool_name,omitempty"`
	ToolInput   any                    `json:"tool_input,omitempty"`
	ToolOutput  any                    `json:"tool_output,omitempty"`
	TokensUsed  int                    `json:"tokens_used"`
	BlockedBy   *string                `json:"blocked_by,omitempty"`
}

type auditBatch struct {
	TaskID  string       `json:"task_id"`
	OrgID   string       `json:"org_id"`
	Entries []auditEntry `json:"entries"`
}

func handleAuditBatch(w http.ResponseWriter, r *http.Request) {
	var body auditBatch
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if body.TaskID == "" {
		http.Error(w, "task_id required", http.StatusBadRequest)
		return
	}
	prev, _ := auditStore.Load(body.TaskID)
	var existing []auditEntry
	if prev != nil {
		existing = prev.([]auditEntry)
	}
	auditStore.Store(body.TaskID, append(existing, body.Entries...))
	w.WriteHeader(http.StatusNoContent)
}

func handleAuditGet(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("taskID")
	if taskID == "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	v, ok := auditStore.Load(taskID)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"entries": []auditEntry{}})
		return
	}
	entries := v.([]auditEntry)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"entries": entries})
}
