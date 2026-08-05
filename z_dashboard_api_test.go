package allino_test

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/wh-kuromai/allino"
	_ "github.com/wh-kuromai/allino/dashboard/api"
)

type dashboardFunctionsTestOutput struct {
	Functions []dashboardFunctionTestInfo `json:"functions"`
	Count     int                         `json:"count"`
}

type dashboardFunctionTestInfo struct {
	Path         string `json:"path"`
	Method       string `json:"method"`
	ContentType  string `json:"contentType"`
	InputSchema  any    `json:"inputSchema"`
	OutputSchema any    `json:"outputSchema"`
}

type dashboardJobsTestOutput struct {
	Count  int            `json:"count"`
	Status map[string]any `json:"status"`
}

type dashboardHealthTestOutput struct {
	AppName  string                    `json:"appName"`
	Services map[string]map[string]any `json:"services"`
}

type dashboardMeTestOutput struct {
	LoggedIn    bool   `json:"loggedIn"`
	UID         string `json:"uid"`
	DisplayName string `json:"displayName"`
}

type dashboardMCPTestOutput struct {
	Summary map[string]any   `json:"summary"`
	Tools   []map[string]any `json:"tools"`
}

func TestDashboardFunctionsAPI(t *testing.T) {
	req := httptest.NewRequest("GET", "/dashboard/api/functions", nil)
	w, err := s.Fiber.Test(req, -1)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	bodybuf, _ := io.ReadAll(w.Body)

	if w.StatusCode != 200 {
		t.Fatalf("Expected status code 200, got %d: %s", w.StatusCode, string(bodybuf))
	}

	var resp allino.APIResponse[dashboardFunctionsTestOutput]
	if err := json.Unmarshal(bodybuf, &resp); err != nil {
		t.Fatalf("Failed to decode response body: %v", err)
	}

	if resp.Data.Count == 0 {
		t.Fatalf("Expected functions to be returned")
	}
	if resp.Data.Count != len(resp.Data.Functions) {
		t.Fatalf("Expected count to match functions length, got %d and %d", resp.Data.Count, len(resp.Data.Functions))
	}

	var foundDashboard bool
	var foundEcho bool
	for _, fn := range resp.Data.Functions {
		switch fn.Path {
		case "/dashboard/api/functions":
			foundDashboard = true
			if fn.Method != "GET" {
				t.Fatalf("Expected dashboard function method GET, got %s", fn.Method)
			}
		case "/test/echo":
			foundEcho = true
			if fn.ContentType != allino.JSON {
				t.Fatalf("Expected echo content type JSON, got %s", fn.ContentType)
			}
			if fn.InputSchema == nil || fn.OutputSchema == nil {
				t.Fatalf("Expected echo schemas")
			}
		}
	}

	if !foundDashboard {
		t.Fatalf("Expected dashboard function to be listed")
	}
	if !foundEcho {
		t.Fatalf("Expected existing typed function to be listed")
	}
}

func TestDashboardJobsAPI(t *testing.T) {
	var resp allino.APIResponse[dashboardJobsTestOutput]
	requestDashboardJSON(t, "/dashboard/api/jobs?limit=5", &resp)
	if resp.Data.Status == nil {
		t.Fatalf("Expected job store status")
	}
}

func TestDashboardHealthAPI(t *testing.T) {
	var resp allino.APIResponse[dashboardHealthTestOutput]
	requestDashboardJSON(t, "/dashboard/api/health", &resp)
	if resp.Data.AppName == "" {
		t.Fatalf("Expected app name")
	}
	if resp.Data.Services == nil {
		t.Fatalf("Expected service health map")
	}
}

func TestDashboardMeAPI(t *testing.T) {
	var resp allino.APIResponse[dashboardMeTestOutput]
	requestDashboardJSON(t, "/dashboard/api/me?.user=dashboard-user", &resp)
	if !resp.Data.LoggedIn {
		t.Fatalf("Expected debug user to be logged in")
	}
	if resp.Data.UID != "dashboard-user" {
		t.Fatalf("Expected debug user id, got %s", resp.Data.UID)
	}
}

func TestDashboardMetaAPI(t *testing.T) {
	var openapiResp allino.APIResponse[map[string]any]
	requestDashboardJSON(t, "/dashboard/api/openapi", &openapiResp)
	if openapiResp.Data["paths"] == nil {
		t.Fatalf("Expected OpenAPI paths")
	}

	var mcpResp allino.APIResponse[dashboardMCPTestOutput]
	requestDashboardJSON(t, "/dashboard/api/mcp", &mcpResp)
	if mcpResp.Data.Summary == nil {
		t.Fatalf("Expected MCP summary")
	}
}

func requestDashboardJSON(t *testing.T, target string, out any) {
	t.Helper()
	req := httptest.NewRequest("GET", target, nil)
	w, err := s.Fiber.Test(req, -1)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	bodybuf, _ := io.ReadAll(w.Body)
	if w.StatusCode != 200 {
		t.Fatalf("Expected status code 200, got %d: %s", w.StatusCode, string(bodybuf))
	}
	if err := json.Unmarshal(bodybuf, out); err != nil {
		t.Fatalf("Failed to decode response body: %v\n%s", err, string(bodybuf))
	}
}
