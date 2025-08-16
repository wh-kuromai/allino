package allino_test

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wh-kuromai/allino"
	"github.com/wh-kuromai/allino/example/test/handlers"

	_ "github.com/lib/pq"
)

var s = allino.NewTestServer(&allino.Config{
	Redis: allino.RedisConfig{
		URL: "redis://localhost:6379/0",
	},
	SQL: allino.SQLConfig{
		Driver: "postgres",
		DSN:    "postgresql://testuser@localhost:5432/testdb?sslmode=disable",
	},
})

func TestEchoAPI(t *testing.T) {
	req := httptest.NewRequest("GET", "/test/echo?echo=hello", nil)
	w, _ := s.Fiber.Test(req, -1)
	bodybuf, _ := io.ReadAll(w.Body)

	if w.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %d", w.StatusCode)
	}
	var resp allino.APIResponse[handlers.EchoAPIOutput]

	if err := json.Unmarshal(bodybuf, &resp); err != nil {
		t.Fatalf("Failed to decode response body: %v", err)
	}

	if resp.Data.Echo != "hello" {
		t.Errorf("Expected Echo to be `hello`, got %s", resp.Data.Echo)
	}
}

func TestPathParamAPIWithForm(t *testing.T) {
	form := "myform=formvalue"
	req := httptest.NewRequest("POST", "/test/pathparam/wow", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w, _ := s.Fiber.Test(req, -1)
	bodybuf, _ := io.ReadAll(w.Body)

	if w.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %d", w.StatusCode)
	}
	var resp allino.APIResponse[handlers.PathParamAPIOutput]

	if err := json.Unmarshal(bodybuf, &resp); err != nil {
		t.Fatalf("Failed to decode response body: %v", err)
	}

	if resp.Data.Path != "wow" {
		t.Errorf("Expected Path to be `wow`, got %s", resp.Data.Path)
	}
	if resp.Data.Form != "formvalue" {
		t.Errorf("Expected Form to be `formvalue`, got %s", resp.Data.Form)
	}
}

func TestValidationAPI_Success(t *testing.T) {
	form := "email=test@example.com"
	req := httptest.NewRequest("POST", "/test/validate?name=Yotsuba", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w, _ := s.Fiber.Test(req, -1)
	bodybuf, _ := io.ReadAll(w.Body)

	if w.StatusCode != 200 {
		t.Fatalf("Expected status 200, got %d", w.StatusCode)
	}

	var resp allino.APIResponse[handlers.ValidationAPIOutput]
	if err := json.Unmarshal(bodybuf, &resp); err != nil {
		t.Fatalf("JSON parse error: %v", err)
	}

	if resp.Data.Message == "" {
		t.Errorf("Expected message in response, got empty string")
	}
}

func TestValidationAPI_MissingName(t *testing.T) {
	form := "email=test@example.com"
	req := httptest.NewRequest("POST", "/test/validate", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w, _ := s.Fiber.Test(req, -1)
	bodybuf, _ := io.ReadAll(w.Body)

	if w.StatusCode == 200 {
		t.Errorf("Expected validation error, got 200 OK," + string(bodybuf))
	}
	var resp allino.APIResponse[any]
	if err := json.Unmarshal(bodybuf, &resp); err == nil {
		if resp.Data != nil {
			t.Errorf("Expected no data, got: %+v", resp.Data)
		}
	}
}

func TestValidationAPI_InvalidEmail(t *testing.T) {
	form := "email=not-an-email"
	req := httptest.NewRequest("POST", "/test/validate?name=Yotsuba", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w, _ := s.Fiber.Test(req, -1)

	if w.StatusCode == 200 {
		t.Errorf("Expected validation error for invalid email, got 200 OK")
	}
}

func TestErrorAPI_NormalError(t *testing.T) {
	req := httptest.NewRequest("GET", "/test/error?mode=normal", nil)
	w, _ := s.Fiber.Test(req, -1)
	bodybuf, _ := io.ReadAll(w.Body)

	if w.StatusCode != 400 {
		t.Errorf("Expected status 400, got %d", w.StatusCode)
	}

	var resp allino.APIError[allino.Error]
	if err := json.Unmarshal(bodybuf, &resp); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	if resp.Err.Msg == "" {
		t.Errorf("Expected error message, got empty string")
	}
}

func TestErrorAPI_CodeError(t *testing.T) {
	req := httptest.NewRequest("GET", "/test/error?mode=code", nil)
	w, _ := s.Fiber.Test(req, -1)
	bodybuf, _ := io.ReadAll(w.Body)

	if w.StatusCode != 403 {
		t.Errorf("Expected status 403, got %d", w.StatusCode)
	}

	var resp allino.APIError[*allino.CodeError]
	if err := json.Unmarshal(bodybuf, &resp); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	if resp.Err.Code != "FORBIDDEN" {
		t.Errorf("Expected error code FORBIDDEN, got %s", resp.Err.Code)
	}
	if resp.Err.Msg != "you are not allowed" {
		t.Errorf("Unexpected error message: %s", resp.Err.Msg)
	}
}

func TestRedirectAPI(t *testing.T) {
	target := "https://example.com"
	req := httptest.NewRequest("GET", "/test/redirect?target="+target, nil)
	w, _ := s.Fiber.Test(req, -1)

	if w.StatusCode != 302 {
		t.Errorf("Expected redirect status 302, got %d", w.StatusCode)
	}
	loc := w.Header.Get("Location")
	if loc != target {
		t.Errorf("Expected Location header to be %s, got %s", target, loc)
	}
}
