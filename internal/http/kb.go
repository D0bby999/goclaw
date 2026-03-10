package http

import (
	"net/http"

	"github.com/nextlevelbuilder/goclaw/internal/kb"
	"github.com/nextlevelbuilder/goclaw/internal/media"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

const maxKBUploadSize int64 = 10 * 1024 * 1024 // 10MB

// KBHandler handles knowledge base endpoints.
type KBHandler struct {
	store     store.KBStore
	processor *kb.Processor
	storage   media.Storage
	token     string
}

func NewKBHandler(s store.KBStore, proc *kb.Processor, storage media.Storage, token string) *KBHandler {
	return &KBHandler{store: s, processor: proc, storage: storage, token: token}
}

func (h *KBHandler) RegisterRoutes(mux *http.ServeMux) {
	// Collections
	mux.HandleFunc("GET /v1/agents/{agentID}/kb/collections", h.auth(h.handleListCollections))
	mux.HandleFunc("POST /v1/agents/{agentID}/kb/collections", h.auth(h.handleCreateCollection))
	mux.HandleFunc("GET /v1/agents/{agentID}/kb/collections/{collectionID}", h.auth(h.handleGetCollection))
	mux.HandleFunc("PATCH /v1/agents/{agentID}/kb/collections/{collectionID}", h.auth(h.handleUpdateCollection))
	mux.HandleFunc("DELETE /v1/agents/{agentID}/kb/collections/{collectionID}", h.auth(h.handleDeleteCollection))
	// Documents
	mux.HandleFunc("GET /v1/agents/{agentID}/kb/collections/{collectionID}/documents", h.auth(h.handleListDocuments))
	mux.HandleFunc("POST /v1/agents/{agentID}/kb/collections/{collectionID}/documents", h.auth(h.handleUploadDocument))
	mux.HandleFunc("GET /v1/agents/{agentID}/kb/documents/{documentID}", h.auth(h.handleGetDocument))
	mux.HandleFunc("DELETE /v1/agents/{agentID}/kb/documents/{documentID}", h.auth(h.handleDeleteDocument))
	mux.HandleFunc("POST /v1/agents/{agentID}/kb/documents/{documentID}/reprocess", h.auth(h.handleReprocess))
	// Chunks
	mux.HandleFunc("GET /v1/agents/{agentID}/kb/documents/{documentID}/chunks", h.auth(h.handleListChunks))
	// Search
	mux.HandleFunc("POST /v1/agents/{agentID}/kb/search", h.auth(h.handleSearch))
}

func (h *KBHandler) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.token != "" {
			if extractBearerToken(r) != h.token {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
		}
		next(w, r)
	}
}
