package methods

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/nextlevelbuilder/goclaw/internal/gateway"
	"github.com/nextlevelbuilder/goclaw/internal/kb"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

// KBMethods handles kb.* RPC methods.
type KBMethods struct {
	store     store.KBStore
	processor *kb.Processor
}

func NewKBMethods(s store.KBStore, proc *kb.Processor) *KBMethods {
	return &KBMethods{store: s, processor: proc}
}

func (m *KBMethods) Register(router *gateway.MethodRouter) {
	router.Register(protocol.MethodKBCollectionsList, m.handleCollectionsList)
	router.Register(protocol.MethodKBCollectionsCreate, m.handleCollectionsCreate)
	router.Register(protocol.MethodKBCollectionsUpdate, m.handleCollectionsUpdate)
	router.Register(protocol.MethodKBCollectionsDelete, m.handleCollectionsDelete)
	router.Register(protocol.MethodKBDocumentsList, m.handleDocumentsList)
	router.Register(protocol.MethodKBDocumentsGet, m.handleDocumentsGet)
	router.Register(protocol.MethodKBDocumentsDelete, m.handleDocumentsDelete)
	router.Register(protocol.MethodKBDocumentsReprocess, m.handleDocumentsReprocess)
	router.Register(protocol.MethodKBSearch, m.handleSearch)
}

func (m *KBMethods) handleCollectionsList(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		AgentID string `json:"agentId"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}
	if params.AgentID == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "agentId is required"))
		return
	}
	cols, err := m.store.ListCollections(ctx, params.AgentID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	if cols == nil {
		cols = []store.KBCollection{}
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"collections": cols}))
}

func (m *KBMethods) handleCollectionsCreate(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		AgentID     string `json:"agentId"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}
	if params.AgentID == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "agentId is required"))
		return
	}
	if params.Name == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "name is required"))
		return
	}
	col, err := m.store.CreateCollection(ctx, params.AgentID, params.Name, params.Description)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, col))
}

func (m *KBMethods) handleCollectionsUpdate(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		ID      string         `json:"id"`
		Updates map[string]any `json:"updates"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}
	if params.ID == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "id is required"))
		return
	}
	safe := make(map[string]any)
	for k, v := range params.Updates {
		switch k {
		case "name", "description", "status":
			safe[k] = v
		}
	}
	if err := m.store.UpdateCollection(ctx, params.ID, safe); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]string{"ok": "true"}))
}

func (m *KBMethods) handleCollectionsDelete(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		ID string `json:"id"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}
	if params.ID == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "id is required"))
		return
	}
	if err := m.store.DeleteCollection(ctx, params.ID); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]string{"ok": "true"}))
}

func (m *KBMethods) handleDocumentsList(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		CollectionID string `json:"collection_id"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}
	if params.CollectionID == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "collection_id is required"))
		return
	}
	docs, err := m.store.ListDocuments(ctx, params.CollectionID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	if docs == nil {
		docs = []store.KBDocument{}
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"documents": docs}))
}

func (m *KBMethods) handleDocumentsGet(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		ID string `json:"id"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}
	if params.ID == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "id is required"))
		return
	}
	doc, err := m.store.GetDocument(ctx, params.ID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrNotFound, "document not found"))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, doc))
}

func (m *KBMethods) handleDocumentsDelete(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		ID string `json:"id"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}
	if params.ID == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "id is required"))
		return
	}
	if err := m.store.DeleteDocument(ctx, params.ID); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]string{"ok": "true"}))
}

func (m *KBMethods) handleDocumentsReprocess(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		ID string `json:"id"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}
	if params.ID == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "id is required"))
		return
	}
	go func() {
		bgCtx := context.Background()
		if err := m.processor.ProcessDocument(bgCtx, params.ID); err != nil {
			slog.Warn("kb.reprocess_failed", "doc_id", params.ID, "error", err)
		}
	}()
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]string{"status": "processing"}))
}

func (m *KBMethods) handleSearch(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		AgentID       string   `json:"agentId"`
		Query         string   `json:"query"`
		CollectionIDs []string `json:"collection_ids"`
		MaxResults    int      `json:"max_results"`
		MinScore      float64  `json:"min_score"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}
	if params.AgentID == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "agentId is required"))
		return
	}
	if params.Query == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "query is required"))
		return
	}
	results, err := m.store.Search(ctx, params.Query, params.AgentID, store.KBSearchOptions{
		CollectionIDs: params.CollectionIDs,
		MaxResults:    params.MaxResults,
		MinScore:      params.MinScore,
	})
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	if results == nil {
		results = []store.KBSearchResult{}
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"results": results, "count": len(results)}))
}
