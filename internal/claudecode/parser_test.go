package claudecode

import (
	"testing"
)

func TestParseStreamLine_SystemInit(t *testing.T) {
	line := []byte(`{"type":"system","subtype":"init","session_id":"abc123","tools":["Read","Edit"]}`)
	event, err := parseStreamLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.Type != "system" {
		t.Errorf("expected type=system, got %s", event.Type)
	}
	if event.Subtype != "init" {
		t.Errorf("expected subtype=init, got %s", event.Subtype)
	}
	if event.SessionID != "abc123" {
		t.Errorf("expected session_id=abc123, got %s", event.SessionID)
	}
}

func TestParseStreamLine_Assistant(t *testing.T) {
	line := []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"Hello"}]}}`)
	event, err := parseStreamLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.Type != "assistant" {
		t.Errorf("expected type=assistant, got %s", event.Type)
	}
	if event.Raw == nil {
		t.Error("expected raw to be non-nil")
	}
}

func TestParseStreamLine_Result(t *testing.T) {
	line := []byte(`{"type":"result","subtype":"success","session_id":"abc123","input_tokens":1000,"output_tokens":500,"cost_usd":0.05}`)
	event, err := parseStreamLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.Type != "result" {
		t.Errorf("expected type=result, got %s", event.Type)
	}
	if event.Subtype != "success" {
		t.Errorf("expected subtype=success, got %s", event.Subtype)
	}
	if event.InputTokens != 1000 {
		t.Errorf("expected input_tokens=1000, got %d", event.InputTokens)
	}
	if event.OutputTokens != 500 {
		t.Errorf("expected output_tokens=500, got %d", event.OutputTokens)
	}
	if event.CostUSD != 0.05 {
		t.Errorf("expected cost_usd=0.05, got %f", event.CostUSD)
	}
}

func TestParseStreamLine_ResultError(t *testing.T) {
	line := []byte(`{"type":"result","subtype":"error","error":"something went wrong"}`)
	event, err := parseStreamLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.Subtype != "error" {
		t.Errorf("expected subtype=error, got %s", event.Subtype)
	}
}

func TestParseStreamLine_MalformedJSON(t *testing.T) {
	line := []byte(`not json at all`)
	_, err := parseStreamLine(line)
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestParseStreamLine_EmptyLine(t *testing.T) {
	_, err := parseStreamLine([]byte{})
	if err == nil {
		t.Error("expected error for empty line")
	}
}

func TestParseStreamLine_ToolUse(t *testing.T) {
	line := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"/foo"}}]}}`)
	event, err := parseStreamLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.Type != "assistant" {
		t.Errorf("expected type=assistant, got %s", event.Type)
	}
}

func TestParseStreamLine_ToolResult(t *testing.T) {
	line := []byte(`{"type":"tool_result","tool_use_id":"xxx","content":"file contents here"}`)
	event, err := parseStreamLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.Type != "tool_result" {
		t.Errorf("expected type=tool_result, got %s", event.Type)
	}
}
