package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// NotificationHandler handles notification HTTP endpoints.
type NotificationHandler struct {
	notifications store.NotificationStore
	token         string
}

func NewNotificationHandler(notifications store.NotificationStore, token string) *NotificationHandler {
	return &NotificationHandler{notifications: notifications, token: token}
}

func (h *NotificationHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/notifications", h.auth(h.handleList))
	mux.HandleFunc("GET /v1/notifications/unread", h.auth(h.handleUnreadCount))
	mux.HandleFunc("POST /v1/notifications/{id}/read", h.auth(h.handleMarkRead))
	mux.HandleFunc("POST /v1/notifications/read-all", h.auth(h.handleMarkAllRead))
}

func (h *NotificationHandler) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.token != "" && extractBearerToken(r) != h.token {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

func (h *NotificationHandler) handleList(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("userId")
	if userID == "" {
		userID = "admin"
	}
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, _ := strconv.Atoi(v); n > 0 && n <= 200 {
			limit = n
		}
	}
	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, _ := strconv.Atoi(v); n >= 0 {
			offset = n
		}
	}

	items, err := h.notifications.List(r.Context(), userID, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if items == nil {
		items = []store.Notification{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"notifications": items})
}

func (h *NotificationHandler) handleUnreadCount(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("userId")
	if userID == "" {
		userID = "admin"
	}

	count, err := h.notifications.CountUnread(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"count": count})
}

func (h *NotificationHandler) handleMarkRead(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid notification id"})
		return
	}

	if err := h.notifications.MarkRead(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *NotificationHandler) handleMarkAllRead(w http.ResponseWriter, r *http.Request) {
	var body struct {
		UserID string `json:"userId"`
	}
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&body)
	}
	if body.UserID == "" {
		body.UserID = "admin"
	}

	if err := h.notifications.MarkAllRead(r.Context(), body.UserID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
