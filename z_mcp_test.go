package allino_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/wh-kuromai/allino"
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

func TestMCPMarkdownPrompts(t *testing.T) {
	dir := t.TempDir()
	resourceDir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "with-frontmatter.md"), []byte(`---
name: mounted_prompt
description: Mounted prompt description.
---
Use the mounted prompt body.
`), 0600)
	if err != nil {
		t.Fatalf("failed to write prompt: %v", err)
	}
	err = os.WriteFile(filepath.Join(dir, "filename_prompt.md"), []byte("Use filename fallback."), 0600)
	if err != nil {
		t.Fatalf("failed to write prompt: %v", err)
	}
	err = os.WriteFile(filepath.Join(resourceDir, "note.txt"), []byte("local resource body"), 0600)
	if err != nil {
		t.Fatalf("failed to write resource: %v", err)
	}

	server := allino.NewTestServer(&allino.Config{
		ConfigBytes: []byte("mcp:\n  endpoint: /mounted_mcp\n  resourceScheme: app\n  resourceHost: file\n  promptDirs:\n    - " + dir + "\n  resourceDirs:\n    - " + resourceDir + "\n"),
		Debug:       true,
		SQL: allino.SQLConfig{
			Driver: "sqlite",
		},
	})

	post := func(body string) map[string]any {
		t.Helper()
		req := httptest.NewRequest("POST", "/mounted_mcp", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := server.Fiber.Test(req, -1)
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

	listOut := post(`{"jsonrpc":"2.0","id":1,"method":"prompts/list"}`)
	prompts := listOut["result"].(map[string]any)["prompts"].([]any)
	foundFrontmatter := false
	foundFilename := false
	for _, raw := range prompts {
		prompt := raw.(map[string]any)
		switch prompt["name"] {
		case "mounted_prompt":
			foundFrontmatter = true
			if prompt["description"] != "Mounted prompt description." {
				t.Fatalf("unexpected description: %#v", prompt)
			}
		case "filename_prompt":
			foundFilename = true
		}
	}
	if !foundFrontmatter || !foundFilename {
		t.Fatalf("expected mounted prompts in list, got %#v", prompts)
	}

	getOut := post(`{"jsonrpc":"2.0","id":2,"method":"prompts/get","params":{"name":"mounted_prompt"}}`)
	messages := getOut["result"].(map[string]any)["messages"].([]any)
	content := messages[0].(map[string]any)["content"].(map[string]any)
	if content["text"] != "Use the mounted prompt body." {
		t.Fatalf("unexpected prompt body: %#v", content)
	}

	resourceListOut := post(`{"jsonrpc":"2.0","id":3,"method":"resources/list"}`)
	resources := resourceListOut["result"].(map[string]any)["resources"].([]any)
	var resourceURI string
	for _, raw := range resources {
		resource := raw.(map[string]any)
		if resource["name"] == "note.txt" {
			resourceURI = resource["uri"].(string)
			if resourceURI != "app://file/note.txt" {
				t.Fatalf("unexpected resource uri: %#v", resource)
			}
			if resource["mimeType"] != "text/plain; charset=utf-8" {
				t.Fatalf("unexpected resource mime type: %#v", resource)
			}
		}
	}
	if resourceURI == "" {
		t.Fatalf("expected mounted resource in list, got %#v", resources)
	}

	resourceReadOut := post(`{"jsonrpc":"2.0","id":4,"method":"resources/read","params":{"uri":"` + resourceURI + `"}}`)
	contents := resourceReadOut["result"].(map[string]any)["contents"].([]any)
	resourceContent := contents[0].(map[string]any)
	if resourceContent["text"] != "local resource body" {
		t.Fatalf("unexpected resource body: %#v", resourceContent)
	}
}
