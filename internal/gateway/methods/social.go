package methods

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/gateway"
	"github.com/nextlevelbuilder/goclaw/internal/social"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

// SocialMethods handles social.* RPC methods.
type SocialMethods struct {
	store   store.SocialStore
	manager *social.Manager
}

func NewSocialMethods(store store.SocialStore, manager *social.Manager) *SocialMethods {
	return &SocialMethods{store: store, manager: manager}
}

func (m *SocialMethods) Register(router *gateway.MethodRouter) {
	router.Register(protocol.MethodSocialAccountsList, m.handleAccountsList)
	router.Register(protocol.MethodSocialAccountsCreate, m.handleAccountsCreate)
	router.Register(protocol.MethodSocialAccountsUpdate, m.handleAccountsUpdate)
	router.Register(protocol.MethodSocialAccountsDelete, m.handleAccountsDelete)
	router.Register(protocol.MethodSocialPostsList, m.handlePostsList)
	router.Register(protocol.MethodSocialPostsCreate, m.handlePostsCreate)
	router.Register(protocol.MethodSocialPostsGet, m.handlePostsGet)
	router.Register(protocol.MethodSocialPostsUpdate, m.handlePostsUpdate)
	router.Register(protocol.MethodSocialPostsDelete, m.handlePostsDelete)
	router.Register(protocol.MethodSocialPostsPublish, m.handlePostsPublish)
	router.Register(protocol.MethodSocialTargetsAdd, m.handleTargetsAdd)
	router.Register(protocol.MethodSocialTargetsRemove, m.handleTargetsRemove)
	router.Register(protocol.MethodSocialMediaAdd, m.handleMediaAdd)
	router.Register(protocol.MethodSocialMediaRemove, m.handleMediaRemove)
}

// --- Accounts ---

func (m *SocialMethods) handleAccountsList(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		Platform string `json:"platform"`
	}
	_ = json.Unmarshal(req.Params, &params)

	var accounts []store.SocialAccountData
	var err error
	if params.Platform != "" {
		accounts, err = m.store.ListAccountsByPlatform(context.Background(), client.UserID(), params.Platform)
	} else {
		accounts, err = m.store.ListAccounts(context.Background(), client.UserID())
	}
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	if accounts == nil {
		accounts = []store.SocialAccountData{}
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"accounts": accounts, "count": len(accounts)}))
}

func (m *SocialMethods) handleAccountsCreate(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var a store.SocialAccountData
	if err := json.Unmarshal(req.Params, &a); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid params"))
		return
	}
	if a.Platform == "" || a.PlatformUserID == "" || a.AccessToken == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "platform, platform_user_id, access_token required"))
		return
	}
	a.OwnerID = client.UserID()
	if err := m.store.CreateAccount(context.Background(), &a); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	a.AccessToken = ""
	a.RefreshToken = nil
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"account": a}))
}

func (m *SocialMethods) handleAccountsUpdate(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
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
	a, err := m.store.GetAccount(context.Background(), id)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "account not found"))
		return
	}
	if a.OwnerID != client.UserID() {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrForbidden, "forbidden"))
		return
	}
	delete(params.Updates, "id")
	delete(params.Updates, "owner_id")
	if err := m.store.UpdateAccount(context.Background(), id, params.Updates); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]string{"status": "ok"}))
}

func (m *SocialMethods) handleAccountsDelete(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
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
	a, err := m.store.GetAccount(context.Background(), id)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, "account not found"))
		return
	}
	if a.OwnerID != client.UserID() {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrForbidden, "forbidden"))
		return
	}
	if err := m.store.DeleteAccount(context.Background(), id); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]string{"status": "deleted"}))
}
