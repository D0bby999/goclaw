package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"

	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/claudecode"
)

const maxProjectSessionsInList = 10

// handleProjectCallback routes callback queries for project commands.
func (c *Channel) handleProjectCallback(ctx context.Context, query *telego.CallbackQuery) {
	chat := query.Message.GetChat()
	chatID := chat.ID
	parts := strings.SplitN(query.Data, ":", 3)
	if len(parts) < 2 {
		return
	}

	action := parts[1]
	switch action {
	case "back":
		// Back to projects list
		c.handleProjectsList(ctx, chatID, func(msg *telego.SendMessageParams) {
			msg.ChatID = tu.ID(chatID)
		})
	case "proj":
		// Show sessions for a project
		if len(parts) >= 3 {
			c.handleProjectSelected(ctx, chatID, parts[2])
		}
	case "sess":
		// Select/activate a session
		if len(parts) >= 3 {
			c.handleProjectSessionSelected(ctx, chatID, parts[2])
		}
	case "new":
		// Start new session (user will send prompt next)
		if len(parts) >= 3 {
			c.handleProjectNewSession(ctx, chatID, parts[2])
		}
	case "stop":
		// Stop a running session
		if len(parts) >= 3 {
			c.handleProjectStopSession(ctx, chatID, parts[2])
		}
	case "del":
		// Delete a session
		if len(parts) >= 3 {
			c.handleProjectDeleteSession(ctx, chatID, parts[2])
		}
	case "rename":
		// Start rename flow (user will send new label next)
		if len(parts) >= 3 {
			c.handleProjectRenameSession(ctx, chatID, parts[2])
		}
	case "logs":
		// Show recent session logs
		if len(parts) >= 3 {
			c.handleProjectViewLogs(ctx, chatID, parts[2])
		}
	}
}

// projectStartOpts builds StartOpts for a new project session.
func projectStartOpts(projectID uuid.UUID, prompt string) claudecode.StartOpts {
	return claudecode.StartOpts{
		ProjectID: projectID,
		Prompt:    prompt,
	}
}

// handleProjectsList lists available projects as inline keyboard buttons.
func (c *Channel) handleProjectsList(ctx context.Context, chatID int64, setThread func(*telego.SendMessageParams)) {
	chatIDObj := tu.ID(chatID)

	send := func(text string) {
		msg := tu.Message(chatIDObj, text)
		setThread(msg)
		c.bot.SendMessage(ctx, msg)
	}

	if c.projectStore == nil {
		send("Projects are not available.")
		return
	}

	projects, err := c.projectStore.ListProjects(ctx, "")
	if err != nil {
		slog.Warn("projects: list failed", "error", err)
		send("Failed to list projects.")
		return
	}
	if len(projects) == 0 {
		send("No projects found.")
		return
	}

	var rows [][]telego.InlineKeyboardButton
	for _, p := range projects {
		if p.Status != "active" {
			continue
		}
		label := fmt.Sprintf("📁 %s", p.Name)
		rows = append(rows, []telego.InlineKeyboardButton{
			{Text: label, CallbackData: "pj:proj:" + p.ID.String()},
		})
	}

	if len(rows) == 0 {
		send("No active projects found.")
		return
	}

	msg := tu.Message(chatIDObj, "Select a project:")
	setThread(msg)
	msg.ReplyMarkup = &telego.InlineKeyboardMarkup{InlineKeyboard: rows}
	c.bot.SendMessage(ctx, msg)
}

