package http

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// AnalyticsHandler handles analytics HTTP endpoints.
type AnalyticsHandler struct {
	analytics store.AnalyticsStore
	token     string
}

func NewAnalyticsHandler(analytics store.AnalyticsStore, token string) *AnalyticsHandler {
	return &AnalyticsHandler{analytics: analytics, token: token}
}

func (h *AnalyticsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/analytics/summary", h.auth(h.handleSummary))
	mux.HandleFunc("GET /v1/analytics/report", h.auth(h.handleReport))
}

func (h *AnalyticsHandler) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.token != "" && extractBearerToken(r) != h.token {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

func (h *AnalyticsHandler) handleSummary(w http.ResponseWriter, r *http.Request) {
	filter, err := h.parseFilter(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	summary, err := h.analytics.Summary(r.Context(), filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"summary": summary})
}

func (h *AnalyticsHandler) handleReport(w http.ResponseWriter, r *http.Request) {
	filter, err := h.parseFilter(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	limit := 10
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, _ := strconv.Atoi(v); n > 0 && n <= 50 {
			limit = n
		}
	}

	summary, _ := h.analytics.Summary(r.Context(), filter)
	agents, _ := h.analytics.TopAgents(r.Context(), filter, limit)
	models, _ := h.analytics.TopModels(r.Context(), filter, limit)
	tools, _ := h.analytics.TopTools(r.Context(), filter, limit)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"summary":   summary,
		"topAgents": agents,
		"topModels": models,
		"topTools":  tools,
	})
}

func (h *AnalyticsHandler) parseFilter(r *http.Request) (store.AnalyticsFilter, error) {
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "today"
	}

	var from, to time.Time
	switch period {
	case "today":
		from, to = today, now
	case "yesterday":
		from, to = today.Add(-24*time.Hour), today
	case "7d":
		from, to = now.Add(-7*24*time.Hour), now
	case "30d":
		from, to = now.Add(-30*24*time.Hour), now
	default:
		fromStr := r.URL.Query().Get("from")
		if fromStr == "" {
			return store.AnalyticsFilter{}, fmt.Errorf("unknown period '%s'; use from/to params", period)
		}
		var parseErr error
		from, parseErr = time.Parse(time.RFC3339, fromStr)
		if parseErr != nil {
			return store.AnalyticsFilter{}, fmt.Errorf("invalid from: %v", parseErr)
		}
		to = now
		if toStr := r.URL.Query().Get("to"); toStr != "" {
			to, parseErr = time.Parse(time.RFC3339, toStr)
			if parseErr != nil {
				return store.AnalyticsFilter{}, fmt.Errorf("invalid to: %v", parseErr)
			}
		}
	}

	filter := store.AnalyticsFilter{From: from, To: to}
	if v := r.URL.Query().Get("agentId"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			filter.AgentID = &id
		}
	}
	return filter, nil
}
