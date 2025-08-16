package handlers_test

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/wh-kuromai/allino"
	"github.com/wh-kuromai/allino/example/simple/handlers"
)

var s = allino.NewTestServer(nil)

func TestHealthcheckAPI(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/healthcheck?echo=hello", nil)
	w, _ := s.Fiber.Test(r)
	bodybuf, _ := io.ReadAll(w.Body)

	if w.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %d", w.StatusCode)
	}
	var resp allino.APIResponse[handlers.HealthcheckAPIOutput]

	if err := json.Unmarshal(bodybuf, &resp); err != nil {
		t.Fatalf("Failed to decode response body: %v", err)
	}

	if resp.Data.Echo != "hello" {
		t.Errorf("Expected Echo to be empty, got %s", resp.Data.Echo)
	}
}

func TestHealthcheckPointerAPI(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/healthcheck_pointer?echo=hello", nil)
	w, _ := s.Fiber.Test(r)
	bodybuf, _ := io.ReadAll(w.Body)

	if w.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %d", w.StatusCode)
	}
	var resp allino.APIResponse[handlers.HealthcheckAPIOutput]

	if err := json.Unmarshal(bodybuf, &resp); err != nil {
		t.Fatalf("Failed to decode response body: %v", err)
	}

	if resp.Data.Echo != "hello" {
		t.Errorf("Expected Echo to be empty, got %s", resp.Data.Echo)
	}
}
