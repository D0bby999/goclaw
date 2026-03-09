package methods

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/gateway"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

func (m *ContentScheduleMethods) handleUpdate(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		ID             string      `json:"id"`
		Name           *string     `json:"name"`
		CronExpression *string     `json:"cronExpression"`
		Timezone       *string     `json:"timezone"`
		Prompt         *string     `json:"prompt"`
		PageIDs        []uuid.UUID `json:"pageIds"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	id, err := uuid.Parse(params.ID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid id"))
		return
	}

	existing, err := m.store.Get(context.Background(), id)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrNotFound, "schedule not found"))
		return
	}
	if existing.OwnerID != client.UserID() {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrForbidden, "forbidden"))
		return
	}

	updates := map[string]any{}
	if params.Name != nil {
		updates["name"] = *params.Name
	}
	if params.CronExpression != nil {
		updates["cron_expression"] = *params.CronExpression
	}
	if params.Timezone != nil {
		updates["timezone"] = *params.Timezone
	}
	if params.Prompt != nil {
		updates["prompt"] = *params.Prompt
	}

	if len(updates) > 0 {
		if err := m.store.Update(context.Background(), id, updates); err != nil {
			client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
			return
		}
	}

	// If schedule changed, update cron job too
	if (params.CronExpression != nil || params.Timezone != nil) && existing.CronJobID != nil {
		expr := existing.CronExpression
		tz := existing.Timezone
		if params.CronExpression != nil {
			expr = *params.CronExpression
		}
		if params.Timezone != nil {
			tz = *params.Timezone
		}
		patch := store.CronJobPatch{
			Schedule: &store.CronSchedule{Kind: "cron", Expr: expr, TZ: tz},
		}
		_, _ = m.cronSvc.UpdateJob(*existing.CronJobID, patch)
	}

	if params.PageIDs != nil {
		_ = m.store.SetPages(context.Background(), id, params.PageIDs)
	}

	updated, err := m.store.Get(context.Background(), id)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"schedule": updated}))
}

func (m *ContentScheduleMethods) handleDelete(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		ID string `json:"id"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	id, err := uuid.Parse(params.ID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid id"))
		return
	}

	existing, err := m.store.Get(context.Background(), id)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrNotFound, "schedule not found"))
		return
	}
	if existing.OwnerID != client.UserID() {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrForbidden, "forbidden"))
		return
	}

	if err := m.store.Delete(context.Background(), id); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}

	if existing.CronJobID != nil {
		_ = m.cronSvc.RemoveJob(*existing.CronJobID)
	}

	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]string{"status": "deleted"}))
}

func (m *ContentScheduleMethods) handleToggle(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	id, err := uuid.Parse(params.ID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid id"))
		return
	}

	existing, err := m.store.Get(context.Background(), id)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrNotFound, "schedule not found"))
		return
	}
	if existing.OwnerID != client.UserID() {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrForbidden, "forbidden"))
		return
	}

	if err := m.store.Update(context.Background(), id, map[string]any{"enabled": params.Enabled}); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}

	if existing.CronJobID != nil {
		_ = m.cronSvc.EnableJob(*existing.CronJobID, params.Enabled)
	}

	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"id": params.ID, "enabled": params.Enabled}))
}

func (m *ContentScheduleMethods) handleLogs(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		ScheduleID string `json:"scheduleId"`
		Limit      int    `json:"limit"`
		Offset     int    `json:"offset"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	scheduleID, err := uuid.Parse(params.ScheduleID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid scheduleId"))
		return
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}

	logs, total, err := m.store.ListLogs(context.Background(), scheduleID, limit, params.Offset)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	if logs == nil {
		logs = []store.ContentScheduleLogData{}
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"logs": logs, "total": total}))
}
