# allino - TEST

## ✍ How to generate test with AI

`allino` makes it easy to write tests that are also highly compatible with AI-generated code.  
The general pattern follows `httptest`: create a `Recorder`, build a request using `alltest.NewTestRequest`, and call `HandleRequest` to test your handler.

```go
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
		t.Errorf("Expected Echo to be \"hello\", got %s", resp.Data.Echo)
	}
}
```

Since handlers created with allino define both request and response types, you can easily decode the response using json.Unmarshal.

## Automatic login + CSRF handling in tests

When using `alltest.NewTestRequest(..., "user_id")`, the following are automatically handled:

- A valid JWT access token is issued and attached as `Authorization: Bearer ...`
- CSRF validation is bypassed (writable = true) based on the token
- The request context behaves exactly as if the user is fully logged in
- Path parameters (e.g. :uid) are parsed and injected into `req.PathParams`

## AI Prompt Template for TEST (works well with ChatGPT)

```go
// allino Test Framework
// allino enables you to define API handlers using a unique, strictly-typed approach.
// These handlers are automatically registered to the server.
// When writing tests, you can continue to use standard patterns you're already familiar with.
//
// If you need to generate or modify API handlers, please refer to the full allino documentation.
// Ask the user to provide it if it's not already available.
//
// EXAMPLE: Test Initialization
// IMPORTANT: Be sure to import your handler package with a blank identifier (`_`) 
// even if you don't reference it directly. This is required for automatic handler registration.
// If you're unsure of the handler package path, ask the user to add the appropriate import.
import ( 
  "github.com/wh-kuromai/allino"
	_ "path/to/your/test/handler"
)
// Use allino.NewTestServer to start a test server.
// This can be called inside your test function.
var s = allino.NewTestServer(nil)
// If Redis is required:
var s = allino.NewTestServer(&allino.Config{
	Redis: allino.RedisConfig{URL: "redis://localhost:6379/0"},
})
// If SQL is required:
var s = allino.NewTestServer(&allino.Config{
	SQL:   allino.SQLConfig{Driver: "mysql", DSN: "user:password@tcp(localhost:3306)/dbname"},
})
// EXAMPLE: Test Function
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
		t.Errorf("Expected Echo to be \"hello\", got %s", resp.Data.Echo)
	}
}
---
// The following utilities are available when writing tests with allino:
package alltest // github.com/wh-kuromai/allino/alltest
// In most cases, you don't need to construct *allino.Request manually.
func NewTestRequest(s *allino.Server) *allino.Request
---
package allino //github.com/wh-kuromai/allino
// IssueAccessToken issues a short-lived token used to API access.
func IssueAccessToken(r *Request, uid, name string) string
// IssueCSRFToken issues a short-lived token used to protect write operations from CSRF attacks.
func IssueCSRFToken(r *Request, uid string) string
// IssueLoginCookie issues a login cookie for user authentication.
func IssueLoginCookie(r *Request, uid, name string) *http.Cookie
// allino wraps API responses by default. (unless HandlerOption.NoWrapJSON = true)
type APIResponse[T any] struct {
	Data T `json:"data"`
}
type APIError[T error] struct {
	Err T `json:"error"`
}
func (e *APIError[T]) Error() string
```