// handleProjectSelected shows sessions for a project + option to create new.
func (c *Channel) handleProjectSelected(ctx context.Context, chatID int64, projectIDStr string) {
	chatIDObj := tu.ID(chatID)

	send := func(text string) {
		msg := tu.Message(chatIDObj, text)
		c.bot.SendMessage(ctx, msg)
	}

	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		send("Invalid project ID.")
		return
	}

	proj, err := c.projectStore.GetProject(ctx, projectID)
	if err != nil {
		send("Project not found.")
		return
	}

	sessions, _, err := c.projectStore.ListSessions(ctx, projectID, maxProjectSessionsInList, 0)
	if err != nil {
		slog.Warn("projects: list sessions failed", "error", err)
		send("Failed to list sessions.")
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📁 <b>%s</b>\n", escapeHTML(proj.Name)))
	sb.WriteString(fmt.Sprintf("📂 <code>%s</code>\n\n", escapeHTML(proj.WorkDir)))

	if len(sessions) > 0 {
		sb.WriteString("Recent sessions:\n")
		for i, s := range sessions {
			icon := sessionStatusIcon(s.Status)
			label := s.Label
			if label == "" {
				label = s.ID.String()[:8]
			}
			if len(label) > 50 {
				label = label[:50] + "…"
			}
			sb.WriteString(fmt.Sprintf("%d. %s %s\n", i+1, icon, escapeHTML(label)))
		}
	} else {
		sb.WriteString("No sessions yet.\n")
	}

	// Build inline keyboard: recent sessions + "New Session" button
	var rows [][]telego.InlineKeyboardButton
	for i, s := range sessions {
		icon := sessionStatusIcon(s.Status)
		label := s.Label
		if label == "" {
			label = s.ID.String()[:8]
		}
		if len([]rune(label)) > 30 {
			label = string([]rune(label)[:30]) + "…"
		}
		btnLabel := fmt.Sprintf("%d. %s %s", i+1, icon, label)
		rows = append(rows, []telego.InlineKeyboardButton{
			{Text: btnLabel, CallbackData: "pj:sess:" + s.ID.String()},
		})
	}
	rows = append(rows, []telego.InlineKeyboardButton{
		{Text: "➕ New Session", CallbackData: "pj:new:" + projectIDStr},
	})
	rows = append(rows, []telego.InlineKeyboardButton{
		{Text: "⬅️ Back to Projects", CallbackData: "pj:back"},
	})

	// Use HTML parse mode for inline keyboard
	msgParams := tu.Message(chatIDObj, sb.String())
	msgParams.ParseMode = "HTML"
	msgParams.ReplyMarkup = &telego.InlineKeyboardMarkup{InlineKeyboard: rows}
	c.bot.SendMessage(ctx, msgParams)
}

// handleProjectNewSession prompts user to send a message as the initial prompt.
func (c *Channel) handleProjectNewSession(ctx context.Context, chatID int64, projectIDStr string) {
	chatIDObj := tu.ID(chatID)

	// Store the pending project ID so next message becomes the initial prompt
	c.projectPending.Store(fmt.Sprintf("%d", chatID), projectIDStr)

	msg := tu.Message(chatIDObj, "Send your initial prompt for the new session.\n\nType your message below:")
	c.bot.SendMessage(ctx, msg)
}

// handleProjectSessionSelected shows session info and enables chat mode.
func (c *Channel) handleProjectSessionSelected(ctx context.Context, chatID int64, sessionIDStr string) {
	chatIDObj := tu.ID(chatID)

	send := func(text string) {
		msg := tu.Message(chatIDObj, text)
		msg.ParseMode = "HTML"
		c.bot.SendMessage(ctx, msg)
	}

	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		send("Invalid session ID.")
		return
	}

	sess, err := c.projectStore.GetSession(ctx, sessionID)
	if err != nil {
		send("Session not found.")
		return
	}

	// Set active project session for this chat
	c.projectActiveSession.Store(fmt.Sprintf("%d", chatID), sessionIDStr)

	icon := sessionStatusIcon(sess.Status)
	label := sess.Label
	if label == "" {
		label = sess.ID.String()[:8]
	}

	canChat := sess.ClaudeSessionID != nil
	var statusText string
	if canChat {
		statusText = "You can now send messages to this session."
	} else {
		statusText = "⚠️ Session has no resume ID — cannot send follow-up messages."
	}

	text := fmt.Sprintf(
		"%s <b>Session active</b>\n\n"+
			"Label: %s\n"+
			"Status: %s\n"+
			"Tokens: %d↑ %d↓\n"+
			"Cost: $%.4f\n\n"+
			"%s\n\nUse /project_exit to leave session mode.",
		icon, escapeHTML(label), sess.Status,
		sess.InputTokens, sess.OutputTokens, sess.CostUSD,
		statusText,
	)

	var rows [][]telego.InlineKeyboardButton
	if sess.Status == "running" || sess.Status == "starting" {
		rows = append(rows, []telego.InlineKeyboardButton{
			{Text: "⏹ Stop Session", CallbackData: "pj:stop:" + sessionIDStr},
		})
	}
	rows = append(rows, []telego.InlineKeyboardButton{
		{Text: "📋 View Logs", CallbackData: "pj:logs:" + sessionIDStr},
		{Text: "✏️ Rename", CallbackData: "pj:rename:" + sessionIDStr},
	})
	rows = append(rows, []telego.InlineKeyboardButton{
		{Text: "🗑 Delete", CallbackData: "pj:del:" + sessionIDStr},
		{Text: "⬅️ Back", CallbackData: "pj:proj:" + sess.ProjectID.String()},
	})

	msg := tu.Message(chatIDObj, text)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = &telego.InlineKeyboardMarkup{InlineKeyboard: rows}
	c.bot.SendMessage(ctx, msg)
}

