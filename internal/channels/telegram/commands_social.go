package telegram

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"

	"github.com/nextlevelbuilder/goclaw/internal/social"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// socialPlatformIcon returns an emoji for the given platform.
func socialPlatformIcon(platform string) string {
	switch platform {
	case "facebook":
		return "📘"
	case "instagram":
		return "📷"
	case "twitter":
		return "🐦"
	case "youtube":
		return "🎬"
	case "tiktok":
		return "🎵"
	case "threads":
		return "🧵"
	case "linkedin":
		return "💼"
	case "bluesky":
		return "🦋"
	default:
		return "🌐"
	}
}

// socialPostStatusIcon returns a status emoji for post status.
func socialPostStatusIcon(status string) string {
	switch status {
	case store.SocialPostStatusDraft:
		return "📝"
	case store.SocialPostStatusScheduled:
		return "⏰"
	case store.SocialPostStatusPublishing:
		return "🔄"
	case store.SocialPostStatusPublished:
		return "✅"
	case store.SocialPostStatusPartial:
		return "⚠️"
	case store.SocialPostStatusFailed:
		return "❌"
	default:
		return "❓"
	}
}

// handleSocialList handles /social — lists connected accounts.
func (c *Channel) handleSocialList(ctx context.Context, chatID int64, setThread func(*telego.SendMessageParams)) {
	chatIDObj := tu.ID(chatID)
	send := func(text string) {
		msg := tu.Message(chatIDObj, text)
		setThread(msg)
		c.bot.SendMessage(ctx, msg)
	}

	if c.socialStore == nil {
		send("Social features are not available.")
		return
	}

	agentID, err := c.resolveAgentUUID(ctx)
	if err != nil {
		send("Social features are not available (no agent).")
		return
	}

	accounts, err := c.socialStore.ListAccounts(ctx, agentID.String())
	if err != nil {
		slog.Warn("social command: ListAccounts failed", "error", err)
		send("Failed to fetch accounts.")
		return
	}

	if len(accounts) == 0 {
		send("No social accounts connected. Use the web dashboard to connect accounts.")
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>Social Accounts</b> (%d)\n", len(accounts)))
	for _, a := range accounts {
		icon := socialPlatformIcon(a.Platform)
		name := a.Platform
		if a.PlatformUsername != nil && *a.PlatformUsername != "" {
			name = "@" + *a.PlatformUsername
		} else if a.DisplayName != nil && *a.DisplayName != "" {
			name = *a.DisplayName
		}
		sb.WriteString(fmt.Sprintf("\n%s <b>%s</b> — %s [%s]",
			icon, html.EscapeString(a.Platform), html.EscapeString(name), a.Status))
	}

	htmlText := sb.String()
	probe := tu.Message(chatIDObj, "")
	setThread(probe)
	threadID := probe.MessageThreadID
	for _, chunk := range chunkHTML(htmlText, telegramMaxMessageLen) {
		if err := c.sendHTML(ctx, chatID, chunk, 0, threadID); err != nil {
			slog.Warn("social command: sendHTML failed", "error", err)
		}
	}
}

// handlePostCreate handles /post <content> — creates a draft post.
func (c *Channel) handlePostCreate(ctx context.Context, chatID int64, text string, setThread func(*telego.SendMessageParams)) {
	chatIDObj := tu.ID(chatID)
	send := func(t string) {
		msg := tu.Message(chatIDObj, t)
		setThread(msg)
		c.bot.SendMessage(ctx, msg)
	}

	if c.socialStore == nil {
		send("Social features are not available.")
		return
	}

	// Extract content after "/post "
	content := strings.TrimSpace(strings.TrimPrefix(text, "/post"))
	content = strings.TrimPrefix(content, "@"+c.bot.Username())
	content = strings.TrimSpace(content)
	if content == "" {
		send("Usage: /post <content>\nExample: /post Check out our latest update!")
		return
	}

	agentID, err := c.resolveAgentUUID(ctx)
	if err != nil {
		send("Social features are not available (no agent).")
		return
	}

	post := &store.SocialPostData{
		OwnerID:  agentID.String(),
		Content:  content,
		PostType: "standard",
		Status:   store.SocialPostStatusDraft,
	}
	if err := c.socialStore.CreatePost(ctx, post); err != nil {
		slog.Warn("social command: CreatePost failed", "error", err)
		send("Failed to create post.")
		return
	}

	probe := tu.Message(chatIDObj, "")
	setThread(probe)
	threadID := probe.MessageThreadID

	htmlText := fmt.Sprintf("📝 <b>Draft post created</b>\nID: <code>%s</code>\n\n%s\n\nUse /publish to publish or the web dashboard to add targets.",
		post.ID.String(), html.EscapeString(truncateStr(content, 200)))
	if err := c.sendHTML(ctx, chatID, htmlText, 0, threadID); err != nil {
		slog.Warn("social command: sendHTML failed", "error", err)
	}
}

// handlePostsList handles /posts — lists recent posts.
func (c *Channel) handlePostsList(ctx context.Context, chatID int64, setThread func(*telego.SendMessageParams)) {
	chatIDObj := tu.ID(chatID)
	send := func(t string) {
		msg := tu.Message(chatIDObj, t)
		setThread(msg)
		c.bot.SendMessage(ctx, msg)
	}

	if c.socialStore == nil {
		send("Social features are not available.")
		return
	}

	agentID, err := c.resolveAgentUUID(ctx)
	if err != nil {
		send("Social features are not available (no agent).")
		return
	}

	posts, _, err := c.socialStore.ListPosts(ctx, agentID.String(), "", 10, 0)
	if err != nil {
		slog.Warn("social command: ListPosts failed", "error", err)
		send("Failed to fetch posts.")
		return
	}

	if len(posts) == 0 {
		send("No posts found. Use /post <content> to create one.")
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>Recent Posts</b> (%d)\n", len(posts)))
	for i, p := range posts {
		icon := socialPostStatusIcon(p.Status)
		preview := truncateStr(p.Content, 60)
		sb.WriteString(fmt.Sprintf("\n%d. %s %s\n   <code>%s</code>",
			i+1, icon, html.EscapeString(preview), p.ID.String()))
	}

	htmlText := sb.String()
	probe := tu.Message(chatIDObj, "")
	setThread(probe)
	threadID := probe.MessageThreadID
	for _, chunk := range chunkHTML(htmlText, telegramMaxMessageLen) {
		if err := c.sendHTML(ctx, chatID, chunk, 0, threadID); err != nil {
			slog.Warn("social command: sendHTML failed", "error", err)
		}
	}
}

// handlePublishMenu handles /publish — shows draft posts as inline keyboard.
func (c *Channel) handlePublishMenu(ctx context.Context, chatID int64, setThread func(*telego.SendMessageParams)) {
	chatIDObj := tu.ID(chatID)
	send := func(t string) {
		msg := tu.Message(chatIDObj, t)
		setThread(msg)
		c.bot.SendMessage(ctx, msg)
	}

	if c.socialStore == nil || c.socialManager == nil {
		send("Social features are not available.")
		return
	}

	agentID, err := c.resolveAgentUUID(ctx)
	if err != nil {
		send("Social features are not available (no agent).")
		return
	}

	posts, _, err := c.socialStore.ListPosts(ctx, agentID.String(), store.SocialPostStatusDraft, 10, 0)
	if err != nil {
		slog.Warn("social command: ListPosts failed", "error", err)
		send("Failed to fetch posts.")
		return
	}

	if len(posts) == 0 {
		send("No draft posts to publish. Use /post <content> to create one.")
		return
	}

	// Build inline keyboard with one button per draft post
	var rows [][]telego.InlineKeyboardButton
	for _, p := range posts {
		preview := truncateStr(p.Content, 40)
		rows = append(rows, []telego.InlineKeyboardButton{
			{Text: fmt.Sprintf("📤 %s", preview), CallbackData: "sc:" + p.ID.String()},
		})
	}

	msg := tu.Message(chatIDObj, "Select a post to publish:")
	setThread(msg)
	msg.ReplyMarkup = &telego.InlineKeyboardMarkup{InlineKeyboard: rows}
	if _, err := c.bot.SendMessage(ctx, msg); err != nil {
		slog.Warn("social command: send publish menu failed", "error", err)
	}
}

// handleSocialCallback handles "sc:<postID>" callback queries from /publish inline keyboard.
func (c *Channel) handleSocialCallback(ctx context.Context, cb *telego.CallbackQuery, postIDStr string) {
	answer := func(text string) {
		c.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
			CallbackQueryID: cb.ID,
			Text:            text,
			ShowAlert:       true,
		})
	}

	if c.socialStore == nil || c.socialManager == nil {
		answer("Social features not available.")
		return
	}

	postID, err := uuid.Parse(postIDStr)
	if err != nil {
		answer("Invalid post ID.")
		return
	}

	mgr, ok := c.socialManager.(*social.Manager)
	if !ok {
		answer("Social manager not configured.")
		return
	}

	if err := mgr.PublishPost(ctx, postID); err != nil {
		slog.Warn("social callback: PublishPost failed", "error", err, "post_id", postIDStr)
		answer("Failed to publish: " + err.Error())
		return
	}

	answer("Publishing started! Check /posts for status.")

	// Update the inline message to show published status
	if cb.Message != nil {
		chatID := cb.Message.GetChat().ID
		msgID := cb.Message.GetMessageID()
		c.bot.EditMessageText(ctx, &telego.EditMessageTextParams{
			ChatID:    telego.ChatID{ID: chatID},
			MessageID: msgID,
			Text:      fmt.Sprintf("✅ Publishing post <code>%s</code>...", html.EscapeString(postIDStr)),
			ParseMode: "HTML",
		})
	}
}
