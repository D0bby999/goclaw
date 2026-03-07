package claudecode

import (
	"encoding/json"
	"fmt"
)

// parseStreamLine parses a single line of Claude Code stream-json output.
func parseStreamLine(line []byte) (StreamEvent, error) {
	if len(line) == 0 {
		return StreamEvent{}, fmt.Errorf("empty line")
	}

	var event StreamEvent
	event.Raw = json.RawMessage(make([]byte, len(line)))
	copy(event.Raw, line)

	// Parse the envelope to extract type/subtype/session_id
	var envelope struct {
		Type      string `json:"type"`
		Subtype   string `json:"subtype"`
		SessionID string `json:"session_id"`
		// Result fields — tokens nested under "usage", cost at top level
		Usage struct {
			InputTokens              int `json:"input_tokens"`
			OutputTokens             int `json:"output_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		} `json:"usage"`
		TotalCostUSD float64 `json:"total_cost_usd"`
	}
	if err := json.Unmarshal(line, &envelope); err != nil {
		return StreamEvent{}, fmt.Errorf("parse stream line: %w", err)
	}

	event.Type = envelope.Type
	event.Subtype = envelope.Subtype
	event.SessionID = envelope.SessionID

	// Extract token/cost info from result events
	if envelope.Type == "result" {
		event.InputTokens = envelope.Usage.InputTokens
		event.OutputTokens = envelope.Usage.OutputTokens
		event.CacheReadInputTokens = envelope.Usage.CacheReadInputTokens
		event.CacheCreationTokens = envelope.Usage.CacheCreationInputTokens
		event.CostUSD = envelope.TotalCostUSD
	}

	return event, nil
}