// handleProjectStopSession stops a running project session.
func (c *Channel) handleProjectStopSession(ctx context.Context, chatID int64, sessionIDStr string) {
	chatIDObj := tu.ID(chatID)

	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		c.bot.SendMessage(ctx, tu.Message(chatIDObj, "Invalid session ID."))
		return
	}

	if c.projectManager == nil {
		c.bot.SendMessage(ctx, tu.Message(chatIDObj, "Project manager not available."))
		return
	}

	if err := c.projectManager.Stop(ctx, sessionID); err != nil {
		c.bot.SendMessage(ctx, tu.Message(chatIDObj, fmt.Sprintf("Failed to stop session: %s", err.Error())))
		return
	}

	c.bot.SendMessage(ctx, tu.Message(chatIDObj, "⏹ Session stopped."))
}

// handleProjectMessage routes a text message to the active project session.
// Returns true if the message was handled (active session or pending new session).
func (c *Channel) handleProjectMessage(ctx context.Context, chatID int64, text string, setThread func(*telego.SendMessageParams)) bool {
	chatIDStr := fmt.Sprintf("%d", chatID)
	chatIDObj := tu.ID(chatID)

	// Check if user is renaming a session (pending rename)
	if sessionIDRaw, ok := c.projectPendingRename.LoadAndDelete(chatIDStr); ok {
		sessionIDStr := sessionIDRaw.(string)
		c.finishProjectRename(ctx, chatID, sessionIDStr, text)
		return true
	}

	// Check if user is creating a new session (pending project)
	if projectIDRaw, ok := c.projectPending.LoadAndDelete(chatIDStr); ok {
		projectIDStr := projectIDRaw.(string)
		c.startProjectSession(ctx, chatID, projectIDStr, text, setThread)
		return true
	}

	// Check if user has an active project session
	sessionIDRaw, ok := c.projectActiveSession.Load(chatIDStr)
	if !ok {
		return false
	}
	sessionIDStr := sessionIDRaw.(string)

	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		c.projectActiveSession.Delete(chatIDStr)
		return false
	}

	if c.projectManager == nil {
		c.bot.SendMessage(ctx, tu.Message(chatIDObj, "Project manager not available."))
		return true
	}

	// Send prompt to session
	msg := tu.Message(chatIDObj, "⏳ Sending prompt...")
	setThread(msg)
	c.bot.SendMessage(ctx, msg)

	if err := c.projectManager.SendPrompt(ctx, sessionID, text); err != nil {
		errMsg := tu.Message(chatIDObj, fmt.Sprintf("❌ Failed: %s", err.Error()))
		setThread(errMsg)
		c.bot.SendMessage(ctx, errMsg)
		return true
	}

	return true
}

// startProjectSession creates a new project session and activates it.
func (c *Channel) startProjectSession(ctx context.Context, chatID int64, projectIDStr, prompt string, setThread func(*telego.SendMessageParams)) {
	chatIDObj := tu.ID(chatID)
	chatIDStr := fmt.Sprintf("%d", chatID)

	send := func(text string) {
		msg := tu.Message(chatIDObj, text)
		setThread(msg)
		c.bot.SendMessage(ctx, msg)
	}

	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		send("Invalid project ID.")
		return
	}

	if c.projectManager == nil {
		send("Project manager not available.")
		return
	}

	send("⏳ Starting project session...")

	sessionID, err := c.projectManager.Start(ctx, projectStartOpts(projectID, prompt), chatIDStr)
	if err != nil {
		send(fmt.Sprintf("❌ Failed to start session: %s", err.Error()))
		return
	}

	// Activate this session for the chat
	c.projectActiveSession.Store(chatIDStr, sessionID.String())
	send(fmt.Sprintf("✅ Session started. Send messages to chat with the project.\nUse /project_exit to leave session mode."))
}

