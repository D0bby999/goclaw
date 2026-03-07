package methods

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/gateway"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

// NewsMethods handles news.sources.* and news.items.* WS RPC methods.
type NewsMethods struct {
	news store.NewsStore
}

func NewNewsMethods(news store.NewsStore) *NewsMethods {
	return &NewsMethods{news: news}
}

func (m *NewsMethods) Register(router *gateway.MethodRouter) {
	router.Register(protocol.MethodNewsSourcesList, m.handleSourcesList)
	router.Register(protocol.MethodNewsSourcesCreate, m.handleSourcesCreate)
	router.Register(protocol.MethodNewsSourcesUpdate, m.handleSourcesUpdate)
	router.Register(protocol.MethodNewsSourcesDelete, m.handleSourcesDelete)
	router.Register(protocol.MethodNewsItemsList, m.handleItemsList)
	router.Register(protocol.MethodNewsItemsStats, m.handleItemsStats)
}

func (m *NewsMethods) handleSourcesList(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		AgentID     string `json:"agentId"`
		EnabledOnly bool   `json:"enabledOnly"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	agentID, err := uuid.Parse(params.AgentID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "valid agentId is required"))
		return
	}

	sources, err := m.news.ListSources(ctx, agentID, params.EnabledOnly)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}

	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]interface{}{
		"sources": sources,
	}))
}

func (m *NewsMethods) handleSourcesCreate(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		AgentID        string          `json:"agentId"`
		Name           string          `json:"name"`
		SourceType     string          `json:"sourceType"`
		Config         json.RawMessage `json:"config"`
		ScrapeInterval string          `json:"scrapeInterval"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	agentID, err := uuid.Parse(params.AgentID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "valid agentId is required"))
		return
	}
	if params.Name == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "name is required"))
		return
	}
	if params.SourceType == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "sourceType is required (reddit, website, twitter, rss)"))
		return
	}

	src := &store.NewsSource{
		AgentID:        agentID,
		Name:           params.Name,
		SourceType:     params.SourceType,
		Config:         params.Config,
		Enabled:        true,
		ScrapeInterval: params.ScrapeInterval,
	}
	if err := m.news.CreateSource(ctx, src); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}

	client.SendResponse(protocol.NewOKResponse(req.ID, src))
}

func (m *NewsMethods) handleSourcesUpdate(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		ID    string         `json:"id"`
		Patch map[string]any `json:"patch"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	id, err := uuid.Parse(params.ID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "valid id is required"))
		return
	}

	if err := m.news.UpdateSource(ctx, id, params.Patch); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}

	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]string{"status": "updated"}))
}

func (m *NewsMethods) handleSourcesDelete(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		ID string `json:"id"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	id, err := uuid.Parse(params.ID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "valid id is required"))
		return
	}

	if err := m.news.DeleteSource(ctx, id); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}

	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]string{"status": "deleted"}))
}

func (m *NewsMethods) handleItemsList(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		AgentID    string   `json:"agentId"`
		SourceID   string   `json:"sourceId"`
		Categories []string `json:"categories"`
		Since      string   `json:"since"`
		Limit      int      `json:"limit"`
		Offset     int      `json:"offset"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	agentID, err := uuid.Parse(params.AgentID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "valid agentId is required"))
		return
	}

	filter := store.NewsItemFilter{
		AgentID:    agentID,
		Categories: params.Categories,
		Limit:      params.Limit,
		Offset:     params.Offset,
	}

	if params.SourceID != "" {
		if u, err := uuid.Parse(params.SourceID); err == nil {
			filter.SourceID = &u
		}
	}

	items, err := m.news.ListItems(ctx, filter)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}

	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]interface{}{
		"items": items,
		"count": len(items),
	}))
}

func (m *NewsMethods) handleItemsStats(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		AgentID string `json:"agentId"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	agentID, err := uuid.Parse(params.AgentID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "valid agentId is required"))
		return
	}

	total, err := m.news.CountItems(ctx, agentID, nil)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}

	sources, err := m.news.ListSources(ctx, agentID, false)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}

	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]interface{}{
		"totalItems":   total,
		"totalSources": len(sources),
	}))
}
