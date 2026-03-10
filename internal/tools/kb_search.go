package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// KBSearchTool searches the agent's knowledge base.
type KBSearchTool struct {
	kbStore store.KBStore
}

func NewKBSearchTool(kbStore store.KBStore) *KBSearchTool {
	return &KBSearchTool{kbStore: kbStore}
}

func (t *KBSearchTool) Name() string { return "kb_search" }

func (t *KBSearchTool) Description() string {
	return `Search the knowledge base for relevant information.
Use this when you need to look up documentation, product info, FAQs,
or any reference material uploaded to the knowledge base.
Returns ranked text snippets from the most relevant documents.`
}

func (t *KBSearchTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query — describe what information you need",
			},
			"collection_ids": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "Optional: limit search to specific collection IDs",
			},
			"max_results": map[string]interface{}{
				"type":        "number",
				"description": "Max results (default 5, max 20)",
			},
		},
		"required": []string{"query"},
	}
}

func (t *KBSearchTool) Execute(ctx context.Context, args map[string]interface{}) *Result {
	query, _ := args["query"].(string)
	if query == "" {
		return ErrorResult("query is required")
	}

	agentID := store.AgentIDFromContext(ctx)
	if agentID == uuid.Nil {
		return ErrorResult("no agent context")
	}

	maxResults := 5
	if mr, ok := args["max_results"].(float64); ok && mr > 0 {
		maxResults = int(mr)
		if maxResults > 20 {
			maxResults = 20
		}
	}

	var collectionIDs []string
	if ids, ok := args["collection_ids"].([]interface{}); ok {
		for _, id := range ids {
			if s, ok := id.(string); ok {
				collectionIDs = append(collectionIDs, s)
			}
		}
	}

	results, err := t.kbStore.Search(ctx, query, agentID.String(), store.KBSearchOptions{
		CollectionIDs: collectionIDs,
		MaxResults:    maxResults,
		MinScore:      0.1,
	})
	if err != nil {
		return ErrorResult("kb search failed: " + err.Error())
	}

	if len(results) == 0 {
		return NewResult("No relevant results found in the knowledge base for: " + query)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d relevant results:\n\n", len(results)))
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("--- Result %d (from: %s, score: %.2f) ---\n",
			i+1, r.Filename, r.Score))
		sb.WriteString(r.Text)
		sb.WriteString("\n\n")
	}

	return NewResult(sb.String())
}
