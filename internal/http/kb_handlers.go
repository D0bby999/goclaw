package http

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/nextlevelbuilder/goclaw/internal/memory"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// --- Collection handlers ---

func (h *KBHandler) handleListCollections(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentID")
	cols, err := h.store.ListCollections(r.Context(), agentID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if cols == nil {
		cols = []store.KBCollection{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"collections": cols})
}

func (h *KBHandler) handleCreateCollection(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentID")
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if body.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	col, err := h.store.CreateCollection(r.Context(), agentID, body.Name, body.Description)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, col)
}

func (h *KBHandler) handleGetCollection(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentID")
	colID := r.PathValue("collectionID")
	col, err := h.store.GetCollection(r.Context(), colID)
	if err != nil || col.AgentID != agentID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "collection not found"})
		return
	}
	writeJSON(w, http.StatusOK, col)
}

func (h *KBHandler) handleUpdateCollection(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentID")
	colID := r.PathValue("collectionID")
	// Verify ownership
	col, err := h.store.GetCollection(r.Context(), colID)
	if err != nil || col.AgentID != agentID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "collection not found"})
		return
	}
	var updates map[string]any
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	// Only allow safe fields
	allowed := map[string]bool{"name": true, "description": true, "status": true}
	safe := make(map[string]any)
	for k, v := range updates {
		if allowed[k] {
			safe[k] = v
		}
	}
	if err := h.store.UpdateCollection(r.Context(), colID, safe); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *KBHandler) handleDeleteCollection(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentID")
	colID := r.PathValue("collectionID")
	// Verify ownership
	col, err := h.store.GetCollection(r.Context(), colID)
	if err != nil || col.AgentID != agentID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "collection not found"})
		return
	}
	if err := h.store.DeleteCollection(r.Context(), colID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Document handlers ---

func (h *KBHandler) handleListDocuments(w http.ResponseWriter, r *http.Request) {
	colID := r.PathValue("collectionID")
	docs, err := h.store.ListDocuments(r.Context(), colID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if docs == nil {
		docs = []store.KBDocument{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"documents": docs})
}

var allowedKBMimeTypes = map[string]bool{
	"text/plain":       true,
	"text/markdown":    true,
	"text/csv":         true,
	"application/pdf":  true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
}

func (h *KBHandler) handleUploadDocument(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentID")
	colID := r.PathValue("collectionID")

	r.Body = http.MaxBytesReader(w, r.Body, maxKBUploadSize)
	if err := r.ParseMultipartForm(maxKBUploadSize); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file too large or invalid form"})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing 'file' field"})
		return
	}
	defer file.Close()

	// Sanitize filename
	filename := filepath.Base(header.Filename)
	filename = strings.ReplaceAll(filename, "..", "")

	// Detect MIME type from extension
	mimeType := detectKBMimeType(filename)
	if !allowedKBMimeTypes[mimeType] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported file type: " + mimeType})
		return
	}

	// Save to temp file
	tmpFile, err := os.CreateTemp("", "kb-upload-*"+filepath.Ext(filename))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "create temp file failed"})
		return
	}
	defer os.Remove(tmpFile.Name())

	n, err := io.Copy(tmpFile, file)
	tmpFile.Close()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "save upload failed"})
		return
	}

	// Compute content hash before SaveFile (which may remove the temp file)
	content, _ := os.ReadFile(tmpFile.Name())
	hash := memory.ContentHash(string(content))

	// Save to media storage
	storageKey, err := h.storage.SaveFile("kb-"+colID, tmpFile.Name(), mimeType)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "storage save failed"})
		return
	}

	// Create document record
	doc := &store.KBDocument{
		CollectionID: colID,
		AgentID:      agentID,
		Filename:     filename,
		MimeType:     mimeType,
		FileSize:     n,
		StorageKey:   storageKey,
		ContentHash:  hash,
	}
	if err := h.store.CreateDocument(r.Context(), doc); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Process async
	go func() {
		ctx := context.Background()
		if err := h.processor.ProcessDocument(ctx, doc.ID); err != nil {
			slog.Warn("kb.document.process_failed", "doc_id", doc.ID, "error", err)
		}
	}()

	writeJSON(w, http.StatusAccepted, doc)
}

func (h *KBHandler) handleGetDocument(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentID")
	docID := r.PathValue("documentID")
	doc, err := h.store.GetDocument(r.Context(), docID)
	if err != nil || doc.AgentID != agentID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "document not found"})
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func (h *KBHandler) handleDeleteDocument(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentID")
	docID := r.PathValue("documentID")
	// Verify ownership
	doc, err := h.store.GetDocument(r.Context(), docID)
	if err != nil || doc.AgentID != agentID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "document not found"})
		return
	}
	if err := h.store.DeleteDocument(r.Context(), docID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *KBHandler) handleReprocess(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentID")
	docID := r.PathValue("documentID")
	// Verify ownership
	doc, err := h.store.GetDocument(r.Context(), docID)
	if err != nil || doc.AgentID != agentID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "document not found"})
		return
	}
	go func() {
		ctx := context.Background()
		if err := h.processor.ProcessDocument(ctx, docID); err != nil {
			slog.Warn("kb.document.reprocess_failed", "doc_id", docID, "error", err)
		}
	}()
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "processing"})
}

// --- Chunk handler ---

func (h *KBHandler) handleListChunks(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentID")
	docID := r.PathValue("documentID")
	// Verify ownership
	doc, err := h.store.GetDocument(r.Context(), docID)
	if err != nil || doc.AgentID != agentID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "document not found"})
		return
	}
	chunks, err := h.store.ListChunks(r.Context(), docID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if chunks == nil {
		chunks = []store.KBChunk{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"chunks": chunks})
}

// --- Search handler ---

func (h *KBHandler) handleSearch(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentID")
	var body struct {
		Query         string   `json:"query"`
		CollectionIDs []string `json:"collection_ids"`
		MaxResults    int      `json:"max_results"`
		MinScore      float64  `json:"min_score"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if body.Query == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "query is required"})
		return
	}

	results, err := h.store.Search(r.Context(), body.Query, agentID, store.KBSearchOptions{
		CollectionIDs: body.CollectionIDs,
		MaxResults:    body.MaxResults,
		MinScore:      body.MinScore,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if results == nil {
		results = []store.KBSearchResult{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results, "count": len(results)})
}

// detectKBMimeType returns MIME type based on file extension.
func detectKBMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".txt":
		return "text/plain"
	case ".md", ".markdown":
		return "text/markdown"
	case ".csv":
		return "text/csv"
	case ".pdf":
		return "application/pdf"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	default:
		return "application/octet-stream"
	}
}
