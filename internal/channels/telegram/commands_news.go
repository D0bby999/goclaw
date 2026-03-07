package telegram

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"strings"
	"time"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

const maxNewsItems = 10

// handleNewsList handles the /news command — shows latest news digest.
func (c *Channel) handleNewsList(ctx context.Context, chatID int64, setThread func(*telego.SendMessageParams)) {
	chatIDObj := tu.ID(chatID)

	send := func(text string) {
		msg := tu.Message(chatIDObj, text)
		setThread(msg)
		c.bot.SendMessage(ctx, msg)
	}

	if c.newsStore == nil {
		send("News features are not available.")
		return
	}

	agentID, err := c.resolveAgentUUID(ctx)
	if err != nil {
		slog.Debug("news command: agent resolve failed", "error", err)
		send("News features are not available (no agent).")
		return
	}

	since := time.Now().Add(-24 * time.Hour)
	items, err := c.newsStore.ListItems(ctx, store.NewsItemFilter{
		AgentID: agentID,
		Since:   &since,
		Limit:   maxNewsItems,
	})
	if err != nil {
		slog.Warn("news command: ListItems failed", "error", err)
		send("Failed to fetch news. Please try again.")
		return
	}

	if len(items) == 0 {
		send("No news items in the last 24 hours.")
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>News Digest</b> (%d items, last 24h)\n", len(items)))

	for i, item := range items {
		sb.WriteString(fmt.Sprintf("\n%d. <b>%s</b>", i+1, html.EscapeString(item.Title)))

		if len(item.Categories) > 0 {
			cats := make([]string, len(item.Categories))
			for j, cat := range item.Categories {
				cats[j] = html.EscapeString(cat)
			}
			sb.WriteString(fmt.Sprintf(" [%s]", strings.Join(cats, ", ")))
		}
		sb.WriteString("\n")

		if item.Summary != nil && *item.Summary != "" {
			summary := truncateStr(*item.Summary, 150)
			sb.WriteString(fmt.Sprintf("   %s\n", html.EscapeString(summary)))
		}

		sourceName := "link"
		if item.SourceName != nil && *item.SourceName != "" {
			sourceName = *item.SourceName
		}
		sb.WriteString(fmt.Sprintf("   <a href=\"%s\">%s</a>\n", html.EscapeString(item.URL), html.EscapeString(sourceName)))
	}

	htmlText := sb.String()
	threadID := 0
	// Extract threadID from setThread by probing a dummy message.
	probe := tu.Message(chatIDObj, "")
	setThread(probe)
	threadID = probe.MessageThreadID

	for _, chunk := range chunkHTML(htmlText, telegramMaxMessageLen) {
		if err := c.sendHTML(ctx, chatID, chunk, 0, threadID); err != nil {
			slog.Warn("news command: sendHTML failed", "error", err)
		}
	}
}
