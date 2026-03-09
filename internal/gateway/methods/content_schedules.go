package methods

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/gateway"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

// ContentScheduleMethods handles schedules.* RPC methods.
type ContentScheduleMethods struct {
	store   store.ContentScheduleStore
	cronSvc store.CronStore
}

func NewContentScheduleMethods(s store.ContentScheduleStore, cron store.CronStore) *ContentScheduleMethods {
	return &ContentScheduleMethods{store: s, cronSvc: cron}
}

func (m *ContentScheduleMethods) Register(router *gateway.MethodRouter) {
	router.Register(protocol.MethodSchedulesList, m.handleList)
	router.Register(protocol.MethodSchedulesCreate, m.handleCreate)
	router.Register(protocol.MethodSchedulesGet, m.handleGet)
	router.Register(protocol.MethodSchedulesUpdate, m.handleUpdate)
	router.Register(protocol.MethodSchedulesDelete, m.handleDelete)
	router.Register(protocol.MethodSchedulesToggle, m.handleToggle)
	router.Register(protocol.MethodSchedulesLogs, m.handleLogs)
}

func (m *ContentScheduleMethods) handleList(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		Enabled *bool `json:"enabled"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	schedules, err := m.store.List(context.Background(), client.UserID(), params.Enabled)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	if schedules == nil {
		schedules = []store.ContentScheduleData{}
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"schedules": schedules, "count": len(schedules)}))
}

func (m *ContentScheduleMethods) handleCreate(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params struct {
		Name           string      `json:"name"`
		CronExpression string      `json:"cronExpression"`
		Timezone       string      `json:"timezone"`
		ContentSource  string      `json:"contentSource"`
		AgentID        *string     `json:"agentId"`
		Prompt         *string     `json:"prompt"`
		PageIDs        []uuid.UUID `json:"pageIds"`
	}
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	if params.Name == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "name is required"))
		return
	}
	if params.CronExpression == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "cronExpression is required"))
		return
	}
	tz := params.Timezone
	if tz == "" {
		tz = "UTC"
	}
	if _, err := time.LoadLocation(tz); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid timezone: "+err.Error()))
		return
	}

	data := store.ContentScheduleData{
		OwnerID:        client.UserID(),
		Name:           params.Name,
		Enabled:        true,
		ContentSource:  params.ContentSource,
		Prompt:         params.Prompt,
		CronExpression: params.CronExpression,
		Timezone:       tz,
	}
	if params.AgentID != nil {
		aid, err := uuid.Parse(*params.AgentID)
		if err != nil {
			client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid agentId"))
			return
		}
		data.AgentID = &aid
	}

	if err := m.store.Create(context.Background(), &data); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}

	// Create backing cron job
	jobName := "sched-" + data.ID.String()[:8]
	msg := "{internal:content_schedule:" + data.ID.String() + "}"
	sched := store.CronSchedule{Kind: "cron", Expr: params.CronExpression, TZ: tz}
	job, err := m.cronSvc.AddJob(jobName, sched, msg, false, "", "", "", "")
	if err == nil {
		_ = m.store.Update(context.Background(), data.ID, map[string]any{"cron_job_id": job.ID})
		data.CronJobID = &job.ID
	}

	if len(params.PageIDs) > 0 {
		_ = m.store.SetPages(context.Background(), data.ID, params.PageIDs)
	}

	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"schedule": data}))
}

func (m *ContentScheduleMethods) handleGet(_ context.Context, client *gateway.Client, req *protocol.RequestFrame) {
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

	s, err := m.store.Get(context.Background(), id)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrNotFound, "schedule not found"))
		return
	}
	if s.OwnerID != client.UserID() {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrForbidden, "forbidden"))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"schedule": s}))
}
