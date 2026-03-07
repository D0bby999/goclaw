package methods

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/gateway"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

// --- Posts ---

func (m *SocialMethods) handlePostsList(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		Status string `json:"status"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	}
	_ = json.Unmarshal(req.Params, &params)
	if params.Limit <= 0 {
		params.Limit = 50
	}

	posts, total, err := m.store.ListPosts(context.Background(), client.UserID(), params.Status, params.Limit, params.Offset)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	if posts == nil {
		posts = []store.SocialPostData{}
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"posts": posts, "total": total}))
}

func (m *SocialMethods) handlePostsCreate(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var p store.SocialPostData
	if err := json.Unmarshal(req.Params, &p); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid params"))
		return
	}
	if p.Content == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "content required"))
		return
	}
	p.OwnerID = client.UserID()
	if err := m.store.CreatePost(context.Background(), &p); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"post": p}))
}

func (m *SocialMethods) handlePostsGet(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil || params.ID == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "id required"))
		return
	}
	id, err := uuid.Parse(params.ID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid id"))
		return
	}
	p, err := m.store.GetPost(context.Background(), id)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	if p.OwnerID != client.UserID() {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrForbidden, "forbidden"))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"post": p}))
}

func (m *SocialMethods) handlePostsUpdate(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		ID      string         `json:"id"`
		Updates map[string]any `json:"updates"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil || params.ID == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "id and updates required"))
		return
	}
	id, err := uuid.Parse(params.ID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid id"))
		return
	}
	p, err := m.store.GetPost(context.Background(), id)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "post not found"))
		return
	}
	if p.OwnerID != client.UserID() {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrForbidden, "forbidden"))
		return
	}
	delete(params.Updates, "id")
	delete(params.Updates, "owner_id")
	if err := m.store.UpdatePost(context.Background(), id, params.Updates); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]string{"status": "ok"}))
}

func (m *SocialMethods) handlePostsDelete(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil || params.ID == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "id required"))
		return
	}
	id, err := uuid.Parse(params.ID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid id"))
		return
	}
	p, err := m.store.GetPost(context.Background(), id)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "post not found"))
		return
	}
	if p.OwnerID != client.UserID() {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrForbidden, "forbidden"))
		return
	}
	if err := m.store.DeletePost(context.Background(), id); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]string{"status": "deleted"}))
}

func (m *SocialMethods) handlePostsPublish(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil || params.ID == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "id required"))
		return
	}
	id, err := uuid.Parse(params.ID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid id"))
		return
	}
	p, err := m.store.GetPost(context.Background(), id)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "post not found"))
		return
	}
	if p.OwnerID != client.UserID() {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrForbidden, "forbidden"))
		return
	}
	if err := m.manager.PublishPost(context.Background(), id); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	p, _ = m.store.GetPost(context.Background(), id)
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"post": p}))
}

// --- Targets ---

func (m *SocialMethods) handleTargetsAdd(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		PostID    string `json:"post_id"`
		AccountID string `json:"account_id"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil || params.PostID == "" || params.AccountID == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "post_id and account_id required"))
		return
	}
	postID, _ := uuid.Parse(params.PostID)
	accountID, _ := uuid.Parse(params.AccountID)
	t := &store.SocialPostTargetData{PostID: postID, AccountID: accountID}
	if err := m.store.AddTarget(context.Background(), t); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"target": t}))
}

func (m *SocialMethods) handleTargetsRemove(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil || params.ID == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "id required"))
		return
	}
	id, _ := uuid.Parse(params.ID)
	if err := m.store.RemoveTarget(context.Background(), id); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]string{"status": "deleted"}))
}

// --- Media ---

func (m *SocialMethods) handleMediaAdd(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		PostID    string `json:"post_id"`
		MediaType string `json:"media_type"`
		URL       string `json:"url"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil || params.PostID == "" || params.URL == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "post_id, url required"))
		return
	}
	postID, _ := uuid.Parse(params.PostID)
	media := &store.SocialPostMediaData{PostID: postID, MediaType: params.MediaType, URL: params.URL}
	if err := m.store.AddMedia(context.Background(), media); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"media": media}))
}

func (m *SocialMethods) handleMediaRemove(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil || params.ID == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "id required"))
		return
	}
	id, _ := uuid.Parse(params.ID)
	if err := m.store.RemoveMedia(context.Background(), id); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]string{"status": "deleted"}))
}