// handleProjectDeleteSession deletes a project session.
func (c *Channel) handleProjectDeleteSession(ctx context.Context, chatID int64, sessionIDStr string) {
	chatIDObj := tu.ID(chatID)

	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		c.bot.SendMessage(ctx, tu.Message(chatIDObj, "Invalid session ID."))
		return
	}

	// Get session to know project for "back" navigation
	sess, err := c.projectStore.GetSession(ctx, sessionID)
	if err != nil {
		c.bot.SendMessage(ctx, tu.Message(chatIDObj, "Session not found."))
		return
	}
	projectID := sess.ProjectID

	if err := c.projectManager.Delete(ctx, sessionID); err != nil {
		c.bot.SendMessage(ctx, tu.Message(chatIDObj, fmt.Sprintf("❌ Failed to delete: %s", err.Error())))
		return
	}

	// Clear active session if it was this one
	chatIDStr := fmt.Sprintf("%d", chatID)
	if activeRaw, ok := c.projectActiveSession.Load(chatIDStr); ok {
		if activeRaw.(string) == sessionIDStr {
			c.projectActiveSession.Delete(chatIDStr)
		}
	}

	c.bot.SendMessage(ctx, tu.Message(chatIDObj, "🗑 Session deleted."))
	// Show sessions list
	c.handleProjectSelected(ctx, chatID, projectID.String())
}

// handleProjectRenameSession prompts user to send new label.
func (c *Channel) handleProjectRenameSession(ctx context.Context, chatID int64, sessionIDStr string) {
	chatIDObj := tu.ID(chatID)
	chatIDStr := fmt.Sprintf("%d", chatID)
	c.projectPendingRename.Store(chatIDStr, sessionIDStr)
	c.bot.SendMessage(ctx, tu.Message(chatIDObj, "Send the new label for this session:"))
}

// finishProjectRename applies the rename to a session.
func (c *Channel) finishProjectRename(ctx context.Context, chatID int64, sessionIDStr, newLabel string) {
	chatIDObj := tu.ID(chatID)

	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		c.bot.SendMessage(ctx, tu.Message(chatIDObj, "Invalid session ID."))
		return
	}

	if err := c.projectStore.UpdateSession(ctx, sessionID, map[string]any{"label": newLabel}); err != nil {
		c.bot.SendMessage(ctx, tu.Message(chatIDObj, fmt.Sprintf("❌ Failed to rename: %s", err.Error())))
		return
	}

	c.bot.SendMessage(ctx, tu.Message(chatIDObj, fmt.Sprintf("✏️ Session renamed to: %s", newLabel)))
}

// handleProjectViewLogs shows recent session output.
func (c *Channel) handleProjectViewLogs(ctx context.Context, chatID int64, sessionIDStr string) {
	chatIDObj := tu.ID(chatID)

	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		c.bot.SendMessage(ctx, tu.Message(chatIDObj, "Invalid session ID."))
		return
	}

	logs, err := c.projectStore.GetLogs(ctx, sessionID, -1, 20)
	if err != nil {
		c.bot.SendMessage(ctx, tu.Message(chatIDObj, "Failed to fetch logs."))
		return
	}

	if len(logs) == 0 {
		c.bot.SendMessage(ctx, tu.Message(chatIDObj, "No logs for this session."))
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📋 <b>Recent logs</b> (%d entries)\n\n", len(logs)))

	for _, l := range logs {
		icon := logTypeIcon(l.EventType)
		summary := extractLogSummary(l.EventType, l.Content)
		sb.WriteString(fmt.Sprintf("%s %s\n", icon, escapeHTML(summary)))
		if sb.Len() > 3500 { // Leave room for Telegram limit
			sb.WriteString("\n... (truncated)")
			break
		}
	}

	msg := tu.Message(chatIDObj, sb.String())
	msg.ParseMode = "HTML"
	c.bot.SendMessage(ctx, msg)
}

