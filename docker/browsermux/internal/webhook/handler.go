package webhook

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

type Handler struct {
	manager Manager
}

func NewHandler(manager Manager) *Handler {
	return &Handler{
		manager: manager,
	}
}

func (h *Handler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/webhooks", h.CreateWebhook).Methods("POST")
	router.HandleFunc("/webhooks", h.ListWebhooks).Methods("GET")
	router.HandleFunc("/webhooks/{id}", h.GetWebhook).Methods("GET")
	router.HandleFunc("/webhooks/{id}", h.UpdateWebhook).Methods("PUT")
	router.HandleFunc("/webhooks/{id}", h.DeleteWebhook).Methods("DELETE")

	router.HandleFunc("/webhook-executions", h.ListExecutions).Methods("GET")
	router.HandleFunc("/webhook-executions/{id}", h.GetExecution).Methods("GET")
	router.HandleFunc("/webhooks/test", h.TestWebhook).Methods("POST")
}

func (h *Handler) CreateWebhook(w http.ResponseWriter, r *http.Request) {
	var config WebhookConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	id, err := h.manager.RegisterWebhook(config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	webhook, _ := h.manager.GetWebhook(id)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(webhook)
}

func (h *Handler) ListWebhooks(w http.ResponseWriter, r *http.Request) {
	webhooks := h.manager.ListWebhooks()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"webhooks": webhooks,
	})
}

func (h *Handler) GetWebhook(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	webhook, err := h.manager.GetWebhook(id)
	if err != nil {
		http.Error(w, "Webhook not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(webhook)
}

func (h *Handler) UpdateWebhook(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	webhook, err := h.manager.UpdateWebhook(id, updates)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(webhook)
}

func (h *Handler) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	err := h.manager.DeleteWebhook(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListExecutions(w http.ResponseWriter, r *http.Request) {
	webhookID := r.URL.Query().Get("webhook_id")
	clientID := r.URL.Query().Get("client_id")
	status := r.URL.Query().Get("status")

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil {
			limit = parsedLimit
		}
	}

	executions := h.manager.ListExecutions(webhookID, clientID, status, limit)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"executions": executions,
	})
}

func (h *Handler) GetExecution(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	execution, err := h.manager.GetExecution(id)
	if err != nil {
		http.Error(w, "Execution not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(execution)
}

func (h *Handler) TestWebhook(w http.ResponseWriter, r *http.Request) {
	var request struct {
		WebhookID string                 `json:"webhook_id"`
		ClientID  string                 `json:"client_id"`
		CDPMethod string                 `json:"cdp_method"`
		CDPParams map[string]interface{} `json:"cdp_params"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	execution, err := h.manager.TestWebhook(request.WebhookID, request.ClientID, request.CDPMethod, request.CDPParams)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(execution)
}
