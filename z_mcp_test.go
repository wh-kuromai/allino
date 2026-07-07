package allino_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
)

func postMCP(t *testing.T, body string) map[string]any {
	t.Helper()
	req := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.Fiber.Test(req, -1)
	if err != nil {
		t.Fatalf("MCP request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		bodybuf, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status 200, got %d: %s", resp.StatusCode, string(bodybuf))
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("failed to decode MCP response: %v", err)
	}
	return out
}

func TestMCPToolsList(t *testing.T) {
	out := postMCP(t, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	result := out["result"].(map[string]any)
	tools := result["tools"].([]any)
	found := false
	for _, raw := range tools {
		tool := raw.(map[string]any)
		if tool["name"] == "mcp_echo" {
			found = true
			if tool["inputSchema"] == nil {
				t.Fatalf("expected inputSchema for mcp_echo")
			}
			if tool["outputSchema"] == nil {
				t.Fatalf("expected outputSchema for mcp_echo")
			}
		}
	}
	if !found {
		t.Fatalf("mcp_echo tool was not listed: %#v", tools)
	}
}

func TestMCPToolsCall(t *testing.T) {
	out := postMCP(t, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"mcp_echo","arguments":{"message":"hello"}}}`)
	result := out["result"].(map[string]any)
	structured := result["structuredContent"].(map[string]any)
	if structured["echo"] != "hello" {
		t.Fatalf("expected echo=hello, got %#v", structured)
	}
	content := result["content"].([]any)[0].(map[string]any)
	if content["type"] != "text" {
		t.Fatalf("expected text content, got %#v", content)
	}
}