// handleProjectOutputEvent forwards a project stream event to active Telegram sessions.
func (c *Channel) handleProjectOutputEvent(ctx context.Context, event bus.Event) {
	payload, ok := event.Payload.(map[string]interface{})
	if !ok {
		return
	}

	sessionID, _ := payload["session_id"].(uuid.UUID)
	if sessionID == uuid.Nil {
		return
	}
	sessionIDStr := sessionID.String()

	rawEvt, _ := payload["event"].(claudecode.StreamEvent)

	// Find chat IDs that have this session active
	c.projectActiveSession.Range(func(key, value any) bool {
		if value.(string) != sessionIDStr {
			return true
		}
		chatIDStr := key.(string)
		chatID, err := parseChatID(chatIDStr)
		if err != nil {
			return true
		}

		text := formatProjectEventForTelegram(rawEvt)
		if text == "" {
			return true
		}

		msg := tu.Message(tu.ID(chatID), text)
		msg.ParseMode = "HTML"
		if _, err := c.bot.SendMessage(ctx, msg); err != nil {
			slog.Debug("projects: failed to forward event to telegram", "chat_id", chatID, "error", err)
		}
		return true
	})
}

// formatProjectEventForTelegram converts a stream event to Telegram HTML text.
// Returns empty string for events that shouldn't be forwarded (e.g. system init).
func formatProjectEventForTelegram(evt claudecode.StreamEvent) string {
	switch evt.Type {
	case "assistant":
		// Extract text blocks from assistant message
		var msg struct {
			Message struct {
				Content []struct {
					Type  string          `json:"type"`
					Text  string          `json:"text"`
					Name  string          `json:"name"`
					Input json.RawMessage `json:"input"`
				} `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal(evt.Raw, &msg); err != nil {
			return ""
		}
		var parts []string
		for _, block := range msg.Message.Content {
			if block.Type == "text" && block.Text != "" {
				parts = append(parts, escapeHTML(block.Text))
			}
			if block.Type == "tool_use" {
				parts = append(parts, fmt.Sprintf("🔧 <i>%s</i>", escapeHTML(block.Name)))
			}
		}
		if len(parts) == 0 {
			return ""
		}
		return strings.Join(parts, "\n")

	case "result":
		icon := "✅"
		label := "Completed"
		if evt.Subtype == "error" {
			icon = "❌"
			label = "Failed"
		}
		cost := ""
		if evt.CostUSD > 0 {
			cost = fmt.Sprintf(" · $%.4f", evt.CostUSD)
		}
		return fmt.Sprintf("%s <b>%s</b> (%d↑ %d↓%s)", icon, label, evt.InputTokens, evt.OutputTokens, cost)

	default:
		return ""
	}
}

// logTypeIcon returns an emoji for a log event type.
func logTypeIcon(eventType string) string {
	switch eventType {
	case "assistant":
		return "💬"
	case "tool_result":
		return "📎"
	case "result":
		return "✅"
	case "system":
		return "⚙️"
	case "user":
		return "👤"
	default:
		return "•"
	}
}

// extractLogSummary returns a short text summary for a log entry.
func extractLogSummary(eventType string, content json.RawMessage) string {
	switch eventType {
	case "assistant":
		var msg struct {
			Message struct {
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
					Name string `json:"name"`
				} `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal(content, &msg); err != nil {
			return "assistant message"
		}
		for _, block := range msg.Message.Content {
			if block.Type == "text" && block.Text != "" {
				text := block.Text
				if len(text) > 100 {
					text = text[:100] + "..."
				}
				return text
			}
			if block.Type == "tool_use" {
				return fmt.Sprintf("Tool: %s", block.Name)
			}
		}
		return "assistant message"

	case "result":
		var r struct {
			Subtype string `json:"subtype"`
		}
		_ = json.Unmarshal(content, &r)
		if r.Subtype == "error" {
			return "Result: error"
		}
		return "Result: success"

	case "user":
		var u struct {
			Text string `json:"text"`
		}
		_ = json.Unmarshal(content, &u)
		if u.Text != "" {
			text := u.Text
			if len(text) > 100 {
				text = text[:100] + "..."
			}
			return text
		}
		return "user message"

	default:
		return eventType
	}
}

// sessionStatusIcon returns an emoji for session status.
func sessionStatusIcon(status string) string {
	switch status {
	case "running", "starting":
		return "🟢"
	case "completed":
		return "✅"
	case "failed":
		return "❌"
	case "stopped":
		return "⏹"
	default:
		return "⚪"
	}
}
