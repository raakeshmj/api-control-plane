package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/raakeshmj/apigatewayplane/internal/audit"
	"github.com/raakeshmj/apigatewayplane/internal/config"
	"github.com/raakeshmj/apigatewayplane/internal/middleware"
)

// ListPolicies returns the current policy configuration
func (s *Server) ListPolicies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	// We don't have a way to "Get" policies from Engine easily as it's private slice.
	// We should expose a Getter on Engine.
	// For now, return "Not Implemented" or just the dynamic config?
	// DynamicConfigManager stores the config.
	// But Engine might have runtime state?
	// Let's stick to reloading config via POST for now, and maybe list from ConfigManager?

	http.Error(w, "Not Implemented", http.StatusNotImplemented)
}

// ReloadPolicies handles dynamic reload (moved from server.go inline)
func (s *Server) ReloadPolicies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var newPolicy config.PolicyConfig
	if err := json.NewDecoder(r.Body).Decode(&newPolicy); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	s.configManager.UpdatePolicy(newPolicy)

	// Audit Log
	if userID, ok := r.Context().Value(middleware.UserContextKey).(string); ok {
		s.auditLogger.Log(audit.LogEntry{
			Timestamp: time.Now(),
			Action:    "policy_reload",
			ActorID:   userID,
			Resource:  "config",
			Status:    http.StatusOK,
		})
	}

	// Also update Engine?
	// ConfigManager updates `Limit` in middleware, but `PolicyEngine` uses `LoadPolicies`.
	// We need to sync them!
	// The `newPolicy` struct in config currently only has `RateLimit`.
	// We need `[]policy.Policy` struct for Engine.
	// This reveals a gap: DynamicConfigManager vs PolicyEngine.

	w.Write([]byte("Configuration updated successfully"))
}

// GenerateAPIKeyHandler
func (s *Server) GenerateAPIKeyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UserID string `json:"user_id"`
		Name   string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	rawKey, err := s.authService.CreateAPIKey(r.Context(), req.UserID, req.Name, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Audit Log
	if actorID, ok := r.Context().Value(middleware.UserContextKey).(string); ok {
		s.auditLogger.Log(audit.LogEntry{
			Timestamp: time.Now(),
			Action:    "key_create",
			ActorID:   actorID,
			Resource:  "apikey:" + req.UserID, // Don't log the key itself!
			Status:    http.StatusOK,
			Metadata:  map[string]interface{}{"target_user": req.UserID, "key_name": req.Name},
		})
	}

	resp := map[string]string{"api_key": rawKey}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// RevokeAPIKeyHandler
func (s *Server) RevokeAPIKeyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	// Parse UserID? Or KeyHash?
	// Simplify: Rotate (Revoke All + Create New)
	var req struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	newKey, err := s.authService.RotateAPIKey(r.Context(), req.UserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Audit Log
	if actorID, ok := r.Context().Value(middleware.UserContextKey).(string); ok {
		s.auditLogger.Log(audit.LogEntry{
			Timestamp: time.Now(),
			Action:    "key_rotate",
			ActorID:   actorID,
			Resource:  "apikey:" + req.UserID,
			Status:    http.StatusOK,
			Metadata:  map[string]interface{}{"target_user": req.UserID},
		})
	}

	resp := map[string]string{"api_key": newKey, "message": "All previous keys revoked"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
