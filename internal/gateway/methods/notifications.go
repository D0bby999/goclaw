package methods

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/gateway"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

// NotificationsMethods handles notification-related RPC methods.
type NotificationsMethods struct {
	notifications store.NotificationStore
}

func NewNotificationsMethods(notifications store.NotificationStore) *NotificationsMethods {
	return &NotificationsMethods{notifications: notifications}
}

func (m *NotificationsMethods) Register(router *gateway.MethodRouter) {
	router.Register(protocol.MethodNotificationsList, m.handleList)
	router.Register(protocol.MethodNotificationsUnread, m.handleUnreadCount)
	router.Register(protocol.MethodNotificationsRead, m.handleMarkRead)
	router.Register(protocol.MethodNotificationsReadAll, m.handleMarkAllRead)
}

func (m *NotificationsMethods) handleList(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		UserID string `json:"userId"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}
	if params.UserID == "" {
		params.UserID = client.UserID()
	}
	if params.UserID == "" {
		params.UserID = "admin"
	}
	if params.Limit <= 0 {
		params.Limit = 50
	}

	items, err := m.notifications.List(ctx, params.UserID, params.Limit, params.Offset)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	if items == nil {
		items = []store.Notification{}
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]interface{}{
		"notifications": items,
	}))
}

func (m *NotificationsMethods) handleUnreadCount(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		UserID string `json:"userId"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}
	if params.UserID == "" {
		params.UserID = client.UserID()
	}
	if params.UserID == "" {
		params.UserID = "admin"
	}

	count, err := m.notifications.CountUnread(ctx, params.UserID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]interface{}{
		"count": count,
	}))
}

func (m *NotificationsMethods) handleMarkRead(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		ID string `json:"id"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}
	id, err := uuid.Parse(params.ID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid notification id"))
		return
	}

	if err := m.notifications.MarkRead(ctx, id); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]string{"status": "ok"}))
}

func (m *NotificationsMethods) handleMarkAllRead(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		UserID string `json:"userId"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}
	if params.UserID == "" {
		params.UserID = client.UserID()
	}
	if params.UserID == "" {
		params.UserID = "admin"
	}

	if err := m.notifications.MarkAllRead(ctx, params.UserID); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]string{"status": "ok"}))
}
