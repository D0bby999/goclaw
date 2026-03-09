package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

// NotificationTool lets agents create, list, and dismiss notifications.
type NotificationTool struct {
	notifications store.NotificationStore
	eventPub      bus.EventPublisher
}

func NewNotificationTool(ns store.NotificationStore, ep bus.EventPublisher) *NotificationTool {
	return &NotificationTool{notifications: ns, eventPub: ep}
}

func (t *NotificationTool) Name() string { return "notification" }

func (t *NotificationTool) Description() string {
	return `Manage user notifications. Actions:
- send: Create and deliver a notification to a user
- list: List recent notifications for a user
- dismiss: Mark a notification as read

Use this to notify users about completed tasks, important updates, errors, or any event worth their attention.`
}

func (t *NotificationTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"send", "list", "dismiss"},
				"description": "Action to perform",
			},
			"user_id": map[string]interface{}{
				"type":        "string",
				"description": "Target user ID (default: 'admin')",
			},
			"title": map[string]interface{}{
				"type":        "string",
				"description": "[send] Notification title",
			},
			"message": map[string]interface{}{
				"type":        "string",
				"description": "[send] Notification body text",
			},
			"type": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"info", "success", "warning", "error"},
				"description": "[send] Notification type/severity (default: 'info')",
			},
			"id": map[string]interface{}{
				"type":        "string",
				"description": "[dismiss] Notification UUID to mark as read",
			},
			"limit": map[string]interface{}{
				"type":        "number",
				"description": "[list] Max results (default 20)",
			},
		},
		"required": []string{"action"},
	}
}

func (t *NotificationTool) Execute(ctx context.Context, args map[string]interface{}) *Result {
	action, _ := args["action"].(string)
	switch action {
	case "send":
		return t.execSend(ctx, args)
	case "list":
		return t.execList(ctx, args)
	case "dismiss":
		return t.execDismiss(ctx, args)
	default:
		return ErrorResult("action must be one of: send, list, dismiss")
	}
}

func (t *NotificationTool) execSend(ctx context.Context, args map[string]interface{}) *Result {
	title, _ := args["title"].(string)
	if title == "" {
		return ErrorResult("title is required for send action")
	}

	userID, _ := args["user_id"].(string)
	if userID == "" {
		userID = "admin"
	}
	message, _ := args["message"].(string)
	nType, _ := args["type"].(string)
	if nType == "" {
		nType = "info"
	}

	agentID := store.AgentIDFromContext(ctx)
	n := &store.Notification{
		ID:        uuid.New(),
		UserID:    userID,
		Type:      nType,
		Title:     title,
		Message:   message,
		CreatedAt: time.Now().UTC(),
	}
	if agentID != uuid.Nil {
		n.AgentID = &agentID
	}

	if err := t.notifications.Create(ctx, n); err != nil {
		return ErrorResult(fmt.Sprintf("failed to create notification: %v", err))
	}

	// Broadcast WS event so frontend picks it up in real time
	if t.eventPub != nil {
		payload := map[string]interface{}{
			"id":      n.ID.String(),
			"type":    n.Type,
			"title":   n.Title,
			"message": n.Message,
			"userId":  n.UserID,
		}
		raw, _ := json.Marshal(payload)
		t.eventPub.Broadcast(bus.Event{
			Name:    protocol.EventNotification,
			Payload: json.RawMessage(raw),
		})
	}

	return NewResult(fmt.Sprintf("Notification sent to %s: %s", userID, title))
}

func (t *NotificationTool) execList(ctx context.Context, args map[string]interface{}) *Result {
	userID, _ := args["user_id"].(string)
	if userID == "" {
		userID = "admin"
	}
	limit := 20
	if v, ok := args["limit"].(float64); ok && v > 0 {
		limit = int(v)
	}

	items, err := t.notifications.List(ctx, userID, limit, 0)
	if err != nil {
		return ErrorResult(fmt.Sprintf("failed to list notifications: %v", err))
	}
	if len(items) == 0 {
		return NewResult("No notifications found.")
	}

	b, _ := json.Marshal(items)
	return NewResult(fmt.Sprintf("Found %d notifications:\n%s", len(items), string(b)))
}

func (t *NotificationTool) execDismiss(ctx context.Context, args map[string]interface{}) *Result {
	idStr, _ := args["id"].(string)
	if idStr == "" {
		return ErrorResult("id is required for dismiss action")
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return ErrorResult("invalid notification id")
	}

	if err := t.notifications.MarkRead(ctx, id); err != nil {
		return ErrorResult(fmt.Sprintf("failed to dismiss notification: %v", err))
	}
	return NewResult("Notification dismissed.")
}
